// Command cloud-ingest consumes the SQS queue fed by the IoT Core rule. Lambda Web Adapter
// delivers each SQS batch as a POST to /events (AWS_LWA_PASS_THROUGH_PATH).
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
	// TODO(phase 4): validate each record, upsert into DynamoDB by dedup_key, update monthly
	// aggregates, archive raw JSON to S3. Non-2xx makes SQS retry, then dead-letter.
	mux.HandleFunc("POST /events", httpx.NotImplemented)

	if err := httpx.Run(httpx.Addr(), mux); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
