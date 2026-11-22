package ledger

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"finance/internal/transaction"
)

// The same checks as the dashboard's forms and src/api/mock.ts; the server is the one that counts.

// ValidationError is a problem with the request that the person can fix. Its text is shown as is.
type ValidationError string

func (e ValidationError) Error() string { return string(e) }

var kinds = []transaction.Kind{transaction.KindExpense, transaction.KindIncome, transaction.KindInvestment}

// catchAll is the built-in category per kind: it can be renamed and recolored, never deleted.
var catchAll = map[transaction.Kind]string{
	transaction.KindExpense:    "other",
	transaction.KindIncome:     "income",
	transaction.KindInvestment: "investment",
}

// maxCategories is the size of the dashboard's categorical palette.
const maxCategories = 8

// maxAmountMinor is 100 million in major units: anything above it is a typo.
const maxAmountMinor = 100_000_000_00

var defaultCategories = []Category{
	{"subscriptions", transaction.KindExpense, "Subscriptions", 1},
	{"mortgage", transaction.KindExpense, "Mortgage", 2},
	{"bills", transaction.KindExpense, "Bills", 3},
	{"restaurants", transaction.KindExpense, "Restaurants", 4},
	{"groceries", transaction.KindExpense, "Groceries", 5},
	{"clothes", transaction.KindExpense, "Clothes", 6},
	{"other", transaction.KindExpense, "Other", 7},
	{"salary", transaction.KindIncome, "Salary", 1},
	{"income", transaction.KindIncome, "Other income", 2},
	{"index-funds", transaction.KindInvestment, "Index funds", 1},
	{"investment", transaction.KindInvestment, "Other investments", 2},
}

var (
	categoryID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)
)

func validKind(k transaction.Kind) bool { return slices.Contains(kinds, k) }

func validateCategories(list []Category) ([]Category, error) {
	ids := make(map[string]bool, len(list))
	for _, c := range list {
		if !validKind(c.Kind) {
			return nil, ValidationError("Unknown category type")
		}
		if !categoryID.MatchString(c.ID) || ids[c.ID] {
			return nil, ValidationError("Every category needs its own id")
		}
		ids[c.ID] = true
	}
	out := make([]Category, 0, len(list))
	for _, kind := range kinds {
		names := map[string]bool{}
		colors := map[int]bool{}
		n := 0
		for _, c := range list {
			if c.Kind != kind {
				continue
			}
			n++
			name := strings.TrimSpace(c.Name)
			if name == "" || utf8.RuneCountInString(name) > 30 {
				return nil, ValidationError("Every category needs a name of up to 30 characters")
			}
			if names[strings.ToLower(name)] {
				return nil, ValidationError("Duplicate name: " + name)
			}
			if c.Color < 1 || c.Color > maxCategories || colors[c.Color] {
				return nil, ValidationError("Each category needs its own color")
			}
			names[strings.ToLower(name)] = true
			colors[c.Color] = true
			out = append(out, Category{c.ID, c.Kind, name, c.Color})
		}
		if n > maxCategories {
			return nil, ValidationError("At most 8 categories per type")
		}
		if !ids[catchAll[kind]] {
			return nil, ValidationError("Catch-all categories cannot be deleted")
		}
	}
	return out, nil
}

// entryFields are the fields a single entry and a recurring rule share.
type entryFields struct {
	kind     transaction.Kind
	amount   int64
	currency string
	merchant string
	category string
	note     string
}

func validateEntry(f entryFields, categories []Category) (entryFields, error) {
	if !validKind(f.kind) {
		return f, ValidationError("Unknown type")
	}
	f, err := validateBasics(f)
	if err != nil {
		return f, err
	}
	if !slices.ContainsFunc(categories, func(c Category) bool { return c.ID == f.category && c.Kind == f.kind }) {
		return f, ValidationError("Pick a category of the same type")
	}
	return f, nil
}

// validateBasics checks what every entry has, transfers included: amount, currency, text.
func validateBasics(f entryFields) (entryFields, error) {
	if f.amount <= 0 || f.amount > maxAmountMinor {
		return f, ValidationError("Amount must be more than 0")
	}
	if !currencyRe.MatchString(f.currency) {
		return f, ValidationError("Unknown currency")
	}
	f.merchant = strings.TrimSpace(f.merchant)
	if f.merchant == "" || utf8.RuneCountInString(f.merchant) > 80 {
		return f, ValidationError("Description must be 1 to 80 characters")
	}
	f.note = strings.TrimSpace(f.note)
	if utf8.RuneCountInString(f.note) > 500 {
		return f, ValidationError("Note must be at most 500 characters")
	}
	return f, nil
}

