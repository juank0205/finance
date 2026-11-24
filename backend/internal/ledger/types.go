// Package ledger is cloud-api's domain: manual entries, recurring rules, categories and the monthly
// summary. The JSON shapes are the contract in frontend/cloud-dashboard/src/api/types.ts.
package ledger

import (
	"context"
	"errors"
	"time"

	"finance/internal/transaction"
)

// Category is a user-configurable category. Each kind has its own list and its own catch-all.
type Category struct {
	ID    string           `json:"id"`
	Kind  transaction.Kind `json:"kind"`
	Name  string           `json:"name"`
	Color int              `json:"color"` // categorical palette slot, 1-8, unique within its kind
}

// TransactionInput is the body of POST /v1/transactions and PUT /v1/transactions/{id}.
type TransactionInput struct {
	Kind        transaction.Kind `json:"kind"`
	AmountMinor int64            `json:"amount_minor"` // always positive; the kind sets the sign
	Currency    string           `json:"currency"`
	OccurredAt  string           `json:"occurred_at"` // RFC 3339, never in the future
	Merchant    string           `json:"merchant"`
	Category    string           `json:"category"` // empty for transfers
	Note        string           `json:"note,omitempty"`
	Account     string           `json:"account,omitempty"` // Account.id; required for transfers
	Refund      bool             `json:"refund,omitempty"`  // expenses only: money back
	// Transfers only: the receiving account, and what it received in its own currency. Zero means
	// the amount converted at the day's rate.
	ToAccount     string `json:"to_account,omitempty"`
	ToAmountMinor int64  `json:"to_amount_minor,omitempty"`
}

// ImportPreviewInput is the body of POST /v1/imports/preview.
type ImportPreviewInput struct {
	Account string `json:"account"` // Account.id the statement belongs to
	File    string `json:"file"`    // the statement file, base64
}

// ImportStatus says what booking a statement line would do.
type ImportStatus string

const (
	ImportNew           ImportStatus = "new"
	ImportDone          ImportStatus = "imported"       // this exact line was imported before
	ImportDuplicate     ImportStatus = "duplicate"      // an entry with the same amount a few days apart exists
	ImportBeforeOpening ImportStatus = "before_opening" // before the account's opening date: cannot be booked
)

// ImportLine is a statement line with the entry suggested for it.
type ImportLine struct {
	Key         string                   `json:"key"`
	Day         string                   `json:"day"`          // YYYY-MM-DD
	AmountMinor int64                    `json:"amount_minor"` // signed: negative = money out of the account
	Currency    string                   `json:"currency"`
	Description string                   `json:"description"` // the bank's text
	Status      ImportStatus             `json:"status"`
	Match       *transaction.Transaction `json:"match,omitempty"` // the existing entry, when imported or duplicate

	// The suggested entry. Learned means it copies an earlier import from the same counterparty.
	Kind         transaction.Kind `json:"kind"`
	Merchant     string           `json:"merchant"`
	Category     string           `json:"category,omitempty"`
	OtherAccount string           `json:"other_account,omitempty"` // transfers: the account on the other side
	Learned      bool             `json:"learned"`
}

// ImportPreview is the reply of POST /v1/imports/preview.
type ImportPreview struct {
	Format  string       `json:"format"` // "sg", "revolut" or "bancolombia"
	Account string       `json:"account"`
	Lines   []ImportLine `json:"lines"`
	Skipped int          `json:"skipped"` // rows that are not completed movements in the account's currency
}

// ImportEntry is a line to book, as the person confirmed it. A positive expense is a refund; a
// transfer goes to OtherAccount when the amount is negative and comes from it otherwise.
type ImportEntry struct {
	Key          string           `json:"key"`
	Day          string           `json:"day"`
	AmountMinor  int64            `json:"amount_minor"`
	Currency     string           `json:"currency"`
	Kind         transaction.Kind `json:"kind"`
	Merchant     string           `json:"merchant"`
	Category     string           `json:"category,omitempty"`
	Note         string           `json:"note,omitempty"`
	OtherAccount string           `json:"other_account,omitempty"`
}

// ImportInput is the body of POST /v1/imports: up to 200 lines of one statement.
type ImportInput struct {
	Account string        `json:"account"`
	Lines   []ImportEntry `json:"lines"`
}

// ImportResult counts what POST /v1/imports did.
type ImportResult struct {
	Added        int `json:"added"`
	AlreadyThere int `json:"already_there"` // imported before, so skipped
}

// AccountKind says whether an account holds money or is a debt.
type AccountKind string

const (
	AccountCash   AccountKind = "cash"   // bank or wallet balance
	AccountCredit AccountKind = "credit" // credit card: the balance is negative while you owe
)

