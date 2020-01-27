package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/m-lab/go/prometheusx"
	"github.com/m-lab/go/prometheusx/promtest"
)

func TestMetrics(t *testing.T) {
	receiverDuration.WithLabelValues("x")
	promtest.LintMetrics(t)
}

func Test_main(t *testing.T) {
	tests := []struct {
		name         string
		authfile     string
		authtoken    string
		repo         string
		titleTmpl    string
		inmemory     bool
		expectStatus int
	}{
		{
			name:      "okay-default",
			repo:      "fake-repo",
			authtoken: "token",
			inmemory:  false,
		},
		{
			name:     "okay-inmemory",
			authfile: "fake-token",
			repo:     "fake-repo",
			inmemory: true,
		},
		{
			name:         "missing-flags-usage",
			expectStatus: 1,
		},
		{
			name:         "bad-title-tmpl",
			repo:         "fake-repo",
			authtoken:    "token",
			titleTmpl:    "{{ x }}",
			expectStatus: 1,
		},
	}
	flag.CommandLine.SetOutput(ioutil.Discard)
	for _, tt := range tests {
		osExit = func(status int) {
			if status != tt.expectStatus {
				t.Error(status)
			}
		}
		*authtoken = tt.authtoken
		authtokenFile = []byte(tt.authfile)
		*githubOrg = "fake-org"
		*githubRepo = tt.repo
		*enableInMemory = tt.inmemory
		// Guarantee no port conflicts between tests of main.
		*prometheusx.ListenAddress = ":0"
		*receiverAddr = ":0"

		// Create template files.
		if tt.titleTmpl != "" {
			file, err := ioutil.TempFile("", "github-receiver")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file.Name())
			if _, err := fmt.Fprint(file, tt.titleTmpl); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			titleTmplFiles = append(titleTmplFiles, file.Name())
		}

		t.Run(tt.name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			wg.Add(1)
			go func() {
				main()
				wg.Done()
			}()
			cancelCtx()
			wg.Wait()
		})
	}
}
