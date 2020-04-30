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

package local

import (
	"reflect"
	"testing"

	"github.com/google/go-github/github"
)

func TestMemoryClient(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		body         string
		labelIssue   *github.Issue
		label        string
		wantErr      bool
		wantLabelErr bool
	}{
		{
			name:       "success",
			title:      "alert name",
			body:       "foobar",
			labelIssue: &github.Issue{Title: github.String("alert name")},
			label:      "label",
		},
		{
			name:         "failure-label-nonexistent-issue",
			title:        "alert name",
			body:         "foobar",
			labelIssue:   &github.Issue{Title: github.String("other alert")},
			label:        "label",
			wantLabelErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient()
			got, err := c.CreateIssue("fake-repo", tt.title, tt.body, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.CreateIssue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			wantIssue := &github.Issue{
				Title: &tt.title,
				Body:  &tt.body,
			}
			if !reflect.DeepEqual(got, wantIssue) {
				t.Errorf("Client.CreateIssue() = %v, want %v", got, wantIssue)
			}

			wantList := []*github.Issue{
				{
					Title: &tt.title,
					Body:  &tt.body,
				},
			}
			listAndCheck(t, c, tt.wantErr, wantList)

			err = c.LabelIssue(tt.labelIssue, "", true)
			if (err != nil) != tt.wantErr {
				t.Error(err)
			}

			err = c.LabelIssue(tt.labelIssue, tt.label, true)
			if (err != nil) != tt.wantLabelErr {
				t.Error(err)
			}
			if !tt.wantLabelErr {
				wantList[0].Labels = append(wantList[0].Labels, github.Label{Name: &tt.label})
			}
			listAndCheck(t, c, tt.wantErr, wantList)

			err = c.LabelIssue(tt.labelIssue, tt.label, false)
			if (err != nil) != tt.wantLabelErr {
				t.Error(err)
			}
			if !tt.wantLabelErr {
				wantList[0].Labels = []github.Label{}
			}
			listAndCheck(t, c, tt.wantErr, wantList)

			closed, err := c.CloseIssue(got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.CloseIssue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(closed, got) {
				t.Errorf("Client.CloseIssue() = %v, want %v", closed, got)
			}

			_, err = c.CloseIssue(&github.Issue{
				Title: github.String("cannot-close-missing-issue"),
			})
			if err == nil {
				t.Errorf("Client.CloseIssue(), got nil, want error")
			}
		})
	}
}

func listAndCheck(t *testing.T, c *Client, wantErr bool, wantList []*github.Issue) {
	list, err := c.ListOpenIssues()
	if (err != nil) != wantErr {
		t.Errorf("Client.ListOpenIssues() error = %v, wantErr %v", err, wantErr)
		return
	}
	if !reflect.DeepEqual(list, wantList) {
		t.Errorf("Client.ListOpenIssues() =\n%v\n, want\n%v", list, wantList)
	}
}