// Account is a place money sits. Its balance is the opening balance plus every transaction tied to
// it from the opening day on; nothing else is stored, so the balance cannot drift from the entries.
type Account struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Kind         AccountKind `json:"kind"`
	Currency     string      `json:"currency"`      // fixed once the account exists
	OpeningMinor int64       `json:"opening_minor"` // balance at the start of OpeningDate
	OpeningDate  string      `json:"opening_date"`  // YYYY-MM-DD
}

// Rates converts between currencies. internal/fx implements it with daily reference rates.
type Rates interface {
	// Rate returns how many units of to one unit of from was worth on day (YYYY-MM-DD).
	Rate(ctx context.Context, day, from, to string) (float64, error)
}

// Unit is the period of a recurring rule.
type Unit string

const (
	UnitWeek  Unit = "week"
	UnitMonth Unit = "month"
	UnitYear  Unit = "year"
)

// RuleInput is the body of POST /v1/recurring and PUT /v1/recurring/{id}.
type RuleInput struct {
	Kind        transaction.Kind `json:"kind"`
	AmountMinor int64            `json:"amount_minor"`
	Currency    string           `json:"currency"`
	Merchant    string           `json:"merchant"`
	Category    string           `json:"category"`
	Note        string           `json:"note,omitempty"`
	StartDate   string           `json:"start_date"`         // YYYY-MM-DD; may be in the past (backfilled)
	EndDate     string           `json:"end_date,omitempty"` // YYYY-MM-DD, inclusive
	Every       int              `json:"every"`              // 1-12
	Unit        Unit             `json:"unit"`
}

// Rule is a stored recurring rule. Each occurrence becomes a transaction with source "recurring" and
// dedup key "recurring:<rule id>:<YYYY-MM-DD>".
type Rule struct {
	ID string `json:"id"`
	RuleInput
	// Through is the last day already turned into transactions. Only later days are ever created, so
	// editing a rule changes only the future, and a deleted occurrence never comes back.
	Through string `json:"-"`
}

// CategoryTotal is the spending of one expense category.
type CategoryTotal struct {
	Category   string `json:"category"`
	SpentMinor int64  `json:"spent_minor"`
	Count      int    `json:"count"`
}

// MonthTotal is one month of the summary's 12-month history.
type MonthTotal struct {
	Month         string          `json:"month"`
	SpentMinor    int64           `json:"spent_minor"`
	IncomeMinor   int64           `json:"income_minor"`
	InvestedMinor int64           `json:"invested_minor"`
	ByCategory    []CategoryTotal `json:"by_category"`
}

// Summary is the response of GET /v1/summary.
type Summary struct {
	Month         string          `json:"month"`
	Currency      string          `json:"currency"`
	SpentMinor    int64           `json:"spent_minor"`
	IncomeMinor   int64           `json:"income_minor"`
	InvestedMinor int64           `json:"invested_minor"`
	Count         int             `json:"count"`
	ByCategory    []CategoryTotal `json:"by_category"`
	Months        []MonthTotal    `json:"months"` // the 12 months ending at Month, oldest first
}

// Holding is one asset or debt in the net worth.
type Holding struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"` // cash, investment, property, liability
	Currency     string `json:"currency"`
	BalanceMinor int64  `json:"balance_minor"` // in Currency; liabilities are negative
	BaseMinor    int64  `json:"base_minor"`    // in NetWorth.Currency at today's rate
}

// NetWorth is the response of GET /v1/net-worth.
type NetWorth struct {
	AsOf     string    `json:"as_of"`
	Currency string    `json:"currency"`
	Holdings []Holding `json:"holdings"`
}

var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("duplicate")
)

// Store persists the ledger: MemoryStore for quick local runs, dynamostore in AWS. Both pass
// ledgertest.Run.
type Store interface {
	// Categories returns the saved list, or ok=false if none was ever saved.
	Categories(ctx context.Context) (list []Category, ok bool, err error)
	SaveCategories(ctx context.Context, list []Category) error

	Accounts(ctx context.Context) ([]Account, error)
	SaveAccounts(ctx context.Context, list []Account) error

	// TransactionsBetween returns transactions with from <= OccurredAt < to, in any order.
	TransactionsBetween(ctx context.Context, from, to time.Time) ([]transaction.Transaction, error)
	Transaction(ctx context.Context, id string) (transaction.Transaction, error)
	// InsertTransaction fails with ErrDuplicate if the id already exists. Every producer derives the
	// id from the dedup key (one id per key, e.g. "o-<rule>-<day>"), so this also keeps dedup keys
	// unique without a second lookup.
	InsertTransaction(ctx context.Context, t transaction.Transaction) error
	ReplaceTransaction(ctx context.Context, t transaction.Transaction) error
	DeleteTransaction(ctx context.Context, id string) error

	Rules(ctx context.Context) ([]Rule, error)
	Rule(ctx context.Context, id string) (Rule, error)
	PutRule(ctx context.Context, r Rule) error
	DeleteRule(ctx context.Context, id string) error
}
