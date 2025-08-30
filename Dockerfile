FROM golang:1.24-alpine AS builder
RUN apk add --no-cache make git tzdata ca-certificates curl bash

ENV BUF_VERSION=1.43.0
RUN curl -sSL "https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-Linux-x86_64" \
      -o /usr/local/bin/buf && \
    chmod +x /usr/local/bin/buf && buf --version

ENV PATH="/go/bin:${PATH}"
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN make proto
RUN make build

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/bin/hobom-event-processor /app/app
EXPOSE 8082
ENTRYPOINT ["/app/app"]
