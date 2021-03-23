# alertmanager-github-receiver
 [![Version](https://img.shields.io/github/tag/m-lab/alertmanager-github-receiver.svg)](https://github.com/m-lab/alertmanager-github-receiver/releases) [![Build Status](https://travis-ci.org/m-lab/alertmanager-github-receiver.svg?branch=master)](https://travis-ci.org/m-lab/alertmanager-github-receiver) [![Coverage Status](https://coveralls.io/repos/m-lab/alertmanager-github-receiver/badge.svg?branch=master)](https://coveralls.io/github/m-lab/alertmanager-github-receiver?branch=master) [![GoDoc](https://godoc.org/github.com/m-lab/alertmanager-github-receiver?status.svg)](https://godoc.org/github.com/m-lab/alertmanager-github-receiver) | [![Go Report Card](https://goreportcard.com/badge/github.com/m-lab/alertmanager-github-receiver)](https://goreportcard.com/report/github.com/m-lab/alertmanager-github-receiver)

Not all alerts are an emergency. But, we want to track every one
because alerts are always an actual problem. Either:

 * an actual problem in the monitored system
 * an actual problem in processes around the monitored system
 * an actual problem with the alert itself

The alertmanager github receiver creates GitHub issues using
[Alertmanager](https://github.com/prometheus/alertmanager) webhook
notifications.

# Build
```
make docker DOCKER_TAG=repo/imageName
```
This will build the binary and push it to repo/imageName.

# Setup

## Create GitHub access token

The github receiver uses user access tokens to create issues in an existing
repository.

Generate a new access token:

* Log into GitHub and visit https://github.com/settings/tokens
* Click the 'Generate new token' button
* Select the 'repo' scope and all subscopes of 'repo'

Because this access token has permission to create issues and operate on
repositories the access token user can access, protect the access token as
you would a password.

## Start GitHub Receiver

To start the github receiver locally:
```
docker run -it -p 9393:9393 measurementlab/alertmanager-github-receiver:latest
        -authtoken=$(GITHUB_AUTH_TOKEN) -org=<org> -repo=<repo>
```

Note: both the org and repo must already exist.

## Configure Alertmanager Webhook Plugin

The Prometheus Alertmanager supports third-party notification mechanisms
using the [Alertmanager Webhook API](https://prometheus.io/docs/alerting/configuration/#webhook_config).

Add a receiver definition to the alertmanager configuration.

```
- name: 'github-receiver-issues'
  webhook_configs:
  - url: 'http://localhost:9393/v1/receiver'
```

To publish a test notification by hand, try:

```
msg='{
  "version": "4",
  "groupKey": "fakegroupkey",
  "status": "firing",
  "receiver": "http://localhost:9393/v1/receiver",
  "groupLabels": {"alertname": "FoobarIsBroken"},
  "externalURL": "http://localhost:9093",
  "alerts": [
    {
      "labels": {"thing": "value"},
      "annotations": {"hint": "how to fix foobar"},
      "status": "firing",
      "startsAt": "2018-06-12T01:00:00Z",
      "endsAt": "2018-06-14T01:00:00Z"
    }
  ]
}'
curl -XPOST --data-binary "${msg}" http://localhost:9393/v1/receiver
```

# Configuration

## Alertmanager & Github Receiver

The Alertmanager configuration controls what labels are present on alerts
delivered to the github-receiver. The github-receiver configuration must be
compatible with these settings to work effectively.

For example, it is common for the Alertmanager to use alert routes that
`group_by: ['alertname']`. See: https://github.com/prometheus/alertmanager#example

And, the github-receiver's default "title template" is
`{{ .Data.GroupLabels.alertname }}`, which depends on an alertname group
label.

If an alert does not include this label, the template will evaluate to `<no value>`.
To prevent this, ensure that the github-receiver title template uses labels available
in an Alertmanager [Message](https://godoc.org/github.com/prometheus/alertmanager/notify/webhook#Message).

## Auto close

If `-enable-auto-close` is specified, the program will close each issue as its
corresponding alert is resolved. It searches for matching issues by filtering
open issues on the value of `-alertlabel` and then matching issue titles. The
issue title template can be overridden using `-title-template-file` for greater
(or lesser) specificity. The default template is
`{{ .Data.GroupLabels.alertname }}`, which sets the issue title to the alert
name. The template is passed a
[Message](https://godoc.org/github.com/prometheus/alertmanager/notify/webhook#Message)
as its argument.

## Repository

If the alert includes a `repo` label, issues will be created in that repository,
under the GitHub organization specified by `-org`. If no `repo` label is
present, issues will be created in the repository specified by the `-repo`
option.
