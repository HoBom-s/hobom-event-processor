PROTO_DIR := ./infra/proto/v1
PB_BASE := ./infra/grpc/v1

PROTO_FILES := $(shell find $(PROTO_DIR) -name "*.proto")

.PHONY: proto run clean

proto:
	@echo "📦 Compiling proto files..."
	@mkdir -p $(PB_BASE)
	@for file in $(PROTO_FILES); do \
		name=$$(basename $$file .proto); \
		out_dir=$(PB_BASE)/$$name; \
		mkdir -p $$out_dir; \
		protoc \
			--proto_path=$(PROTO_DIR) \
			--go_out=$$out_dir \
			--go-grpc_out=$$out_dir \
			--go_opt=paths=source_relative \
			--go-grpc_opt=paths=source_relative \
			$$file; \
	done
	@echo "✅ Done!"

run: proto
	go run ./cmd/main.go

clean:
	rm -rf $(PB_BASE)
