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

// Kind says what a money movement is. An investment leaves cash but is not spending; a transfer moves
// money between two of the person's own accounts and is neither spending nor income.
type Kind string

const (
	KindExpense    Kind = "expense"
	KindIncome     Kind = "income"
	KindInvestment Kind = "investment"
	KindTransfer   Kind = "transfer"
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
	// Refund marks an expense that brought money back (a refund, a friend paying their share): its
	// amount is positive and it lowers the category's spending.
	Refund bool `json:"refund,omitempty"`

	// Account is the account the money left or entered; empty means none, so no balance changes.
	Account string `json:"account,omitempty"`
	// AccountMinor is AmountMinor in the account's currency, converted at the day's rate.
	AccountMinor int64 `json:"account_minor,omitempty"`
	// BaseMinor is AmountMinor in the summary currency (EUR), converted at the day's rate.
	BaseMinor int64 `json:"base_minor,omitempty"`
	// ToAccount and ToMinor are set on transfers only: where the money arrived and how much, in that
	// account's currency. A transfer's AmountMinor and AccountMinor are what left Account.
	ToAccount string `json:"to_account,omitempty"`
	ToMinor   int64  `json:"to_minor,omitempty"`

	Source   Source `json:"source"`
	DedupKey string `json:"dedup_key"`
}
