FROM golang:1.12 AS builder
WORKDIR /example
COPY . ./

RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /example/example .
ENTRYPOINT ["/example"]
