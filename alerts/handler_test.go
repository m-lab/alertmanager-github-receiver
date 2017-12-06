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
package alerts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/github"

	"github.com/m-lab/alertmanager-github-receiver/alerts"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
)

type fakeClient struct {
	listIssues   []*github.Issue
	createdIssue *github.Issue
	closedIssue  *github.Issue
}

func (f *fakeClient) ListOpenIssues() ([]*github.Issue, error) {
	fmt.Println("list open issues")
	return f.listIssues, nil
}

func (f *fakeClient) CreateIssue(title, body string) (*github.Issue, error) {
	fmt.Println("create issue")
	f.createdIssue = createIssue(title, body)
	return f.createdIssue, nil
}

func (f *fakeClient) CloseIssue(issue *github.Issue) (*github.Issue, error) {
	fmt.Println("close issue")
	f.closedIssue = issue
	return issue, nil
}

func createWebhookMessage(alertname, status string) *bytes.Buffer {
	msg := &notify.WebhookMessage{
		Data: &template.Data{
			Receiver: "webhook",
			Status:   status,
			Alerts: template.Alerts{
				template.Alert{
					Status:       status,
					Labels:       template.KV{"dev": "sda3", "instance": "example4", "alertname": alertname},
					Annotations:  template.KV{"description": "This is how to handle the alert"},
					StartsAt:     time.Unix(1498614000, 0),
					GeneratorURL: "http://generator.url/",
				},
			},
			GroupLabels:  template.KV{"alertname": alertname},
			CommonLabels: template.KV{"alertname": alertname},
			ExternalURL:  "http://localhost:9093",
		},
		Version:  "4",
		GroupKey: fmt.Sprintf("{}:{alertname=\"%s\"}", alertname),
	}
	if status == "resolved" {
		msg.Data.Alerts[0].EndsAt = time.Unix(1498618000, 0)
	}
	b, _ := json.Marshal(msg)
	return bytes.NewBuffer(b)
	// return msg
}

func createIssue(title, body string) *github.Issue {
	return &github.Issue{
		Title: github.String(title),
		Body:  github.String(body),
	}
}

func TestReceiverHandler(t *testing.T) {
	// Test: resolve an existing issue.
	// * msg is "resolved"
	// * issue returned by list
	// * issue is closed
	postBody := createWebhookMessage("DiskRunningFull", "resolved")
	// Create a response recorder.
	rw := httptest.NewRecorder()
	// Create a synthetic request object for ServeHTTP.
	req, err := http.NewRequest("POST", "/v1/receiver", postBody)
	if err != nil {
		t.Fatal(err)
	}

	// Provide a pre-existing issue to close.
	f := &fakeClient{
		listIssues: []*github.Issue{
			createIssue("DiskRunningFull", "body1"),
		},
	}
	handler := alerts.ReceiverHandler{f, true}
	handler.ServeHTTP(rw, req)
	resp := rw.Result()

	// Check the results.
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ReceiverHandler got %d; want %d", resp.StatusCode, http.StatusOK)
	}
	if f.closedIssue == nil {
		t.Fatalf("ReceiverHandler failed to close issue")
	}
	if *f.closedIssue.Title != "DiskRunningFull" {
		t.Errorf("ReceiverHandler closed wrong issue; got %q want \"DiskRunningFull\"",
			*f.closedIssue.Title)
	}
	t.Logf("body: %s", body)

	// Test: create a new issue.
	// * msg is "firing"
	// * issue list is empty.
	// * issue is created
	postBody = createWebhookMessage("DiskRunningFull", "firing")
	// Create a response recorder.
	rw = httptest.NewRecorder()
	// Create a synthetic request object for ServeHTTP.
	req, err = http.NewRequest("POST", "/v1/receiver", postBody)
	if err != nil {
		t.Fatal(err)
	}

	// No pre-existing issues to close.
	f = &fakeClient{}
	handler = alerts.ReceiverHandler{f, true}
	handler.ServeHTTP(rw, req)
	resp = rw.Result()

	// Check the results.
	body, _ = ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ReceiverHandler got %d; want %d", resp.StatusCode, http.StatusOK)
	}
	if f.createdIssue == nil {
		t.Fatalf("ReceiverHandler failed to close issue")
	}
	if *f.createdIssue.Title != "DiskRunningFull" {
		t.Errorf("ReceiverHandler closed wrong issue; got %q want \"DiskRunningFull\"",
			*f.closedIssue.Title)
	}
	t.Logf("body: %s", body)
}
