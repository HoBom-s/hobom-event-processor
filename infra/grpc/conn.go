// Package grpc provides a shared gRPC client connection factory for
// all outbound gRPC calls in hobom-event-processor.
//
// Every connection is configured with:
//   - API key interceptor: injects x-api-key metadata into every unary RPC,
//     matching the server-side ApiKeyInterceptor on backend services.
//   - Transport credentials: TLS if HOBOM_GRPC_TLS_CERT is set, otherwise
//     insecure plaintext (suitable for Docker internal network).
package grpc

import (
	"context"
	"log/slog"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// NewConn creates a gRPC client connection with API key auth and optional TLS.
//
// The apiKey is attached to every outgoing RPC as an "x-api-key" metadata
// header via a unary interceptor. This authenticates against the backend
// services' gRPC interceptors (e.g. ApiKeyInterceptor in hobom-space-backend).
func NewConn(addr, apiKey string) (*grpc.ClientConn, error) {
	interceptor := func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-api-key", apiKey)
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	return grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(transportCredentials()),
		grpc.WithUnaryInterceptor(interceptor),
	)
}

// transportCredentials returns TLS credentials if HOBOM_GRPC_TLS_CERT is
// set to a valid PEM file path, otherwise returns insecure credentials.
// Falls back to insecure on any TLS loading error (with a warning log).
func transportCredentials() credentials.TransportCredentials {
	certFile := os.Getenv("HOBOM_GRPC_TLS_CERT")
	if certFile == "" {
		return insecure.NewCredentials()
	}
	creds, err := credentials.NewClientTLSFromFile(certFile, "")
	if err != nil {
		slog.Error("failed to load TLS cert, falling back to insecure", "cert", certFile, "err", err)
		return insecure.NewCredentials()
	}
	slog.Info("gRPC TLS enabled", "cert", certFile)
	return creds
}
