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
	"hash/fnv"
	"log"

	"github.com/google/go-github/github"
)

// Client manages operations on the in memory store.
type Client struct {
	issues   map[string]*github.Issue
	comments map[int]map[int64]*github.IssueComment
}

// NewClient creates a Client.
func NewClient() *Client {
	return &Client{
		issues:   make(map[string]*github.Issue),
		comments: make(map[int]map[int64]*github.IssueComment),
	}
}

// CreateIssue adds a new issue to the in memory store.
func (c *Client) CreateIssue(repo, title, body string, extra []string) (*github.Issue, error) {
	id := generateID(title)
	c.issues[title] = &github.Issue{
		Title: &title,
		Body:  &body,
		ID:    &id,
	}
	return c.issues[title], nil
}

// CreateComment adds a new comment to the in memory store.
func (c *Client) CreateComment(repo, body string, issueNum int) (*github.IssueComment, error) {
	id := generateID(body)
	comment := &github.IssueComment{
		Body: &body,
		ID:   &id,
	}
	c.comments[issueNum][id] = comment
	return comment, nil
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

// GetIssue by its ID.
func (c *Client) GetIssue(repo string, issueID int) (*github.Issue, error) {
	for _, i := range c.issues {
		if *i.ID == int64(issueID) {
			return i, nil
		}
	}
	return nil, fmt.Errorf("issue not found")
}

// Generate an ID form the title's hash.
func generateID(s string) int64 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return int64(h.Sum32())
}
