// Command admin-api backs the edge admin UI: login, manual entries, statement uploads,
// review queue, categories and parser rules. Served by Caddy under /api.
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
	// TODO(phase 3): session login with password + TOTP/passkey, audit log on every write.
	mux.HandleFunc("POST /api/session", httpx.NotImplemented)
	mux.HandleFunc("GET /api/transactions", httpx.NotImplemented)
	mux.HandleFunc("POST /api/transactions", httpx.NotImplemented)
	mux.HandleFunc("POST /api/uploads", httpx.NotImplemented)
	mux.HandleFunc("GET /api/review", httpx.NotImplemented)

	if err := httpx.Run(httpx.Addr(), mux); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
