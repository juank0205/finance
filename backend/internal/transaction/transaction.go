// Package transaction defines the normalized transaction shared by edge and cloud.
package transaction

import "time"

// Source records where a transaction came from.
type Source string

const (
	SourceWallet    Source = "google_wallet"
	SourceManual    Source = "manual"
	SourceStatement Source = "statement"
)

// Transaction is a cleaned, deduplicated money movement.
// Amounts are integer minor units (cents) to avoid float rounding errors.
type Transaction struct {
	ID          string    `json:"id"`
	OccurredAt  time.Time `json:"occurred_at"`
	AmountMinor int64     `json:"amount_minor"` // negative = money out
	Currency    string    `json:"currency"`     // ISO 4217, e.g. "COP"
	Merchant    string    `json:"merchant"`
	Category    string    `json:"category,omitempty"`
	CardLast4   string    `json:"card_last4,omitempty"` // never store full card numbers
	Source      Source    `json:"source"`
	DedupKey    string    `json:"dedup_key"`
}
