package dynamostore

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"finance/internal/ledger"
	"finance/internal/ledger/ledgertest"
)

// Runs against DynamoDB Local only:
//
//	docker run --rm -d -p 8000:8000 amazon/dynamodb-local
//	$env:FINANCE_DYNAMODB_ENDPOINT = "http://localhost:8000"; go test ./internal/ledger/dynamostore
func TestStore(t *testing.T) {
	endpoint := os.Getenv("FINANCE_DYNAMODB_ENDPOINT")
	if endpoint == "" {
		t.Skip("FINANCE_DYNAMODB_ENDPOINT is not set (e.g. http://localhost:8000 for DynamoDB Local)")
	}
	ctx := context.Background()
	db, err := Connect(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	ledgertest.Run(t, func(t *testing.T) ledger.Store {
		n++
		table := fmt.Sprintf("test-%d-%d", time.Now().UnixNano(), n)
		if err := CreateTable(ctx, db, table); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = db.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: &table})
		})
		return New(db, table)
	})
}
