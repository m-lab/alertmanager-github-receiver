FROM golang:1.10

WORKDIR /go/src/github.com/m-lab/alertmanager-github-receiver
ADD . ./

# TODO(soltesz): Use vgo for dependancies.
RUN go get -v ./...
RUN CGO_ENABLED=0 go build -o alertmanager-github-receiver cmd/github_receiver/main.go


FROM alpine
RUN apk add --no-cache ca-certificates && \
    update-ca-certificates
WORKDIR /root/
COPY --from=0 /go/src/github.com/m-lab/alertmanager-github-receiver/alertmanager-github-receiver ./
ENTRYPOINT ["./alertmanager-github-receiver"]