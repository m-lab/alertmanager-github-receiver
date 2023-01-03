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
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/m-lab/go/httpx"
	"github.com/m-lab/go/rtx"

	"github.com/m-lab/alertmanager-github-receiver/alerts"
	"github.com/m-lab/alertmanager-github-receiver/issues"
	"github.com/m-lab/alertmanager-github-receiver/issues/local"
	"github.com/m-lab/go/flagx"
	"github.com/m-lab/go/prometheusx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	authtoken       = flag.String("authtoken", "", "Oauth2 token for access to github API.")
	authtokenFile   = flagx.File{}
	githubOrg       = flag.String("org", "", "The github user or organization name where all repos are found.")
	githubRepo      = flag.String("repo", "", "The default repository for creating issues when alerts do not include a repo label.")
	githubBaseURL   = flag.String("enterprise.base-url", "", "The URL of your GitHub Enterprise with API suffix (for example '/api/v3/').")
	githubUploadURL = flag.String("enterprise.upload-url", "", "The upload URL needs to be set if it differs from the Github Enterprise base URL.")
	enableAutoClose = flag.Bool("enable-auto-close", false, "Once an alert stops firing, automatically close open issues.")
	labelOnResolved = flag.String("label-on-resolved", "", "Once an alert stops firing, apply this label.")
	enableInMemory  = flag.Bool("enable-inmemory", false, "Perform all operations in memory, without using github API.")
	receiverAddr    = flag.String("webhook.listen-address", ":9393", "Listen on address for new alertmanager webhook messages.")
	alertLabel      = flag.String("alertlabel", "alert:boom:", "The default label applied to all alerts. Also used to search the repo to discover exisitng alerts.")
	extraLabels     = flagx.StringArray{}
	titleTmplFile   = flagx.FileBytes(alerts.DefaultTitleTmpl)
	alertTmplFile   = flagx.FileBytes(alerts.DefaultAlertTmpl)
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

var (
	ctx, cancelCtx = context.WithCancel(context.Background())
	osExit         = os.Exit
)

const (
	usage = `
NAME
  github_receiver receives Alertmanager webhook notifications and creates
  corresponding issues on Github.

DESCRIPTION
  The github_receiver authenticates all actions using the given -authtoken
  or the value read from -authtokenFile. As well, the given -org and -repo
  names are used as the default destination for new issues.

EXAMPLE
  github_receiver -org <name> -repo <repo> -authtoken <token>
`
)

func init() {
	flag.Var(&extraLabels, "label", "Extra labels to add to issues at creation time.")
	flag.Var(&authtokenFile, "authtoken-file", "Oauth2 token file for access to github API. When provided it takes precedence over authtoken.")
	flag.Var(&titleTmplFile, "title-template-file", "File containing a template to generate issue titles.")
	flag.Var(&alertTmplFile, "alert-template-file", "File containing Markdown template to generate issue context.")
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), usage)
		flag.PrintDefaults()
	}
}

func mustServeWebhookReceiver(receiver *alerts.ReceiverHandler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", &issues.ListHandler{ListClient: receiver.Client})
	mux.Handle("/v1/receiver", promhttp.InstrumentHandlerDuration(receiverDuration, receiver))
	srv := &http.Server{
		Addr:    *receiverAddr,
		Handler: mux,
	}
	rtx.Must(httpx.ListenAndServeAsync(srv), "Failed to start webhook receiver server")
	return srv
}

func main() {
	flag.Parse()
	rtx.Must(flagx.ArgsFromEnv(flag.CommandLine), "Failed to read ArgsFromEnv")
	if (*authtoken == "" && len(authtokenFile.Bytes) == 0) || *githubOrg == "" || *githubRepo == "" {
		flag.Usage()
		osExit(1)
		return
	}

	var token string
	if len(authtokenFile.Bytes) != 0 {
		token = authtokenFile.Content()
	} else {
		token = *authtoken
	}

	var client alerts.ReceiverClient
	if *enableInMemory {
		client = local.NewClient()
	} else if *githubBaseURL == "" {
		client = issues.NewClient(*githubOrg, token, *alertLabel)
	} else {
		var err error
		client, err = issues.NewEnterpriseClient(*githubBaseURL, *githubUploadURL, *githubOrg, token, *alertLabel)
		if err != nil {
			fmt.Print(err)
			osExit(1)
			return
		}
	}

	promSrv := prometheusx.MustServeMetrics()
	defer promSrv.Close()

	receiver, err := alerts.NewReceiver(client, *githubRepo, *enableAutoClose, *labelOnResolved, extraLabels, string(titleTmplFile), string(alertTmplFile))
	if err != nil {
		fmt.Print(err)
		osExit(1)
		return
	}
	srv := mustServeWebhookReceiver(receiver)
	defer srv.Close()
	<-ctx.Done()
}
