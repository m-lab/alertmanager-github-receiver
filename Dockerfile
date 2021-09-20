FROM golang:1.17-alpine as builder

WORKDIR /go/src/github.com/m-lab/alertmanager-github-receiver
ADD go.mod go.sum ./
RUN go mod download
ADD . ./

RUN CGO_ENABLED=0 GOARCH=amd64 GOOS=linux \
    go build \
    -v \
    -ldflags "-X github.com/m-lab/go/prometheusx.GitShortCommit=$(git log -1 --format=%h) -w -s" \
    ./cmd/github_receiver

# See also: https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md
FROM gcr.io/distroless/static

COPY --from=builder /go/src/github.com/m-lab/alertmanager-github-receiver/github_receiver ./
ENTRYPOINT ["/github_receiver"]
