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
package issues

import (
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/github"
)

type fakeClient struct {
	issues []*github.Issue
	err    error
}

func (f *fakeClient) ListOpenIssues() ([]*github.Issue, error) {
	return f.issues, f.err
}
func TestListHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		listClient     ListClient
		method         string
		expectedStatus int
		wantErr        bool
		template       *template.Template
	}{
		{
			name:           "okay",
			method:         http.MethodGet,
			expectedStatus: http.StatusOK,
			listClient: &fakeClient{
				issues: []*github.Issue{
					&github.Issue{
						HTMLURL: github.String("http://foo.bar"),
						Title:   github.String("issue1 title"),
					},
				},
				err: nil,
			},
			template: listTemplate,
		},
		{
			name:           "issues-error",
			method:         http.MethodGet,
			expectedStatus: http.StatusInternalServerError,
			listClient: &fakeClient{
				issues: nil,
				err:    fmt.Errorf("Fake error"),
			},
			template: listTemplate,
		},
		{
			name:           "bad-method",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "bad-template",
			method:         http.MethodGet,
			expectedStatus: http.StatusInternalServerError,
			listClient: &fakeClient{
				issues: []*github.Issue{
					&github.Issue{
						HTMLURL: github.String("http://foo.bar"),
						Title:   github.String("issue1 title"),
					},
				},
				err: nil,
			},
			template: template.Must(template.New("list").Parse(`{{range .}}{{.KeyDoesNotExist}}{{end}}`)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a response recorder.
			rw := httptest.NewRecorder()
			// Create a synthetic request object for ServeHTTP.
			req, err := http.NewRequest(tt.method, "/", nil)
			if err != nil {
				t.Fatal(err)
			}
			lh := &ListHandler{
				ListClient: tt.listClient,
			}
			listTemplate = tt.template
			lh.ServeHTTP(rw, req)
			if rw.Code != tt.expectedStatus {
				t.Errorf("ListHandler wrong status; want %d, got %d", tt.expectedStatus, rw.Code)
			}
		})
	}
}
