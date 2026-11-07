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
	// Contract: frontend/cloud-dashboard/src/api/types.ts.
	mux.HandleFunc("GET /v1/summary", httpx.NotImplemented)
	mux.HandleFunc("GET /v1/transactions", httpx.NotImplemented)
	// Manual entries: the server sets id, source=manual and the sign from kind, and rejects future
	// dates, unknown categories and edits to imported transactions.
	mux.HandleFunc("POST /v1/transactions", httpx.NotImplemented)
	mux.HandleFunc("PUT /v1/transactions/{id}", httpx.NotImplemented)
	mux.HandleFunc("DELETE /v1/transactions/{id}", httpx.NotImplemented)
	// Recurring rules. A daily EventBridge Scheduler run (free tier) creates due occurrences as
	// transactions; the dedup key makes reruns harmless. Creating a rule backfills past dates;
	// editing one affects only occurrences not created yet; deleting one keeps created entries.
	// Deleting an occurrence records its date as skipped so it is never re-created.
	mux.HandleFunc("GET /v1/recurring", httpx.NotImplemented)
	mux.HandleFunc("POST /v1/recurring", httpx.NotImplemented)
	mux.HandleFunc("PUT /v1/recurring/{id}", httpx.NotImplemented)
	mux.HandleFunc("DELETE /v1/recurring/{id}", httpx.NotImplemented)
	mux.HandleFunc("GET /v1/net-worth", httpx.NotImplemented)
	mux.HandleFunc("GET /v1/categories", httpx.NotImplemented)
	mux.HandleFunc("PUT /v1/categories", httpx.NotImplemented)

	if err := httpx.Run(httpx.Addr(), mux); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
