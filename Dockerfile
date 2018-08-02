FROM golang:1.10

WORKDIR /go/src/github.com/m-lab/alertmanager-github-receiver
ADD . ./

# TODO(soltesz): Use vgo for dependencies.
RUN go get -v ./...
RUN CGO_ENABLED=0 go get -v github.com/m-lab/alertmanager-github-receiver/cmd/github_receiver


FROM alpine
RUN apk add --no-cache ca-certificates && \
    update-ca-certificates
WORKDIR /
COPY --from=0 /go/bin/github_receiver ./
ENTRYPOINT ["/github_receiver"]