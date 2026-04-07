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
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/m-lab/go/httpx"
	"github.com/m-lab/go/rtx"
	"github.com/urfave/cli/v3"

	"github.com/m-lab/alertmanager-github-receiver/alerts"
	"github.com/m-lab/alertmanager-github-receiver/issues"
	"github.com/m-lab/alertmanager-github-receiver/issues/local"
	"github.com/m-lab/go/prometheusx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

func mustServeWebhookReceiver(addr string, receiver *alerts.ReceiverHandler) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/", &issues.ListHandler{ListClient: receiver.Client})
	mux.Handle("/v1/receiver", promhttp.InstrumentHandlerDuration(receiverDuration, receiver))
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	rtx.Must(httpx.ListenAndServeAsync(srv), "Failed to start webhook receiver server")
	slog.Info("HTTP receiver for Alertmanager started", "address", addr)
	return srv
}

func run(ctx context.Context, cmd *cli.Command) error {
	// Configure structured JSON logging.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	authtoken := cmd.String("authtoken")
	authtokenFile := cmd.String("authtoken-file")
	org := cmd.String("org")
	repo := cmd.String("repo")

	if (authtoken == "" && authtokenFile == "") || org == "" || repo == "" {
		_ = cli.ShowAppHelp(cmd)
		return cli.Exit("required flags missing: --org, --repo, and one of --authtoken / --authtoken-file", 1)
	}

	var token string
	if authtokenFile != "" {
		b, err := os.ReadFile(authtokenFile)
		if err != nil {
			return err
		}
		token = strings.TrimSpace(string(b))
	} else {
		token = authtoken
	}

	alertLabel := cmd.String("alertlabel")
	baseURL := cmd.String("enterprise.base-url")
	uploadURL := cmd.String("enterprise.upload-url")

	var client alerts.ReceiverClient
	if cmd.Bool("enable-inmemory") {
		client = local.NewClient()
	} else if baseURL == "" {
		client = issues.NewClient(org, token, alertLabel)
	} else {
		var err error
		client, err = issues.NewEnterpriseClient(baseURL, uploadURL, org, token, alertLabel)
		if err != nil {
			return err
		}
	}

	// Set prometheus listen address before starting metrics server.
	*prometheusx.ListenAddress = cmd.String("prometheus.listen-address")
	promSrv := prometheusx.MustServeMetrics()
	defer promSrv.Close()

	titleTmpl := cmd.String("title-template")
	alertTmpl := cmd.String("alert-template")
	receiver, err := alerts.NewReceiver(
		client, repo,
		cmd.Bool("enable-auto-close"),
		cmd.String("label-on-resolved"),
		cmd.StringSlice("label"),
		titleTmpl, alertTmpl,
	)
	if err != nil {
		return err
	}

	srv := mustServeWebhookReceiver(cmd.String("webhook.listen-address"), receiver)
	defer srv.Close()

	<-ctx.Done()
	return nil
}

func newApp() *cli.Command {
	return &cli.Command{
		Name:  "github_receiver",
		Usage: "Receive Alertmanager webhook notifications and create/close GitHub issues.",
		Description: `github_receiver authenticates all actions using the given --authtoken
or the file path in --authtoken-file. The --org and --repo flags set the
default destination for new issues.

Example:
  github_receiver --org <name> --repo <repo> --authtoken <token>`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "authtoken",
				Usage:   "Oauth2 token for access to github API.",
				Sources: cli.EnvVars("AUTHTOKEN"),
			},
			&cli.StringFlag{
				Name:    "authtoken-file",
				Usage:   "Path to file containing Oauth2 token. Takes precedence over --authtoken.",
				Sources: cli.EnvVars("AUTHTOKEN_FILE"),
			},
			&cli.StringFlag{
				Name:    "org",
				Usage:   "The github user or organization name where all repos are found.",
				Sources: cli.EnvVars("ORG"),
			},
			&cli.StringFlag{
				Name:    "repo",
				Usage:   "The default repository for creating issues when alerts do not include a repo label.",
				Sources: cli.EnvVars("REPO"),
			},
			&cli.StringFlag{
				Name:    "enterprise.base-url",
				Usage:   "The URL of your GitHub Enterprise with API suffix (e.g. '/api/v3/').",
				Sources: cli.EnvVars("ENTERPRISE_BASE_URL"),
			},
			&cli.StringFlag{
				Name:    "enterprise.upload-url",
				Usage:   "The upload URL if it differs from the GitHub Enterprise base URL.",
				Sources: cli.EnvVars("ENTERPRISE_UPLOAD_URL"),
			},
			&cli.BoolFlag{
				Name:    "enable-auto-close",
				Usage:   "Once an alert stops firing, automatically close open issues.",
				Sources: cli.EnvVars("ENABLE_AUTO_CLOSE"),
			},
			&cli.StringFlag{
				Name:    "label-on-resolved",
				Usage:   "Once an alert stops firing, apply this label.",
				Sources: cli.EnvVars("LABEL_ON_RESOLVED"),
			},
			&cli.BoolFlag{
				Name:    "enable-inmemory",
				Usage:   "Perform all operations in memory, without using github API.",
				Sources: cli.EnvVars("ENABLE_INMEMORY"),
			},
			&cli.StringFlag{
				Name:    "webhook.listen-address",
				Value:   ":9393",
				Usage:   "Listen address for new Alertmanager webhook messages.",
				Sources: cli.EnvVars("WEBHOOK_LISTEN_ADDRESS"),
			},
			&cli.StringFlag{
				Name:    "prometheus.listen-address",
				Value:   ":9090",
				Usage:   "Listen address for the Prometheus metrics endpoint.",
				Sources: cli.EnvVars("PROMETHEUS_LISTEN_ADDRESS"),
			},
			&cli.StringFlag{
				Name:    "alertlabel",
				Value:   "alert:boom:",
				Usage:   "Default label applied to all alerts. Used to search the repo for existing alerts.",
				Sources: cli.EnvVars("ALERTLABEL"),
			},
			&cli.StringSliceFlag{
				Name:    "label",
				Usage:   "Extra labels to add to issues at creation time (repeatable).",
				Sources: cli.EnvVars("LABEL"),
			},
			&cli.StringFlag{
				Name:    "title-template",
				Value:   alerts.DefaultTitleTmpl,
				Usage:   "Go template string for issue titles.",
				Sources: cli.EnvVars("TITLE_TEMPLATE"),
			},
			&cli.StringFlag{
				Name:    "alert-template",
				Value:   alerts.DefaultAlertTmpl,
				Usage:   "Go template string (Markdown) for issue bodies.",
				Sources: cli.EnvVars("ALERT_TEMPLATE"),
			},
		},
		Action: run,
	}
}

func main() {
	app := newApp()
	if err := app.Run(context.Background(), os.Args); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}
