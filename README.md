# alertmanager-github-receiver

Not all alerts are an emergency. But, we want to track every one
because alerts are always an actual problem. Either:

 * an actual problem in the monitored system
 * an actual problem in processes around the monitored system
 * an actual problem with the alert itself

The alertmanager github receiver creates GitHub issues or posts new comments to existing issues.
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
docker run -it measurementlab/alertmanager-github-receiver:latest
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
   "commonAnnotations": {"issue": "53"}, // When this is present it will post a new comment to issue 53 if it exists.
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

