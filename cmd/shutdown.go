package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// gracefulShutdown blocks until a SIGTERM or SIGINT is received, then
// orchestrates an ordered shutdown sequence:
//
//  1. Cancel the root context → all pollers observe ctx.Done() and exit
//     their current tick loop after the in-flight Poll() completes.
//  2. Wait up to 10s for all poller goroutines to finish (wg.Wait).
//     If they don't finish in time, log a warning and proceed anyway.
//  3. Shut down the HTTP server with a 5s deadline, allowing in-flight
//     DLQ/health requests to complete.
//  4. Log completion and return, letting deferred cleanup (conn.Close, etc.)
//     run in main().
func gracefulShutdown(cancel context.CancelFunc, wg *sync.WaitGroup, server *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit
	slog.Info("shutdown signal received")

	// Step 1: Signal all pollers to stop.
	cancel()

	// Step 2: Wait for pollers to drain in-flight work.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("all pollers stopped")
	case <-time.After(10 * time.Second):
		slog.Warn("poller shutdown timed out after 10s, forcing exit")
	}

	// Step 3: Shut down HTTP server.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown failed", "err", err)
	}

	slog.Info("shutdown complete")
}
