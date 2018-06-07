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

// github_receiver accepts Alertmanager webhook notifications and creates or
// closes corresponding issues on Github.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/m-lab/alertmanager-github-receiver/alerts"
	"github.com/m-lab/alertmanager-github-receiver/issues"
	// TODO: add prometheus metrics for errors and github api access.
)

var (
	authtoken       = flag.String("authtoken", "", "Oauth2 token for access to github API.")
	githubOrg       = flag.String("org", "", "The github user or organization name where all repos are found.")
	githubRepo      = flag.String("repo", "", "The default repository for creating issues when alerts do not include a repo label.")
	enableAutoClose = flag.Bool("enable-auto-close", false, "Once an alert stops firing, automatically close open issues.")
)

const (
	usage = `
Usage of %s:

Github receiver requires a github --authtoken and target github --owner and
--repo names.

`
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}
}

func serveListener(client *issues.Client) {
	http.Handle("/", &issues.ListHandler{ListClient: client})
	http.Handle("/v1/receiver", &alerts.ReceiverHandler{
		Client:      client,
		DefaultRepo: *githubRepo,
		AutoClose:   *enableAutoClose,
	})
	http.ListenAndServe(":9393", nil)
}

func main() {
	flag.Parse()
	if *authtoken == "" || *githubOrg == "" || *githubRepo == "" {
		flag.Usage()
		os.Exit(1)
	}
	client := issues.NewClient(*githubOrg, *authtoken)
	serveListener(client)
}
