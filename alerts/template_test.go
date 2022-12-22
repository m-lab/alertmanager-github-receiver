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
	"fmt"
	"strings"
	"testing"

	"github.com/prometheus/alertmanager/notify/webhook"
	amtmpl "github.com/prometheus/alertmanager/template"
)

func TestFormatIssueBodySimple(t *testing.T) {
	msg := webhook.Message{
		Data: &amtmpl.Data{
			Status: "firing",
			Alerts: []amtmpl.Alert{
				{
					Annotations: amtmpl.KV{"env": "prod", "svc": "foo"},
				},
				{
					Annotations: amtmpl.KV{"env": "stage", "svc": "foo"},
				},
			},
		},
	}
	tests := []struct {
		tmplTxt      string
		expectErrTxt string
		expectOutput string
	}{
		{"foo", "", "foo"},
		{"{{ .Data.Status }}", "", "firing"},
		{"{{ .Status }}", "", "firing"},
		{"{{ range .NOT_REAL_FIELD }}\n* {{.Status}}\n{{end}}", "can't evaluate field NOT_REAL_FIELD in type *webhook.Message", ""},
		{"{{ .Foo }}", "can't evaluate field Foo", ""},
	}

	for testNum, tc := range tests {
		testName := fmt.Sprintf("tc=%d", testNum)
		t.Run(testName, func(t *testing.T) {
			rh, err := NewReceiver(&fakeClient{}, "default", false, "", nil, tc.tmplTxt, tc.tmplTxt)
			if err != nil {
				t.Fatal(err)
			}

			body, err := rh.formatIssueBody(&msg)
			if tc.expectErrTxt == "" && err != nil {
				t.Error(err)
			}
			if tc.expectErrTxt != "" {
				if err == nil {
					t.Error()
				} else if !strings.Contains(err.Error(), tc.expectErrTxt) {
					t.Error(err.Error())
				}
			}
			if tc.expectOutput == "" && body != "" {
				t.Error(body)
			}
			if !strings.Contains(body, tc.expectOutput) {
				t.Error(body)
			}
		})
	}
}


func TestFormatTitleSimple(t *testing.T) {
	msg := webhook.Message{
		Data: &amtmpl.Data{
			Status: "firing",
			Alerts: []amtmpl.Alert{
				{
					Annotations: amtmpl.KV{"env": "prod", "svc": "foo"},
				},
				{
					Annotations: amtmpl.KV{"env": "stage", "svc": "foo"},
				},
			},
		},
	}
	tests := []struct {
		tmplTxt      string
		expectErrTxt string
		expectOutput string
	}{
		{"foo", "", "foo"},
		{"{{ .Data.Status }}", "", "firing"},
		{"{{ .Status }}", "", "firing"},
		{"{{ range .Alerts }}{{ .Annotations.env }} {{ end }}", "", "prod stage "},
		{"{{ .Foo }}", "can't evaluate field Foo", ""},
	}

	for testNum, tc := range tests {
		testName := fmt.Sprintf("tc=%d", testNum)
		t.Run(testName, func(t *testing.T) {
			rh, err := NewReceiver(&fakeClient{}, "default", false, "", nil, tc.tmplTxt, tc.tmplTxt)
			if err != nil {
				t.Fatal(err)
			}

			title, err := rh.formatTitle(&msg)
			if tc.expectErrTxt == "" && err != nil {
				t.Error(err)
			}
			if tc.expectErrTxt != "" {
				if err == nil {
					t.Error()
				} else if !strings.Contains(err.Error(), tc.expectErrTxt) {
					t.Error(err.Error())
				}
			}
			if tc.expectOutput == "" && title != "" {
				t.Error(title)
			}
			if !strings.Contains(title, tc.expectOutput) {
				t.Error(title)
			}
		})
	}
}
