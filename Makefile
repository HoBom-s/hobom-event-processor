PROTO_DIR := hobom-buf-proto
PB_DIR := ./infra/grpc

.PHONY: proto run clean sync-submodule

sync-submodule:
	@git submodule update --remote --merge $(PROTO_DIR)
	@echo "🔄 Submodule updated: $(PROTO_DIR)"

proto:
	@command -v buf >/dev/null 2>&1 || { echo >&2 "❌ buf CLI not found. Please install: brew install bufbuild/buf/buf"; exit 1; }
	@echo "📦 Generating proto files with buf..."
	cd $(PROTO_DIR) && buf generate
	@echo "✅ Done!"

run: proto
	go run ./cmd/main.go

clean:
	rm -rf $(PB_DIR)
