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
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/m-lab/alertmanager-github-receiver/alerts"
	"github.com/m-lab/alertmanager-github-receiver/issues"
	"github.com/m-lab/alertmanager-github-receiver/issues/local"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	flag "github.com/spf13/pflag"
)

var (
	authtoken       = flag.String("authtoken", "", "Oauth2 token for access to github API.")
	authtokenFile   = flag.String("authtokenFile", "", "Oauth2 token file for access to github API. When provided it takes precedence over authtoken.")
	githubOrg       = flag.String("org", "", "The github user or organization name where all repos are found.")
	githubRepo      = flag.String("repo", "", "The default repository for creating issues when alerts do not include a repo label.")
	enableAutoClose = flag.Bool("enable-auto-close", false, "Once an alert stops firing, automatically close open issues.")
	enableInMemory  = flag.Bool("enable-inmemory", false, "Perform all operations in memory, without using github API.")
	receiverPort    = flag.String("port", "9393", "The port for accepting alertmanager webhook messages.")
	alertLabel      = flag.String("alertlabel", "alert:boom:", "The default label applied to all alerts. Also used to search the repo to discover exisitng alerts.")
	extraLabels     = flag.StringArray("label", nil, "Extra labels to add to issues at creation time.")
)

// Metrics.
var (
	receiverDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "github_receiver_duration_seconds",
			Help: "A histogram of request latencies to the receiver handler.",
		},
		[]string{"code"},
	)
)

const (
	usage = `
Usage of %s:

Github receiver requires a github --authtoken or --authtokenFile, target github --owner and
--repo names.

`
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}
}

func serveReceiverHandler(client alerts.ReceiverClient) {
	receiverHandler := &alerts.ReceiverHandler{
		Client:      client,
		DefaultRepo: *githubRepo,
		AutoClose:   *enableAutoClose,
		ExtraLabels: *extraLabels,
	}
	http.Handle("/", &issues.ListHandler{ListClient: client})
	http.Handle("/v1/receiver", promhttp.InstrumentHandlerDuration(receiverDuration, receiverHandler))
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":"+*receiverPort, nil))
}

func main() {
	flag.Parse()
	if (*authtoken == "" && *authtokenFile == "") || *githubOrg == "" || *githubRepo == "" {
		flag.Usage()
		os.Exit(1)
	}

	var token string
	if *authtokenFile != "" {
		d, err := ioutil.ReadFile(*authtokenFile)
		if err != nil {
			log.Fatal(err)
		}
		token = string(d)
	} else {
		token = *authtoken
	}

	var client alerts.ReceiverClient
	if *enableInMemory {
		client = local.NewClient()
	} else {
		client = issues.NewClient(*githubOrg, token, *alertLabel)
	}
	serveReceiverHandler(client)
}
