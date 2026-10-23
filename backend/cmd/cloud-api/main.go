// Command cloud-api serves the dashboard API. It runs in Lambda through Lambda Web Adapter,
// behind API Gateway's Cognito JWT authorizer, and runs unchanged locally for development.
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
	// TODO(phase 5): read pre-computed monthly aggregates and transactions from DynamoDB.
	mux.HandleFunc("GET /v1/summary", httpx.NotImplemented)
	mux.HandleFunc("GET /v1/transactions", httpx.NotImplemented)

	if err := httpx.Run(httpx.Addr(), mux); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
