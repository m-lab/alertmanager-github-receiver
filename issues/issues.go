// Copyright 2017 alertmanager-github-receiver Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//////////////////////////////////////////////////////////////////////////////

// Package issues defines a client interface wrapping the Github API for
// creating, listing, and closing issues on a single repository.
package issues

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

var (
	rateLimit = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "issues_api_rate_limit",
			Help: "The limit of API requests the client can make.",
		},
		// The GitHub API this rate limit applies to. e.g. "search" or "issues"
		[]string{"api"},
	)
	rateRemaining = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "issues_api_rate_remaining",
			Help: "The remaining API requests the client can make until reset time.",
		},
		// The GitHub API this rate limit applies to. e.g. "search" or "issues"
		[]string{"api"},
	)
	rateResetTime = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "issues_api_rate_reset",
			Help: "The time when the current rate limit will reset.",
		},
		// The GitHub API this rate limit applies to. e.g. "search" or "issues"
		[]string{"api"},
	)
	rateErrorCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "issues_api_rate_error_total",
			Help: "Number of API operations that encountered an API rate limit error.",
		},
	)
	operationCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "issues_api_total",
			Help: "Number of API operations performed.",
		},
		[]string{"status"},
	)
)

// A Client manages communication with the Github API.
type Client struct {
	// githubClient is an authenticated client for accessing the github API.
	GithubClient *github.Client
	// org is the github user or organization name (e.g. github.com/<org>/<repo>).
	org string
	// alertLabel is the label applied to all alerts.  It is also used as
	// the label to search to discover all existing alerts.
	alertLabel string
}

// NewClient creates an Client authenticated using the Github authToken.
// Future operations are only performed on the given github "org/repo".
func NewClient(org, authToken, alertLabel string) *Client {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: authToken},
	)

	client := &Client{
		GithubClient: github.NewClient(oauth2.NewClient(ctx, tokenSource)),
		org:          org,
		alertLabel:   alertLabel,
	}
	return client
}

// NewEnterpriseClient creates an Enterprise Client authenticated using the Github authToken.
// Future operations are only performed on the given github enterprise "org/repo".
// If uploadURL is empty it will be set to baseURL
func NewEnterpriseClient(baseURL, uploadURL, org, authToken, alertLabel string) (*Client, error) {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: authToken},
	)

	if uploadURL == "" {
		uploadURL = baseURL
	}

	githubClient, err := github.NewEnterpriseClient(baseURL, uploadURL, oauth2.NewClient(ctx, tokenSource))

	client := &Client{
		GithubClient: githubClient,
		org:          org,
		alertLabel:   alertLabel,
	}
	return client, err
}

// CreateIssue creates a new Github issue. New issues are unassigned. Issues are
// labeled with with an alert named alertLabel. Labels are created automatically
// if they do not already exist in a repo.
func (c *Client) CreateIssue(repo, title, body string, extra []string) (*github.Issue, error) {
	labels := make([]string, len(extra)+1)
	labels[0] = c.alertLabel
	for i := range extra {
		labels[i+1] = extra[i]
	}
	// Construct a minimal github issue request.
	issueReq := github.IssueRequest{
		Title:  &title,
		Body:   &body,
		Labels: &labels, // Search using: label:alertLabel
	}

	// Enforce a timeout on the issue creation.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create the issue.
	// See also: https://developer.github.com/v3/issues/#create-an-issue
	// See also: https://godoc.org/github.com/google/go-github/github#IssuesService.Create
	issue, resp, err := c.GithubClient.Issues.Create(
		ctx, c.org, repo, &issueReq)
	updateRateMetrics("issues", resp, err)
	if err != nil {
		log.Printf("Error in CreateIssue: response: %v", err)
		return nil, err
	}
	return issue, nil
}

// LabelIssue adds or removes a label from an issue. This call is idempotent;
// it returns errors if there's a network error, but
// no error is returned if trying to add a label that's already present or
// remove one that's absent.
func (c *Client) LabelIssue(issue *github.Issue, label string, add bool) error {
	// The GitHub API is weird about which actions are idempotent:
	// - Adding a label that exists in the project is idempotent.
	// - Adding a label that doesn't exist will silently create it.
	// - Deleting a label that's associated with an issue returns success.
	// - Deleting a label that's defined, but not associated with the issue, returns 404.
	// - Deleting a label that's not defined for the repo returns 404 with identical JSON text.
	// - Deleting a label from an issue that doesn't exist returns 404 with JSON message "Not Found".
	// We choose to ignore errors on delete if the status code is 404.
	if label == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	org, repo, err := getOrgAndRepoFromIssue(issue)
	if err != nil {
		return err
	}

	var resp *github.Response
	if add {
		_, resp, err = c.GithubClient.Issues.AddLabelsToIssue(ctx, org, repo, *issue.Number, []string{label})
	} else {
		resp, err = c.GithubClient.Issues.RemoveLabelForIssue(ctx, org, repo, *issue.Number, label)
		if resp.StatusCode == http.StatusNotFound {
			err = nil
		}
	}
	updateRateMetrics("issues", resp, err)

	return err
}

