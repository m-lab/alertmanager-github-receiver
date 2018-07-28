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
		name    string
		title   string
		body    string
		wantErr bool
	}{
		{
			name:  "success",
			title: "alert name",
			body:  "foobar",
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
				ID:    got.ID,
			}
			if !reflect.DeepEqual(got, wantIssue) {
				t.Errorf("Client.CreateIssue() = %v, want %v", got, wantIssue)
			}
			list, err := c.ListOpenIssues()
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.ListOpenIssues() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			wantList := []*github.Issue{
				&github.Issue{
					Title: &tt.title,
					Body:  &tt.body,
					ID:    wantIssue.ID,
				},
			}
			if !reflect.DeepEqual(list, wantList) {
				t.Errorf("Client.ListOpenIssues() = %v, want %v", list, wantList)
			}
			closed, err := c.CloseIssue(got)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.CloseIssue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(closed, got) {
				t.Errorf("Client.CloseIssue() = %v, want %v", closed, got)
			}
		})
	}
}
