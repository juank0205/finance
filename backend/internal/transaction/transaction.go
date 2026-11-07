// Package transaction defines the normalized transaction shared by edge and cloud.
package transaction

import "time"

// Source records where a transaction came from.
type Source string

const (
	SourceWallet    Source = "google_wallet"
	SourceManual    Source = "manual"
	SourceStatement Source = "statement"
	SourceRecurring Source = "recurring" // created by a recurring rule; dedup key "recurring:<rule>:<YYYY-MM-DD>"
)

// Kind says what a money movement is. An investment leaves cash but is not spending.
type Kind string

const (
	KindExpense    Kind = "expense"
	KindIncome     Kind = "income"
	KindInvestment Kind = "investment"
)

// Transaction is a cleaned, deduplicated money movement.
// Amounts are integer minor units (cents) to avoid float rounding errors.
type Transaction struct {
	ID          string    `json:"id"`
	Kind        Kind      `json:"kind"`
	OccurredAt  time.Time `json:"occurred_at"`
	AmountMinor int64     `json:"amount_minor"`         // negative = money out of cash
	Currency    string    `json:"currency"`             // ISO 4217, e.g. "EUR"
	Merchant    string    `json:"merchant"`             // who was paid, or where the money came from
	Category    string    `json:"category,omitempty"`   // category id
	Note        string    `json:"note,omitempty"`       // free text; manual entries (cloud) only
	CardLast4   string    `json:"card_last4,omitempty"` // never store full card numbers
	Source      Source    `json:"source"`
	DedupKey    string    `json:"dedup_key"`
}
