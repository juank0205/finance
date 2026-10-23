// Command parser turns raw_events into clean transactions. It runs on a Docker network
// with no internet access, so a malicious upload cannot exfiltrate data.
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

	slog.Info("parser started")
	// TODO(phase 2): poll pending raw_events, parse Wallet text, dedup, categorize, write transactions.
	<-ctx.Done()
	slog.Info("parser stopped")
}
