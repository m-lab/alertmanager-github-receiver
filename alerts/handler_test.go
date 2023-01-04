// Copyright 2017 alertmanager-github-receiver Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// ////////////////////////////////////////////////////////////////////////////
package alerts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/m-lab/go/prometheusx/promtest"

	"github.com/google/go-github/github"
	"github.com/prometheus/alertmanager/notify/webhook"
	"github.com/prometheus/alertmanager/template"
)

type fakeClient struct {
	listIssues   []*github.Issue
	createdIssue *github.Issue
	closedIssue  *github.Issue
	listError    error
	labelError   error
}

func (f *fakeClient) ListOpenIssues() ([]*github.Issue, error) {
	fmt.Println("list open issues")
	if f.listError != nil {
		return nil, f.listError
	}
	return f.listIssues, nil
}

func (f *fakeClient) LabelIssue(issue *github.Issue, label string, add bool) error {
	fmt.Println("label issue")
	if f.labelError != nil {
		return f.labelError
	}
	return nil
}

func (f *fakeClient) CreateIssue(repo, title, body string, extra []string) (*github.Issue, error) {
	fmt.Println("create issue")
	f.createdIssue = createIssue(title, body, repo)
	return f.createdIssue, nil
}

func (f *fakeClient) CloseIssue(issue *github.Issue) (*github.Issue, error) {
	fmt.Println("close issue")
	f.closedIssue = issue
	return issue, nil
}

func createWebhookMessage(alertname, status, repo string) *webhook.Message {
	msg := &webhook.Message{
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
			CommonLabels: template.KV{"alertname": alertname, "repo": repo},
			ExternalURL:  "http://localhost:9093",
		},
		Version:  "4",
		GroupKey: fmt.Sprintf("{}:{alertname=\"%s\"}", alertname),
	}
	if status == "resolved" {
		msg.Data.Alerts[0].EndsAt = time.Unix(1498618000, 0)
	}
	return msg
}

func marshalWebhookMessage(msg *webhook.Message) *bytes.Buffer {
	b, _ := json.Marshal(msg)
	return bytes.NewBuffer(b)
}

func createIssue(title, body, repo string) *github.Issue {
	return &github.Issue{
		Title:         github.String(title),
		Body:          github.String(body),
		RepositoryURL: github.String(repo),
	}
}

type errorReader struct {
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("Fake error")
}

func TestReceiverHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name              string
		method            string
		msgAlert          string
		msgAlertStatus    string
		msgRepo           string
		fakeClient        *fakeClient
		titleTmpl         string
		alertTmpl         string
		httpStatus        int
		expectReceiverErr bool
		wantMessageErr    bool
		wantReadErr       bool
	}{
		{
			name:           "successful-close",
			method:         http.MethodPost,
			msgAlert:       "DiskRunningFull",
			msgAlertStatus: "resolved",
			fakeClient: &fakeClient{
				listIssues: []*github.Issue{
					createIssue("DiskRunningFull", "body1", ""),
				},
			},
			titleTmpl:  DefaultTitleTmpl,
			alertTmpl:  DefaultAlertTmpl,
			httpStatus: http.StatusOK,
		},
		{
			name:           "successful-resolved-after-closed",
			method:         http.MethodPost,
			msgAlert:       "DiskRunningFull",
			msgAlertStatus: "resolved",
			fakeClient:     &fakeClient{},
			titleTmpl:      DefaultTitleTmpl,
			alertTmpl:      DefaultAlertTmpl,
			httpStatus:     http.StatusOK,
		},
		{
			name:           "successful-create",
			method:         http.MethodPost,
			msgAlert:       "DiskRunningFull",
			msgAlertStatus: "firing",
			fakeClient:     &fakeClient{},
			titleTmpl:      DefaultTitleTmpl,
			alertTmpl:      DefaultAlertTmpl,
			httpStatus:     http.StatusOK,
		},
		{
			name:           "successful-create-with-explicit-repo",
			method:         http.MethodPost,
			msgAlert:       "DiskRunningFull",
			msgAlertStatus: "firing",
			msgRepo:        "custom-repo",
			fakeClient:     &fakeClient{},
			titleTmpl:      DefaultTitleTmpl,
			alertTmpl:      DefaultAlertTmpl,
			httpStatus:     http.StatusOK,
		},
		{
			name:           "successful-ignore-existing-issue-for-firing-alert",
			method:         http.MethodPost,
			msgAlert:       "DiskRunningFull",
			msgAlertStatus: "firing",
			fakeClient: &fakeClient{
				listIssues: []*github.Issue{
					createIssue("DiskRunningFull", "body1", ""),
				},
			},
			titleTmpl:  DefaultTitleTmpl,
			alertTmpl:  DefaultAlertTmpl,
			httpStatus: http.StatusOK,
		},
		{
			name:           "successful-title-template",
			method:         http.MethodPost,
			msgAlert:       "DiskRunningFull",
			msgAlertStatus: "firing",
			fakeClient: &fakeClient{
				listIssues: []*github.Issue{
					createIssue("DiskRunningFull", "body1", ""),
				},
			},
			titleTmpl:  `{{ (index .Data.Alerts 0).Labels.alertname }}`,
			alertTmpl:  `Disk is running full on {{ (index .Data.Alerts 0).Labels.instance }}`,
			httpStatus: http.StatusOK,
		},
		{
			name:           "failure-resolved-label",
			method:         http.MethodPost,
			msgAlert:       "DiskRunningFull",
			msgAlertStatus: "resolved",
			fakeClient: &fakeClient{
				listIssues: []*github.Issue{
					createIssue("DiskRunningFull", "body", ""),
				},
				labelError: errors.New("No such label"),
			},
			titleTmpl:  DefaultTitleTmpl,
			alertTmpl:  DefaultAlertTmpl,
			httpStatus: http.StatusInternalServerError,
		},
		{
			name:           "failure-title-template-bad-index",
			method:         http.MethodPost,
			msgAlert:       "DiskRunningFull",
			msgAlertStatus: "firing",
			fakeClient: &fakeClient{
				listIssues: []*github.Issue{
					createIssue("DiskRunningFull", "body1", ""),
				},
			},
			titleTmpl:  `{{ (index .Data.Alerts 1).Status }}`,
			alertTmpl:  DefaultAlertTmpl,
			httpStatus: http.StatusInternalServerError,
		},
		{
			name:           "failure-title-template-bad-syntax",
			method:         http.MethodPost,
			msgAlert:       "DiskRunningFull",
			msgAlertStatus: "firing",
			fakeClient: &fakeClient{
				listIssues: []*github.Issue{
					createIssue("DiskRunningFull", "body1", ""),
				},
			},
			titleTmpl:         `{{ x }}`,
			alertTmpl:         DefaultAlertTmpl,
			expectReceiverErr: true,
			httpStatus:        http.StatusInternalServerError,
		},
		{
			name:           "failure-alert-template-bad-syntax",
			method:         http.MethodPost,
			msgAlert:       "DiskRunningFull",
			msgAlertStatus: "firing",
			fakeClient: &fakeClient{
				listIssues: []*github.Issue{
					createIssue("DiskRunningFull", "body1", ""),
				},
			},
			titleTmpl:         `{{ (index .Data.Alerts 1).Status }}`,
			alertTmpl:         `{{ x }}`,
			expectReceiverErr: true,
			httpStatus:        http.StatusInternalServerError,
		},
		{
			name:           "failure-unmarshal-error",
			method:         http.MethodPost,
			titleTmpl:      DefaultTitleTmpl,
			alertTmpl:      DefaultAlertTmpl,
			httpStatus:     http.StatusBadRequest,
			wantMessageErr: true,
		},
		{
			name:        "failure-reader-error",
			method:      http.MethodPost,
			titleTmpl:   DefaultTitleTmpl,
			alertTmpl:   DefaultAlertTmpl,
			httpStatus:  http.StatusInternalServerError,
			wantReadErr: true,
		},
		{
			name:   "failure-list-error",
			method: http.MethodPost,
			fakeClient: &fakeClient{
				listError: fmt.Errorf("Fake error listing current issues"),
			},
			titleTmpl:  DefaultTitleTmpl,
			alertTmpl:  DefaultAlertTmpl,
			httpStatus: http.StatusInternalServerError,
		},
		{
			name:       "failure-wrong-method",
			method:     http.MethodGet,
			titleTmpl:  DefaultTitleTmpl,
			alertTmpl:  DefaultAlertTmpl,
			httpStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "failure-body-template",
			method:         http.MethodPost,
			msgAlert:       "DiskRunningFull",
			msgAlertStatus: "firing",
			fakeClient:     &fakeClient{},
			titleTmpl:      DefaultTitleTmpl,
			alertTmpl:      `{{ .NOTAREAL_FIELD }}`,
			httpStatus:     http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate fake webhook message buffer.
			msg := marshalWebhookMessage(createWebhookMessage(tt.msgAlert, tt.msgAlertStatus, tt.msgRepo))
			if tt.wantMessageErr {
				// Deliberately corrupt the json content by adding extra braces.
				msg.Write([]byte{'}', '{'})
			}

			// Convert the webhook message into an io.Reader.
			var msgReader io.Reader
			msgReader = msg
			if tt.wantReadErr {
				// Override the reader to return an error on read.
				msgReader = &errorReader{}
			}

			// Create a response recorder.
			rw := httptest.NewRecorder()
			// Create a synthetic request that sends an alertmanager webhook message.
			req, err := http.NewRequest(tt.method, "/v1/receiver", msgReader)
			if err != nil {
				t.Fatal(err)
				return
			}

			rh, err := NewReceiver(tt.fakeClient, "default", true, "", nil, tt.titleTmpl, tt.alertTmpl)
			if tt.expectReceiverErr {
				if err == nil {
					t.Fatal()
				}
				return
			} else if err != nil {
				t.Fatal(err)
			}
			rh.ServeHTTP(rw, req)
			resp := rw.Result()

			// Check the results.
			body, _ := ioutil.ReadAll(resp.Body)
			if resp.StatusCode != tt.httpStatus {
				t.Errorf("ReceiverHandler got %d; want %d", resp.StatusCode, tt.httpStatus)
			}
			if tt.fakeClient != nil && tt.fakeClient.closedIssue != nil {
				if *tt.fakeClient.closedIssue.Title != tt.msgAlert {
					t.Errorf("ReceiverHandler closed wrong issue; got %q want %q",
						*tt.fakeClient.closedIssue.Title, tt.msgAlert)
				}
			}
			if tt.fakeClient != nil && tt.fakeClient.createdIssue != nil {
				if *tt.fakeClient.createdIssue.Title != tt.msgAlert {
					t.Errorf("ReceiverHandler created wrong issue; got %q want %q",
						*tt.fakeClient.createdIssue.Title, tt.msgAlert)
				}
				if tt.msgRepo != "" && *tt.fakeClient.createdIssue.RepositoryURL != tt.msgRepo {
					t.Errorf("ReceiverHandler created wrong repo; got %q want %q",
						*tt.fakeClient.createdIssue.RepositoryURL, tt.msgRepo)
				}
			}
			if string(body) != "" {
				t.Errorf("ReceiverHandler got %q; want empty body", string(body))
			}
		})
	}
}

func TestMetrics(t *testing.T) {
	receivedAlerts.WithLabelValues("x", "y")
	promtest.LintMetrics(t)
}
