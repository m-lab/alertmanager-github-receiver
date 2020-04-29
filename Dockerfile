FROM golang:1.14 as builder

WORKDIR /src
ADD go.mod go.sum ./
RUN go mod download
ADD . ./

# TODO(soltesz): Use vgo for dependencies.
ENV CGO_ENABLED 0
RUN go build \
       -v \
      -ldflags "-X github.com/m-lab/go/prometheusx.GitShortCommit=$(git log -1 --format=%h)" \
       ./cmd/github_receiver

FROM alpine
RUN apk add --no-cache ca-certificates && \
    update-ca-certificates
WORKDIR /
COPY --from=builder /src/github_receiver ./
ENTRYPOINT ["/github_receiver"]
