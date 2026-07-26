package main

import (
	"log/slog"
	"os"

	grpcConn "github.com/HoBom-s/hobom-event-processor/infra/grpc"
	"google.golang.org/grpc"
)

// mustEnv reads an environment variable and terminates the process if it is
// not set. Used for variables that are required for the service to function
// (e.g. HOBOM_GRPC_ADDR, HOBOM_KAFKA_BROKER).
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required env var is not set", "key", key)
		os.Exit(1)
	}
	return v
}

// envOrDefault reads an environment variable, returning fallback if the
// variable is unset or empty. Used for optional configuration with sensible
// defaults (e.g. HOBOM_HTTP_ADDR defaults to ":8082").
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// mustGRPCConn establishes a gRPC client connection and terminates the
// process on failure. The connection includes an API key interceptor
// (x-api-key header) and optional TLS (via HOBOM_GRPC_TLS_CERT env var).
func mustGRPCConn(addr, apiKey string) *grpc.ClientConn {
	conn, err := grpcConn.NewConn(addr, apiKey)
	if err != nil {
		slog.Error("failed to connect to gRPC", "addr", addr, "err", err)
		os.Exit(1)
	}
	return conn
}

// optionalGRPCConn establishes a gRPC client connection only if the address
// env var is set. Returns nil when the env var is absent, allowing callers
// to skip features that depend on the connection (e.g. space poller, angel poller).
// Falls back to fallbackApiKey when the dedicated API key env var is not set.
func optionalGRPCConn(addrEnv, apiKeyEnv, fallbackApiKey string) *grpc.ClientConn {
	addr := os.Getenv(addrEnv)
	if addr == "" {
		return nil
	}
	apiKey := envOrDefault(apiKeyEnv, fallbackApiKey)
	conn, err := grpcConn.NewConn(addr, apiKey)
	if err != nil {
		slog.Error("failed to connect to gRPC", "addr", addr, "err", err)
		os.Exit(1)
	}
	slog.Info("gRPC connected", "addr", addr)
	return conn
}
