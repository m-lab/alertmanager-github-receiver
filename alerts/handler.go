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
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"text/template"

	"github.com/google/go-github/github"
	"github.com/prometheus/alertmanager/notify/webhook"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	receivedAlerts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "githubreceiver_alerts_total",
			Help: "Number of incoming alerts from AlertManager.",
		},
		[]string{"alertname", "status"},
	)

	createdIssues = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "githubreceiver_created_issues_total",
			Help: "Number of firing issues for which an alert has been created.",
		},
		// Here "status" can only be "firing" thus it's not a label.
		[]string{"alertname"},
	)
)

// ReceiverClient defines all issue operations needed by the ReceiverHandler.
type ReceiverClient interface {
	CloseIssue(issue *github.Issue) (*github.Issue, error)
	CreateIssue(repo, title, body string, extra []string) (*github.Issue, error)
	LabelIssue(issue *github.Issue, label string, add bool) error
	ListOpenIssues() ([]*github.Issue, error)
}

// ReceiverHandler contains data needed for HTTP handlers.
type ReceiverHandler struct {
	// Client is an implementation of the ReceiverClient interface. Client is used
	// to handle requests.
	Client ReceiverClient

	// AutoClose indicates whether resolved issues that are still open should be
	// closed automatically.
	AutoClose bool

	// ResolvedLabel is applied to issues when their corresponding alerts are
	// resolved.
	ResolvedLabel string

	// DefaultRepo is the repository where all alerts without a "repo" label will
	// be created. Repo must exist.
	DefaultRepo string

	// ExtraLabels values will be added to new issues as additional labels.
	ExtraLabels []string

	// titleTmpl is used to format the title of the new issue.
	titleTmpl *template.Template

	// alertTmpl is used to format the context of the new issue.
	alertTmpl *template.Template
}

// NewReceiver creates a new ReceiverHandler.
func NewReceiver(client ReceiverClient, githubRepo string, autoClose bool, resolvedLabel string, extraLabels []string, titleTmplStr string, alertTmplStr string) (*ReceiverHandler, error) {
	rh := ReceiverHandler{
		Client:        client,
		DefaultRepo:   githubRepo,
		AutoClose:     autoClose,
		ResolvedLabel: resolvedLabel,
		ExtraLabels:   extraLabels,
	}

	var err error
	rh.titleTmpl, err = template.New("title").Parse(titleTmplStr)
	if err != nil {
		return nil, err
	}

	rh.alertTmpl, err = template.New("alert").Parse(alertTmplStr)
	if err != nil {
		return nil, err
	}

	return &rh, nil
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
	msg := &webhook.Message{}
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
func (rh *ReceiverHandler) processAlert(msg *webhook.Message) error {
	// TODO(dev): Cache list results.
	// List known issues from github.
	issues, err := rh.Client.ListOpenIssues()
	if err != nil {
		return err
	}

	// Search for an issue that matches the notification message from AM.
	msgTitle, err := rh.formatTitle(msg)
	if err != nil {
		return fmt.Errorf("format title for %q: %s", msg.GroupKey, err)
	}
	var foundIssue *github.Issue
	for _, issue := range issues {
		if msgTitle == *issue.Title {
			log.Printf("Found matching issue: %s\n", msgTitle)
			foundIssue = issue
			break
		}
	}

	var alertName = msg.Data.GroupLabels["alertname"]
	receivedAlerts.WithLabelValues(alertName, msg.Data.Status).Inc()

	// The message is currently firing and we did not find a matching
	// issue from github, so create a new issue.
	if msg.Data.Status == "firing" {
		if foundIssue == nil {
			msgBody, err := rh.formatIssueBody(msg)
			if err != nil {
				return fmt.Errorf("format body for %q: %s", msg.GroupKey, err)
			}
			_, err = rh.Client.CreateIssue(rh.getTargetRepo(msg), msgTitle, msgBody, rh.ExtraLabels)
			if err == nil {
				createdIssues.WithLabelValues(alertName).Inc()
			}
		} else {
			err = rh.Client.LabelIssue(foundIssue, rh.ResolvedLabel, false)
		}
		return err
	}

	// The message is resolved and we found a matching open issue from github.
	// If AutoClose is true, then close the issue.
	if msg.Data.Status == "resolved" && foundIssue != nil {
		// NOTE: there can be multiple "resolved" messages for the same
		// alert. Prometheus evaluates rules every `evaluation_interval`.
		// And, alertmanager preserves an alert until `resolve_timeout`. So
		// expect (resolve_timeout / evaluation_interval) messages.
		err := rh.Client.LabelIssue(foundIssue, rh.ResolvedLabel, true)
		if err != nil {
			return err
		}
		if rh.AutoClose {
			_, err := rh.Client.CloseIssue(foundIssue)
			return err
		}
	}

	// log.Printf("Unsupported WebhookMessage.Data.Status: %s", msg.Data.Status)
	return nil
}

// getTargetRepo returns a suitable github repository for creating an issue for
// the given alert message. If the alert includes a "repo" label, then getTargetRepo
// uses that value. Otherwise, getTargetRepo uses the ReceiverHandler's default repo.
func (rh *ReceiverHandler) getTargetRepo(msg *webhook.Message) string {
	repo := msg.CommonLabels["repo"]
	if repo != "" {
		return repo
	}
	return rh.DefaultRepo
}
