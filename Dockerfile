FROM golang:1.12 as builder

WORKDIR /go/src/github.com/m-lab/alertmanager-github-receiver
ADD . ./

# TODO(soltesz): Use vgo for dependencies.
ENV CGO_ENABLED 0
RUN go get \
       -v \
      -ldflags "-X github.com/m-lab/go/prometheusx.GitShortCommit=$(git log -1 --format=%h)" \
       github.com/m-lab/alertmanager-github-receiver/cmd/github_receiver

FROM alpine
RUN apk add --no-cache ca-certificates && \
    update-ca-certificates
WORKDIR /
COPY --from=builder /go/bin/github_receiver ./
ENTRYPOINT ["/github_receiver"]
