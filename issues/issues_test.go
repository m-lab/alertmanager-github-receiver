package issues_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/google/go-github/github"
	"src/github.com/stephen-soltesz/alertmanager-github-receiver/issues"
)

// Adapted from https://github.com/google/go-github/blob/master/github/github_test.go#L39

// Global vars for tests.
//
// Tests should register handlers on mux which provide mock responses for the
// API method being tested.
var (
	// mux is the HTTP request multiplexer used with the test server.
	mux *http.ServeMux

	// server is a test HTTP server used to provide mock API responses.
	server *httptest.Server
)

func setup(client *issues.Client) {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)

	url, _ := url.Parse(server.URL)
	client.GithubClient.BaseURL = url

	// github client configured to use test server
	// client = NewClient(nil)
	// client.BaseURL = url
	// client.UploadURL = url
}

func teardown() {
	server.Close()
}

func TestCreateIssue(t *testing.T) {
	client, err := issues.NewClient("owner", "repo", "fake-auth-token")
	if err != nil {
		t.Fatal("Failed to create new client.")
	}
	setup(client)
	defer teardown()

	title := "my title"
	body := "my issue body"

	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		v := &github.IssueRequest{}
		json.NewDecoder(r.Body).Decode(v)

		if *v.Title != title {
			t.Errorf("Request title = %+v, want %+v", *v.Title, title)
		}
		if *v.Body != body {
			t.Errorf("Request body = %+v, want %+v", *v.Body, body)
		}

		// Fake result.
		fmt.Fprint(w, `{"number":1}`)
	})

	issue, err := client.CreateIssue(title, body)
	if err != nil {
		t.Errorf("Issues.Create returned error: %v", err)
	}

	want := &github.Issue{Number: github.Int(1)}
	if !reflect.DeepEqual(issue, want) {
		t.Errorf("Issues.Create returned %+v, want %+v", issue, want)
	}

}
