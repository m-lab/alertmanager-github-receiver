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
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/kr/pretty"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

// A Client manages communication with the Github API.
type Client struct {
	// githubClient is an authenticated client for accessing the github API.
	GithubClient *github.Client
	// org is the github user or organization name (e.g. github.com/<org>/<repo>).
	org string
}

// NewClient creates an Client authenticated using the Github authToken.
// Future operations are only performed on the given github "org/repo".
func NewClient(org, authToken string) *Client {
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: authToken},
	)
	client := &Client{
		GithubClient: github.NewClient(oauth2.NewClient(ctx, tokenSource)),
		org:          org,
	}
	return client
}

// CreateIssue creates a new Github issue. New issues are unassigned. Issues are
// labeled with with an alert named "alert:boom:". Labels are created automatically
// if they do not already exist in a repo.
func (c *Client) CreateIssue(repo, title, body string) (*github.Issue, error) {
	// Construct a minimal github issue request.
	issueReq := github.IssueRequest{
		Title:  &title,
		Body:   &body,
		Labels: &([]string{"alert:boom:"}), // Search using: label:"alert:boom:"
	}

	// Enforce a timeout on the issue creation.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Create the issue.
	// See also: https://developer.github.com/v3/issues/#create-an-issue
	// See also: https://godoc.org/github.com/google/go-github/github#IssuesService.Create
	issue, resp, err := c.GithubClient.Issues.Create(
		ctx, c.org, repo, &issueReq)
	if err != nil {
		log.Printf("Error in CreateIssue: response: %v\n%s",
			err, pretty.Sprint(resp))
		return nil, err
	}
	return issue, nil
}

// ListOpenIssues returns open issues created by past alerts from the configured
// organization. Because ListOpenIssues uses the Github Search API, the
// *github.Issue objects returned contain partial information.
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
		// The search depends on all relevant issues including the "alert:boom:" label.
		issues, resp, err := c.GithubClient.Search.Issues(
			ctx, `is:issue in:title is:open org:`+c.org+` label:"alert:boom:"`, sopts)
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
	closedIssue, _, err := c.GithubClient.Issues.Edit(
		ctx, org, repo, *issue.Number, &issueReq)
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
		return "", "", fmt.Errorf("Issue has invalid RepositoryURL value")
	}
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", "", err
	}
	fields := strings.Split(u.Path, "/")
	if len(fields) != 4 {
		return "", "", fmt.Errorf("Issue has invalid RepositoryURL value")
	}
	return fields[2], fields[3], nil

}
