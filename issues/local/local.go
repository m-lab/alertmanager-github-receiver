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

// Package memory provides in memory operations on GitHub issues.
package local

import (
	"fmt"
	"log"

	"github.com/google/go-github/github"
)

// Client manages operations on the in memory store.
type Client struct {
	issues map[string]*github.Issue
}

// NewClient creates a Client.
func NewClient() *Client {
	return &Client{
		issues: make(map[string]*github.Issue),
	}
}

// CreateIssue adds a new issue to the in memory store.
func (c *Client) CreateIssue(repo, title, body string, extra []string) (*github.Issue, error) {
	c.issues[title] = &github.Issue{
		Title: &title,
		Body:  &body,
	}
	return c.issues[title], nil
}

// ListOpenIssues returns all issues in the memory store.
func (c *Client) ListOpenIssues() ([]*github.Issue, error) {
	var allIssues []*github.Issue
	for title := range c.issues {
		log.Println("ListOpenIssues:", title)
		allIssues = append(allIssues, c.issues[title])
	}
	return allIssues, nil
}

// CloseIssue removes the issue from the in memory store.
func (c *Client) CloseIssue(issue *github.Issue) (*github.Issue, error) {
	if _, ok := c.issues[issue.GetTitle()]; !ok {
		return nil, fmt.Errorf("Unknown issue:%s", issue.GetTitle())
	}
	delete(c.issues, issue.GetTitle())
	return issue, nil
}
