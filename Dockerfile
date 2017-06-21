FROM golang:1.8

ADD . /go/src/github.com/m-lab/alertmanager-github-receiver

# TODO(soltesz): vendor dependencies.
RUN go get -v github.com/m-lab/alertmanager-github-receiver/cmd/github_receiver

# RUN go install -v
ENTRYPOINT ["/go/bin/github_receiver"]
