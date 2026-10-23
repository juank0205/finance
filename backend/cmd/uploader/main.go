// Command uploader publishes unsynced transactions to AWS IoT Core over MQTT with mTLS.
// It is the only edge service allowed to reach the internet.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("uploader started")
	// TODO(phase 4): connect to IOT_ENDPOINT with device cert, publish transactions where synced_at IS NULL.
	<-ctx.Done()
	slog.Info("uploader stopped")
}
