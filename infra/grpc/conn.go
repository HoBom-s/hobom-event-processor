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

// NewConn creates a gRPC client connection with an API key interceptor.
// If HOBOM_GRPC_TLS_CERT is set, TLS is enabled; otherwise insecure.
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