// ListOpenIssues returns open issues created by past alerts within the
// client organization. Because ListOpenIssues uses the Github Search API,
// the *github.Issue instances returned will contain partial information.
// See also: https://developer.github.com/v3/search/#search-issues
func (c *Client) ListOpenIssues() ([]*github.Issue, error) {
	var allIssues []*github.Issue

	sopts := &github.SearchOptions{}
	for {
		// Enforce a timeout on the issue listing.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Github issues are either "open" or "closed". Closed issues have either been
		// resolved automatically or by a person. So, there will be an ever increasing
		// number of "closed" issues. By only listing "open" issues we limit the
		// number of issues returned.
		//
		// The search depends on all relevant issues including the alertLabel label.
		issues, resp, err := c.GithubClient.Search.Issues(
			ctx, `is:issue in:title is:open org:`+c.org+` label:"`+c.alertLabel+`"`, sopts)
		updateRateMetrics("search", resp, err)
		if err != nil {
			log.Printf("Failed to list open github issues: %v\n", err)
			return nil, err
		}
		// Collect 'em all.
		for i := range issues.Issues {
			log.Println("ListOpenIssues:", issues.Issues[i].GetTitle())
			allIssues = append(allIssues, &issues.Issues[i])
		}

		// Continue loading the next page until all issues are received.
		if resp.NextPage == 0 {
			break
		}
		sopts.ListOptions.Page = resp.NextPage
	}
	return allIssues, nil
}

// CloseIssue changes the issue state to "closed" unconditionally. If the issue
// is already close, then this should have no effect.
func (c *Client) CloseIssue(issue *github.Issue) (*github.Issue, error) {
	issueReq := github.IssueRequest{
		State: github.String("closed"),
	}
	org, repo, err := getOrgAndRepoFromIssue(issue)
	if err != nil {
		return nil, err
	}
	// Enforce a timeout on the issue edit.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Edits the issue to have "closed" state.
	// See also: https://developer.github.com/v3/issues/#edit-an-issue
	// See also: https://godoc.org/github.com/google/go-github/github#IssuesService.Edit
	closedIssue, resp, err := c.GithubClient.Issues.Edit(
		ctx, org, repo, *issue.Number, &issueReq)
	updateRateMetrics("issues", resp, err)
	if err != nil {
		log.Printf("Failed to close issue: %v", err)
		return nil, err
	}
	return closedIssue, nil
}

// getOrgAndRepoFromIssue reads the issue RepositoryURL and extracts the
// owner and repo names. Issues returned by the Search API contain partial
// records.
func getOrgAndRepoFromIssue(issue *github.Issue) (string, string, error) {
	repoURL := issue.GetRepositoryURL()
	if repoURL == "" {
		return "", "", fmt.Errorf("issue has invalid RepositoryURL value")
	}
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", "", err
	}
	fields := strings.Split(u.Path, "/")
	if len(fields) < 4 {
		return "", "", fmt.Errorf("issue has invalid RepositoryURL path values")
	}

	//this supports urls of github ("http(s)://api.github.com/repos/org/repo") and
	//enterprise ("http(s)://hostname/api/version/repos/org/repo")
	for i := 0; i < len(fields); i++ {
		if fields[i] == "repos" {
			if i+2 == len(fields)-1 {
				return fields[i+1], fields[i+2], nil
			}
		}
	}
	return "", "", fmt.Errorf("issue has invalid RepositoryURL path values")
}

func updateRateMetrics(api string, resp *github.Response, err error) {
	// Update rate limit metrics.
	rateLimit.WithLabelValues(api).Set(float64(resp.Rate.Limit))
	rateRemaining.WithLabelValues(api).Set(float64(resp.Rate.Remaining))
	rateResetTime.WithLabelValues(api).Set(float64(resp.Rate.Reset.UTC().Unix()))
	// Count the number of API operations per HTTP Status.
	operationCount.WithLabelValues(resp.Status).Inc()
	// If the err is a RateLimitError, then increment the rateError counter.
	if _, ok := err.(*github.RateLimitError); ok {
		log.Println("Hit rate limit!")
		rateErrorCount.Inc()
	}
}
