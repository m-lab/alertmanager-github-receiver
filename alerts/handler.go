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

	"github.com/google/go-github/github"
	//"github.com/kr/pretty"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/prometheus/alertmanager/notify"
)

type ReceiverClient interface {
	CloseIssue(issue *github.Issue) (*github.Issue, error)
	CreateIssue(repo, title, body string) (*github.Issue, error)
	ListOpenIssues() ([]*github.Issue, error)
}

type ReceiverHandler struct {
	// Client is an implementation of the ReceiverClient interface. Client is used to handle requests.
	Client ReceiverClient

	// AutoClose indicates whether resolved issues that are still open should be closed automatically.
	AutoClose bool

	// DefaultRepo all alerts without a "repo" label will be created in this repository.
	// DefaultRepo must exist.
	DefaultRepo string
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
	// TODO(dev): replace list-and-search with search using labels.
	// TODO(dev): Cache list results.
	// List known issues from github.
	issues, err := rh.Client.ListOpenIssues()
	if err != nil {
		return err
	}

	// Search for an issue that matches the notification message from AM.
	msgTitle := formatTitle(msg)
	var foundIssue *github.Issue
	for _, issue := range issues {
		if msgTitle == *issue.Title {
			log.Printf("Found matching issue: %s\n", msgTitle)
			foundIssue = issue
			break
		}
	}

	// The message is currently firing and we did not find a matching
	// issue from github, so create a new issue.
	if msg.Data.Status == "firing" && foundIssue == nil {
		msgBody := formatIssueBody(msg)
		_, err := rh.Client.CreateIssue(rh.getTargetRepo(msg), msgTitle, msgBody)
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

func (rh *ReceiverHandler) getTargetRepo(msg *notify.WebhookMessage) string {
	repo := msg.CommonLabels["repo"]
	if repo != "" {
		return repo
	}
	return rh.DefaultRepo
}
