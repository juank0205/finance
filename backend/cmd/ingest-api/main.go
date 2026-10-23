// Command ingest-api receives raw Google Wallet notification events from the Android app.
// Caddy terminates mTLS in front of it; only requests with a valid client cert reach here.
package main

import (
	"log/slog"
	"net/http"
	"os"

	"finance/internal/httpx"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", httpx.Health)
	// TODO(phase 2): verify HMAC signature, cap body size, insert into raw_events.
	mux.HandleFunc("POST /v1/events", httpx.NotImplemented)

	if err := httpx.Run(httpx.Addr(), mux); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
