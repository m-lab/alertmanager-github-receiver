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
	"strconv"
	"strings"
	"text/template"

	amTemplate "github.com/prometheus/alertmanager/template"

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
	AssignIssueToProject(issue *github.Issue, columnId int64) (*github.ProjectCard, error)
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
}

// GithubInfo contains information related to assigning new/existing issues to
// projects, and ensuring they have the appropriate labels.
type GithubInfo struct {
	AdditionalLabels []string
	ProjectColumnId  int64
}

// NewReceiver creates a new ReceiverHandler.
func NewReceiver(client ReceiverClient, githubRepo string, autoClose bool, resolvedLabel string, extraLabels []string, titleTmplStr string) (*ReceiverHandler, error) {
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

	// parse the additional github information - will manage any parsing error returned after
	// the issue is created - as we need the list of string labels as part of the creation
	additionalGithubInfo := parseAdditionalGithubInfo(&msg.Data.Alerts[0].Annotations)

	// extract labels from the alert annotations and add them to the pre-configured labels
	allLabels := append(rh.ExtraLabels, additionalGithubInfo.AdditionalLabels...)

	// The message is currently firing and we did not find a matchingissue from github, so create a new issue.
	if msg.Data.Status == "firing" {
		if foundIssue == nil {
			msgBody := formatIssueBody(msg)
			foundIssue, err = rh.Client.CreateIssue(rh.getTargetRepo(msg), msgTitle, msgBody, allLabels)
			if err == nil {
				createdIssues.WithLabelValues(alertName).Inc()
			}

			// attempt to assign to a project card, if a valid project column ID was passed
			// only assign to a project when a new issue is created
			if additionalGithubInfo.ProjectColumnId != 0 {
				_, err := rh.Client.AssignIssueToProject(foundIssue, additionalGithubInfo.ProjectColumnId)
				if err != nil {
					log.Println(err)
				}
			}
		} else {
			// remove the resolved label since this refired - the false value triggers the removal
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

// extractLabels transforms a KV pair named "github-labels" from the alert's annotations
// and returns them as a slice of github.Labels
func parseAdditionalGithubInfo(annotations *amTemplate.KV) *GithubInfo {
	var ghProjectInfo GithubInfo

	// grab the labels
	if lblStr, ok := (*annotations)["github-labels"]; ok {
		ghProjectInfo.AdditionalLabels = strings.Split(lblStr, ",")
	}

	// grab the column id for the project card
	columnIdStr, _ := (*annotations)["github-project-column-id"]
	columnId, err := strconv.ParseInt(columnIdStr, 10, 64)

	if err != nil {
		ghProjectInfo.ProjectColumnId = 0
		log.Printf("Invalid Project Column ID passed via annotations. Issue will not be assigned to a project.")
	} else {
		ghProjectInfo.ProjectColumnId = columnId
	}

	return &ghProjectInfo
}
