FROM golang:1.22-alpine AS builder
RUN apk add --no-cache make git tzdata ca-certificates

WORKDIR /app
COPY . .

# Makefile
RUN make proto
RUN make build

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/bin/hobom-event-processor /app/app
ENTRYPOINT ["/app/app"]
