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

package alerts

import (
	"encoding/json"
	"strconv"

	"github.com/google/go-github/github"
	//"github.com/kr/pretty"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/prometheus/alertmanager/notify"
)

// ReceiverClient defines all issue operations needed by the ReceiverHandler.
type ReceiverClient interface {
	CloseIssue(issue *github.Issue) (*github.Issue, error)
	CreateIssue(repo, title, body string, extra []string) (*github.Issue, error)
	CreateComment(repo, body string, issueNum int) (*github.IssueComment, error)
	ListOpenIssues() ([]*github.Issue, error)
	GetIssue(repo string, issueID int) (*github.Issue, *github.Response, error)
}

// ReceiverHandler contains data needed for HTTP handlers.
type ReceiverHandler struct {
	// Client is an implementation of the ReceiverClient interface. Client is used
	// to handle requests.
	Client ReceiverClient

	// AutoClose indicates whether resolved issues that are still open should be
	// closed automatically.
	AutoClose bool

	// DefaultRepo is the repository where all alerts without a "repo" label will
	// be created. Repo must exist.
	DefaultRepo string

	// ExtraLabels values will be added to new issues as additional labels.
	ExtraLabels []string
}

// ServeHTTP receives and processes alertmanager notifications. If the alert
// is firing and a github issue does not yet exist, one is created. If the
// alert is resolved and a github issue exists, then it is closed.
func (rh *ReceiverHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Verify that request is a POST.
	if req.Method != http.MethodPost {
		log.Printf("Client used unsupported method: %s: %s", req.Method, req.RemoteAddr)
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Read request body.
	alertBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("Failed to read request body: %s", err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// The WebhookMessage is dependent on alertmanager version. Parse it.
	msg := &notify.WebhookMessage{}
	if err := json.Unmarshal(alertBytes, msg); err != nil {
		log.Printf("Failed to parse webhook message from %s: %s", req.RemoteAddr, err)
		log.Printf("%s", string(alertBytes))
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	// log.Print(pretty.Sprint(msg))

	// Handle the webhook message.
	log.Printf("Handling alert: %s", id(msg))
	if err := rh.processAlert(msg); err != nil {
		log.Printf("Failed to handle alert: %s: %s", id(msg), err)
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("Completed alert: %s", id(msg))
	rw.WriteHeader(http.StatusOK)
	// Empty response.
}

// processAlert processes an alertmanager webhook message.
func (rh *ReceiverHandler) processAlert(msg *notify.WebhookMessage) error {
	var foundIssue *github.Issue
	msgTitle := formatTitle(msg)
	msgBody := formatIssueBody(msg)
	var resp *github.Response
	var issueID int
	var err error

	// When the annotation includes an issue id. Than no need to search by the title.
	for k, v := range msg.CommonAnnotations {
		if k == "issue" {
			if issueID, err = strconv.Atoi(v); err == nil {
				if foundIssue, resp, err = rh.Client.GetIssue(rh.getTargetRepo(msg), issueID); err != nil {
					// We will ignore not found errors.
					if resp != nil && resp.StatusCode == http.StatusNotFound {
						break
					}
					return err
				}
			} else {
				return err
			}
			break
		}
	}

	// Didn't find the issue by the issue ID so now try by it's title.
	if foundIssue == nil {
		// TODO(dev): replace list-and-search with search using labels.
		// TODO(dev): Cache list results.
		// List known issues from github.
		issues, err := rh.Client.ListOpenIssues()
		if err != nil {
			return err
		}

		// Search for an issue that matches the notification message from AM.
		for _, issue := range issues {
			if msgTitle == *issue.Title {
				log.Printf("Found matching issue: %s\n", msgTitle)
				foundIssue = issue
				break
			}
		}
	}

	// The message is currently firing and we found  a matching issue so post a comment update.
	if msg.Data.Status == "firing" && foundIssue != nil {
		_, err := rh.Client.CreateComment(rh.getTargetRepo(msg), msgBody, int(issueID))
		return err
	}

	// The message is currently firing and we did not find a matching
	// issue from github, so create a new issue.
	if msg.Data.Status == "firing" && foundIssue == nil {
		_, err := rh.Client.CreateIssue(rh.getTargetRepo(msg), msgTitle, msgBody, rh.ExtraLabels)
		return err
	}

	// The message is resolved and we found a matching open issue from github.
	// If AutoClose is true, then close the issue.
	if msg.Data.Status == "resolved" && foundIssue != nil && rh.AutoClose {
		// NOTE: there can be multiple "resolved" messages for the same
		// alert. Prometheus evaluates rules every `evaluation_interval`.
		// And, alertmanager preserves an alert until `resolve_timeout`. So
		// expect (resolve_timeout / evaluation_interval) messages.
		_, err := rh.Client.CloseIssue(foundIssue)
		return err
	}

	// log.Printf("Unsupported WebhookMessage.Data.Status: %s", msg.Data.Status)
	return nil
}

// getTargetRepo returns a suitable github repository for creating an issue for
// the given alert message. If the alert includes a "repo" label, then getTargetRepo
// uses that value. Otherwise, getTargetRepo uses the ReceiverHandler's default repo.
func (rh *ReceiverHandler) getTargetRepo(msg *notify.WebhookMessage) string {
	repo := msg.CommonLabels["repo"]
	if repo != "" {
		return repo
	}
	return rh.DefaultRepo
}
