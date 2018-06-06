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
package issues_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/github"

	"github.com/m-lab/alertmanager-github-receiver/issues"
)

type fakeClient struct {
	issues []*github.Issue
}

func (f *fakeClient) ListOpenIssues() ([]*github.Issue, error) {
	return f.issues, nil
}

func TestListHandler(t *testing.T) {
	expected := `
<html><body>
<h1>Open Issues</h1>
<table>

	<tr><td><a href=http://foo.bar>issue1 title</a></td></tr>

</table>
</body></html>`
	f := &fakeClient{
		[]*github.Issue{
			&github.Issue{
				HTMLURL: github.String("http://foo.bar"),
				Title:   github.String("issue1 title"),
			},
		},
	}
	// Create a response recorder.
	rw := httptest.NewRecorder()
	// Create a synthetic request object for ServeHTTP.
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Run the list handler.
	handler := issues.ListHandler{
		Client: f,
	}
	handler.ServeHTTP(rw, req)
	resp := rw.Result()

	// Check the results.
	body, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ListHandler got %d; want %d", resp.StatusCode, http.StatusOK)
	}
	if expected != string(body) {
		t.Errorf("ListHandler got %q; want %q", string(body), expected)
	}
}
