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

// A client interface wrapping the Github API for creating, listing, and closing
// issues on a single repository.
package issues

import (
	"fmt"
	"log"

	"github.com/google/go-github/github"
	"github.com/kr/pretty"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

// A Client manages communication with the Github API.
type Client struct {
	// githubClient is an authenticated client for accessing the github API.
	GithubClient *github.Client
	// owner is the github project (e.g. github.com/<owner>/<repo>).
	owner string
	// repo is the github repository under the above owner.
	repo string
}

// NewClient creates an Client authenticated using the Github authToken.
// Future operations are only performed on the given github "owner/repo".
func NewClient(owner, repo, authToken string) (*Client, error) {
	if authToken == "" {
		return nil, fmt.Errorf("Authentication Token must not be empty.")
	}
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: authToken},
	)
	client := &Client{
		GithubClient: github.NewClient(oauth2.NewClient(ctx, tokenSource)),
		owner:        owner,
		repo:         repo,
	}
	return client, nil
}

// CreateIssue creates a new Github issue. New issues are unassigned.
func (c *Client) CreateIssue(title, body string) (*github.Issue, error) {
	// Construct a minimal github issue request.
	issueReq := github.IssueRequest{
		Title: &title,
		Body:  &body,
	}

	// Create the issue.
	// See also: https://developer.github.com/v3/issues/#create-an-issue
	// See also: https://godoc.org/github.com/google/go-github/github#IssuesService.Create
	issue, resp, err := c.GithubClient.Issues.Create(
		context.Background(), c.owner, c.repo, &issueReq)
	if err != nil {
		log.Printf("Error in CreateIssue: response: %v\n%s",
			err, pretty.Sprint(resp))
		return nil, err
	}
	return issue, nil
}

// ListOpenIssues returns open issues from github Github issues are either
// "open" or "closed". Closed issues have either been resolved automatically or
// by a person. So, there will be an ever increasing number of "closed" issues.
// By only listing "open" issues we limit the number of issues returned.
func (c *Client) ListOpenIssues() ([]*github.Issue, error) {
	var allIssues []*github.Issue

	opts := &github.IssueListByRepoOptions{State: "open"}
	for {
		issues, resp, err := c.GithubClient.Issues.ListByRepo(
			context.Background(), c.owner, c.repo, opts)
		if err != nil {
			log.Printf("Failed to list open github issues: %v\n%s",
				err, pretty.Sprint(resp))
			return nil, err
		}
		// Collect 'em all.
		allIssues = append(allIssues, issues...)

		// Continue loading the next page until all issues are received.
		if resp.NextPage == 0 {
			break
		}
		opts.ListOptions.Page = resp.NextPage
	}
	return allIssues, nil
}

// CloseIssue changes the issue state to "closed" unconditionally. If the issue
// is already close, then this should have no effect.
func (c *Client) CloseIssue(issue *github.Issue) (*github.Issue, error) {
	issueReq := github.IssueRequest{
		State: github.String("closed"),
	}

	// Edits the issue to have "closed" state.
	// See also: https://developer.github.com/v3/issues/#edit-an-issue
	// See also: https://godoc.org/github.com/google/go-github/github#IssuesService.Edit
	closedIssue, resp, err := c.GithubClient.Issues.Edit(
		context.Background(), c.owner, c.repo, *issue.Number, &issueReq)
	if err != nil {
		log.Printf("Failed to close issue: %v\n%s", err, pretty.Sprint(resp))
		return nil, err
	}
	return closedIssue, nil
}
