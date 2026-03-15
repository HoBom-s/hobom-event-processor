BSR_MODULE := buf.build/hobom/hobom-buf-proto
PB_DIR := ./infra/grpc

.PHONY: proto run clean

proto:
	@command -v buf >/dev/null 2>&1 || { echo >&2 "buf CLI not found. Please install: brew install bufbuild/buf/buf"; exit 1; }
	rm -rf $(PB_DIR)
	buf generate $(BSR_MODULE)

run:
	go run ./cmd/main.go

clean:
	rm -rf $(PB_DIR)
