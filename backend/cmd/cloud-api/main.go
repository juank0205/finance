// Command cloud-api serves the dashboard API. It runs in Lambda through Lambda Web Adapter and runs
// unchanged locally for development.
//
// Environment:
//
//	FINANCE_TABLE              DynamoDB table; unset keeps data in memory until the process stops
//	FINANCE_DYNAMODB_ENDPOINT  DynamoDB Local, e.g. http://localhost:8000; the table is created if missing
//	FINANCE_TZ                 time zone that decides which day and month a moment falls in (default Europe/Paris)
//	FINANCE_CURRENCY           currency of the summary (default EUR)
//	COGNITO_ISSUER             user pool, https://cognito-idp.<region>.amazonaws.com/<pool id>; with
//	COGNITO_CLIENT_ID          the dashboard's app client, every /v1 request needs its access token.
//	                           Both are required in Lambda; locally, unset means no sign-in.
//	ADDR                       listen address (default :8080)
package main

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"
	_ "time/tzdata" // the distroless image has no zoneinfo of its own

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"finance/internal/auth"
	"finance/internal/fx"
	"finance/internal/httpx"
	"finance/internal/ledger"
	"finance/internal/ledger/dynamostore"
)

func main() {
	if err := run(); err != nil {
		slog.Error("cloud-api stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	loc, err := time.LoadLocation(cmp.Or(os.Getenv("FINANCE_TZ"), "Europe/Paris"))
	if err != nil {
		return err
	}
	currency := cmp.Or(os.Getenv("FINANCE_CURRENCY"), "EUR")

	store, err := openStore(context.Background(), os.Getenv("FINANCE_TABLE"), os.Getenv("FINANCE_DYNAMODB_ENDPOINT"))
	if err != nil {
		return err
	}
	api, err := protect(ledger.NewHandler(ledger.NewService(store, loc, currency, fx.New())))
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", httpx.Health)
	mux.Handle("/v1/", api)
	return httpx.Run(httpx.Addr(), mux)
}

// protect puts the Cognito token check in front of h. In Lambda it refuses to start without one, so
// a missing setting can never publish the ledger without sign-in.
func protect(h http.Handler) (http.Handler, error) {
	issuer, clientID := os.Getenv("COGNITO_ISSUER"), os.Getenv("COGNITO_CLIENT_ID")
	if issuer != "" && clientID != "" {
		return auth.NewVerifier(issuer, clientID).Middleware(h), nil
	}
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		return nil, errors.New("COGNITO_ISSUER and COGNITO_CLIENT_ID must be set in Lambda")
	}
	slog.Warn("COGNITO_ISSUER/COGNITO_CLIENT_ID not set: the API accepts requests without sign-in")
	return h, nil
}

func openStore(ctx context.Context, table, endpoint string) (ledger.Store, error) {
	if table == "" {
		slog.Warn("FINANCE_TABLE is not set: data is kept in memory and lost when the process stops")
		return ledger.NewMemoryStore(), nil
	}
	db, err := dynamostore.Connect(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	if endpoint != "" {
		if err := ensureLocalTable(ctx, db, table); err != nil {
			return nil, err
		}
		slog.Info("using DynamoDB Local", "endpoint", endpoint, "table", table)
	}
	return dynamostore.New(db, table), nil
}

// ensureLocalTable creates the table in DynamoDB Local on first run. In AWS, Terraform owns it.
func ensureLocalTable(ctx context.Context, db *dynamodb.Client, table string) error {
	_, err := db.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &table})
	if notFound := new(types.ResourceNotFoundException); errors.As(err, &notFound) {
		return dynamostore.CreateTable(ctx, db, table)
	}
	return err
}
