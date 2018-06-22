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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-github/github"
	"github.com/m-lab/alertmanager-github-receiver/issues"
)

// Global vars for tests.
//
// Tests should register handlers on testMux which provide mock responses for
// the Github API method used by the method under test.
var (

	// testMux is the HTTP request multiplexer used with the test server.
	testMux *http.ServeMux

	// testServer is a test HTTP server used to provide mock API responses.
	testServer *httptest.Server
)

// setupServer starts a new http test server and returns the test server URL.
func setupServer() *url.URL {
	// test server.
	testMux = http.NewServeMux()
	testServer = httptest.NewServer(testMux)

	// Test server URL is guaranteed to parse successfully.
	// The github client library requires that the URL end with a slash.
	url, _ := url.Parse(testServer.URL + "/")
	return url
}

// teardownServer stops the test server.
func teardownServer() {
	testServer.Close()
}

func TestCreateIssue(t *testing.T) {
	client := issues.NewClient("fake-owner", "FAKE-AUTH-TOKEN")
	client.GithubClient.BaseURL = setupServer()
	defer teardownServer()

	title := "my title"
	body := "my issue body"

	testMux.HandleFunc("/repos/fake-owner/fake-repo/issues", func(w http.ResponseWriter, r *http.Request) {
		v := &github.IssueRequest{}
		json.NewDecoder(r.Body).Decode(v)

		authToken := r.Header.Get("Authorization")
		if !strings.Contains(authToken, "FAKE-AUTH-TOKEN") {
			t.Errorf("Request does not contain bearer token")
		}

		if *v.Title != title {
			t.Errorf("Request title = %+v, want %+v", *v.Title, title)
		}
		if *v.Body != body {
			t.Errorf("Request body = %+v, want %+v", *v.Body, body)
		}

		// Fake result.
		fmt.Fprint(w, `{"number":1}`)
	})

	issue, err := client.CreateIssue("fake-repo", title, body, nil)
	if err != nil {
		t.Errorf("CreateIssue returned error: %v", err)
	}

	want := &github.Issue{Number: github.Int(1)}
	if !reflect.DeepEqual(issue, want) {
		t.Errorf("CreateIssue returned %+v, want %+v", issue, want)
	}
}

func TestListOpenIssues(t *testing.T) {
	client := issues.NewClient("owner", "FAKE-AUTH-TOKEN")
	// Override public github API with local server.
	client.GithubClient.BaseURL = setupServer()
	defer teardownServer()

	testMux.HandleFunc("/search/issues", func(w http.ResponseWriter, r *http.Request) {
		// Fake result.
		val := `{
			"total_count":2,
			"incomplete_results":true,
			"items":[{"number":1}, {"number": 2}]
		}`
		fmt.Fprint(w, val)
	})

	issues, err := client.ListOpenIssues()
	if err != nil {
		t.Errorf("ListOpenIssues returned error: %v", err)
	}

	want := []*github.Issue{{Number: github.Int(1)}, {Number: github.Int(2)}}
	if !reflect.DeepEqual(issues, want) {
		t.Errorf("ListOpenIssues returned %+v, want %+v", issues, want)
	}
}

func TestCloseIssue(t *testing.T) {
	client := issues.NewClient("owner", "FAKE-AUTH-TOKEN")
	client.GithubClient.BaseURL = setupServer()
	defer teardownServer()

	u := "https://api.github.com/repos/fake-owner/fake-repo"
	testMux.HandleFunc("/repos/fake-owner/fake-repo/issues/1", func(w http.ResponseWriter, r *http.Request) {
		v := &github.IssueRequest{}
		err := json.NewDecoder(r.Body).Decode(v)
		if err != nil {
			t.Fatal(err)
		}

		// Fake result.
		fmt.Fprintf(w, `{"number":1, "repository_url":"%s"}`, u)
	})

	openIssue := &github.Issue{Number: github.Int(1), RepositoryURL: &u}

	closedIssue, err := client.CloseIssue(openIssue)
	if err != nil {
		t.Errorf("CloseIssue returned error: %v", err)
	}

	if !reflect.DeepEqual(openIssue, closedIssue) {
		t.Errorf("CloseIssue returned %+v, want %+v", closedIssue, openIssue)
	}
}