// txFields is a checked TransactionInput.
type txFields struct {
	entryFields
	refund   bool
	from     *Account // the account the money left or entered; nil if none
	to       *Account // transfers only
	toAmount int64    // transfers only; 0 = convert
}

// validateTransaction checks in and returns the occurrence time. tomorrow is the start of the next
// day in the person's time zone: anything from then on is in the future.
func validateTransaction(in TransactionInput, categories []Category, accounts []Account, tomorrow time.Time) (txFields, time.Time, error) {
	f := entryFields{in.Kind, in.AmountMinor, in.Currency, in.Merchant, in.Category, in.Note}
	var err error
	if in.Kind == transaction.KindTransfer {
		if f, err = validateBasics(f); err == nil && f.category != "" {
			err = ValidationError("Transfers have no category")
		}
	} else {
		f, err = validateEntry(f, categories)
	}
	if err != nil {
		return txFields{}, time.Time{}, err
	}
	out := txFields{entryFields: f, refund: in.Refund}
	if in.Account != "" {
		if out.from = findAccount(accounts, in.Account); out.from == nil {
			return out, time.Time{}, ValidationError("Unknown account")
		}
	}
	if in.Refund && f.kind != transaction.KindExpense {
		return out, time.Time{}, ValidationError("Only expenses can be refunds")
	}
	if f.kind == transaction.KindTransfer {
		out.to = findAccount(accounts, in.ToAccount)
		switch {
		case out.from == nil:
			return out, time.Time{}, ValidationError("Pick the account the money left")
		case out.to == nil:
			return out, time.Time{}, ValidationError("Pick the account the money went to")
		case out.to.ID == out.from.ID:
			return out, time.Time{}, ValidationError("Pick two different accounts")
		case f.currency != out.from.Currency:
			return out, time.Time{}, ValidationError("A transfer is in the currency of the account it leaves")
		case in.ToAmountMinor < 0 || in.ToAmountMinor > maxAmountMinor:
			return out, time.Time{}, ValidationError("Invalid amount received")
		}
		out.toAmount = in.ToAmountMinor
	} else if in.ToAccount != "" || in.ToAmountMinor != 0 {
		return out, time.Time{}, ValidationError("Only transfers have a receiving account")
	}
	when, err := time.Parse(time.RFC3339, in.OccurredAt)
	if err != nil {
		return out, time.Time{}, ValidationError("Invalid date")
	}
	if !when.Before(tomorrow) {
		return out, time.Time{}, ValidationError("The date cannot be in the future")
	}
	return out, when.UTC(), nil
}

// validateRule checks in. today is "YYYY-MM-DD" in the person's time zone; a start more than 10 years
// back is refused so the backfill stays well under maxOccurrences.
func validateRule(in RuleInput, categories []Category, today string) (RuleInput, error) {
	f, err := validateEntry(entryFields{in.Kind, in.AmountMinor, in.Currency, in.Merchant, in.Category, in.Note}, categories)
	if err != nil {
		return in, err
	}
	start, ok := parseDay(in.StartDate)
	if !ok {
		return in, ValidationError("Invalid start date")
	}
	if t, _ := parseDay(today); start.Before(t.AddDate(-10, 0, 0)) {
		return in, ValidationError("The start date can be at most 10 years ago")
	}
	if in.EndDate != "" {
		if _, ok := parseDay(in.EndDate); !ok || in.EndDate < in.StartDate {
			return in, ValidationError("The end date must be on or after the start date")
		}
	}
	if in.Every < 1 || in.Every > 12 {
		return in, ValidationError("Repeat every 1 to 12")
	}
	if in.Unit != UnitWeek && in.Unit != UnitMonth && in.Unit != UnitYear {
		return in, ValidationError("Unknown repeat unit")
	}
	in.Kind, in.AmountMinor, in.Currency, in.Merchant, in.Category, in.Note =
		f.kind, f.amount, f.currency, f.merchant, f.category, f.note
	return in, nil
}
