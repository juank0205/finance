package ledger_test

import (
	"testing"

	"finance/internal/ledger"
	"finance/internal/ledger/ledgertest"
)

func TestMemoryStore(t *testing.T) {
	ledgertest.Run(t, func(*testing.T) ledger.Store { return ledger.NewMemoryStore() })
}
