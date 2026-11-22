package ledger

import (
	"context"
	"errors"
	"testing"
	"time"

	"finance/internal/transaction"
)

var paris = mustLoad("Europe/Paris")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// clock is a settable time source for tests.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func (c *clock) set(s string) {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, paris)
	if err != nil {
		panic(err)
	}
	c.t = t
}

func newTestService(store Store, c *clock) *Service {
	s := NewService(store, paris, "EUR", testRates{})
	s.now = c.now
	return s
}

func monthlyRule(start string, amount int64) RuleInput {
	return RuleInput{
		Kind: transaction.KindExpense, AmountMinor: amount, Currency: "EUR", Merchant: "StreamBox",
		Category: "subscriptions", StartDate: start, Every: 1, Unit: UnitMonth,
	}
}

func mustTransactions(t *testing.T, s *Service, month string) []transaction.Transaction {
	t.Helper()
	txs, err := s.Transactions(context.Background(), month)
	if err != nil {
		t.Fatal(err)
	}
	return txs
}

func TestRuleBackfillsAndEditsOnlyTheFuture(t *testing.T) {
	ctx := context.Background()
	c := &clock{}
	c.set("2026-11-10 12:00")
	s := newTestService(NewMemoryStore(), c)

	r, err := s.CreateRule(ctx, monthlyRule("2026-09-01", 1299))
	if err != nil {
		t.Fatal(err)
	}
	oct := mustTransactions(t, s, "2026-10")
	if len(oct) != 1 {
		t.Fatalf("October: got %d entries, want 1", len(oct))
	}
	got := oct[0]
	if got.AmountMinor != -1299 || got.Source != transaction.SourceRecurring || got.DedupKey != "recurring:"+r.ID+":2026-10-01" {
		t.Errorf("unexpected occurrence %+v", got)
	}
	if day := got.OccurredAt.In(paris).Format(dayLayout); day != "2026-10-01" {
		t.Errorf("occurrence on %s, want 2026-10-01", day)
	}

	in := monthlyRule("2026-09-01", 1599)
	if _, err := s.UpdateRule(ctx, r.ID, in); err != nil {
		t.Fatal(err)
	}
	if oct := mustTransactions(t, s, "2026-10"); oct[0].AmountMinor != -1299 {
		t.Errorf("editing the rule changed a past entry: %d", oct[0].AmountMinor)
	}

	c.set("2026-12-02 09:00")
	dec := mustTransactions(t, s, "2026-12")
	if len(dec) != 1 || dec[0].AmountMinor != -1599 {
		t.Errorf("December: got %+v, want one entry of -1599", dec)
	}
}

func TestDeletedOccurrenceStaysDeleted(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	c := &clock{}
	c.set("2026-11-10 12:00")
	s := newTestService(store, c)

	if _, err := s.CreateRule(ctx, monthlyRule("2026-09-01", 1000)); err != nil {
		t.Fatal(err)
	}
	oct := mustTransactions(t, s, "2026-10")
	if err := s.DeleteTransaction(ctx, oct[0].ID); err != nil {
		t.Fatal(err)
	}

	// A new day and a fresh process (a new Lambda container) both run the rules again.
	c.set("2026-11-11 08:00")
	fresh := newTestService(store, c)
	if oct := mustTransactions(t, fresh, "2026-10"); len(oct) != 0 {
		t.Errorf("deleted occurrence came back: %+v", oct)
	}
	if sep := mustTransactions(t, fresh, "2026-09"); len(sep) != 1 {
		t.Errorf("September: got %d entries, want 1", len(sep))
	}
}

func TestImportedTransactionsAreReadOnly(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	c := &clock{}
	c.set("2026-11-10 12:00")
	s := newTestService(store, c)

	wallet := transaction.Transaction{
		ID: "w-1", Kind: transaction.KindExpense, OccurredAt: c.t, AmountMinor: -450, Currency: "EUR",
		Merchant: "Fresh Market", Source: transaction.SourceWallet, DedupKey: "wallet-1",
	}
	if err := store.InsertTransaction(ctx, wallet); err != nil {
		t.Fatal(err)
	}
	in := TransactionInput{
		Kind: transaction.KindExpense, AmountMinor: 100, Currency: "EUR", OccurredAt: "2026-11-10T10:00:00Z",
		Merchant: "x", Category: "other",
	}
	if _, err := s.UpdateTransaction(ctx, "w-1", in); !errors.Is(err, ErrReadOnly) {
		t.Errorf("update: got %v, want ErrReadOnly", err)
	}
	if err := s.DeleteTransaction(ctx, "w-1"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("delete: got %v, want ErrReadOnly", err)
	}
	if err := s.DeleteTransaction(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing: got %v, want ErrNotFound", err)
	}
}

func TestTransactionValidation(t *testing.T) {
	ctx := context.Background()
	c := &clock{}
	c.set("2026-11-10 12:00")
	s := newTestService(NewMemoryStore(), c)

	base := TransactionInput{
		Kind: transaction.KindExpense, AmountMinor: 1250, Currency: "EUR", OccurredAt: "2026-11-10T22:59:00Z",
		Merchant: "  Fresh Market  ", Category: "groceries",
	}
	got, err := s.CreateTransaction(ctx, base)
	if err != nil {
		t.Fatalf("23:59 Paris time today should be accepted: %v", err)
	}
	if got.Merchant != "Fresh Market" || got.AmountMinor != -1250 || got.Source != transaction.SourceManual {
		t.Errorf("unexpected entry %+v", got)
	}

	bad := map[string]func(*TransactionInput){
		"tomorrow in Paris":   func(in *TransactionInput) { in.OccurredAt = "2026-11-10T23:00:00Z" },
		"bad date":            func(in *TransactionInput) { in.OccurredAt = "10/11/2026" },
		"zero amount":         func(in *TransactionInput) { in.AmountMinor = 0 },
		"negative amount":     func(in *TransactionInput) { in.AmountMinor = -5 },
		"category other kind": func(in *TransactionInput) { in.Category = "salary" },
		"unknown kind":        func(in *TransactionInput) { in.Kind = "gift" },
		"blank merchant":      func(in *TransactionInput) { in.Merchant = "   " },
		"lowercase currency":  func(in *TransactionInput) { in.Currency = "eur" },
	}
	for name, mutate := range bad {
		in := base
		mutate(&in)
		var ve ValidationError
		if _, err := s.CreateTransaction(ctx, in); !errors.As(err, &ve) {
			t.Errorf("%s: got %v, want a ValidationError", name, err)
		}
	}
}

func TestRuleStartAtMostTenYearsBack(t *testing.T) {
	c := &clock{}
	c.set("2026-11-10 12:00")
	s := newTestService(NewMemoryStore(), c)
	var ve ValidationError
	if _, err := s.CreateRule(context.Background(), monthlyRule("2016-11-09", 100)); !errors.As(err, &ve) {
		t.Errorf("got %v, want a ValidationError", err)
	}
	if _, err := s.CreateRule(context.Background(), monthlyRule("2016-11-10", 100)); err != nil {
		t.Errorf("exactly 10 years back should be accepted: %v", err)
	}
}

func TestSummary(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	c := &clock{}
	c.set("2026-11-10 12:00")
	s := newTestService(store, c)

	add := func(id string, kind transaction.Kind, at string, amount int64, category string) {
		when, _ := time.Parse(time.RFC3339, at)
		err := store.InsertTransaction(ctx, transaction.Transaction{
			ID: id, Kind: kind, OccurredAt: when, AmountMinor: amount, Currency: "EUR",
			Category: category, Source: transaction.SourceStatement, DedupKey: id,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	add("a", transaction.KindExpense, "2026-11-03T10:00:00Z", -1000, "groceries")
	add("b", transaction.KindExpense, "2026-11-04T10:00:00Z", -500, "groceries")
	add("c", transaction.KindExpense, "2026-11-05T10:00:00Z", -300, "deleted-category") // counts as "other"
	add("d", transaction.KindIncome, "2026-11-01T10:00:00Z", 5000, "salary")
	add("e", transaction.KindInvestment, "2026-11-02T10:00:00Z", -2000, "index-funds")
	// 00:30 on 1 November in Paris, still 31 October in UTC: belongs to November.
	add("f", transaction.KindExpense, "2026-10-31T23:30:00Z", -200, "groceries")
	add("g", transaction.KindExpense, "2026-10-15T10:00:00Z", -700, "bills")

	sum, err := s.Summary(ctx, "2026-11")
	if err != nil {
		t.Fatal(err)
	}
	if sum.SpentMinor != 2000 || sum.IncomeMinor != 5000 || sum.InvestedMinor != 2000 || sum.Count != 6 {
		t.Errorf("totals: %+v", sum)
	}
	want := []CategoryTotal{{"groceries", 1700, 3}, {"other", 300, 1}}
	if len(sum.ByCategory) != len(want) || sum.ByCategory[0] != want[0] || sum.ByCategory[1] != want[1] {
		t.Errorf("by category: got %+v, want %+v", sum.ByCategory, want)
	}
	if len(sum.Months) != 12 || sum.Months[0].Month != "2025-12" || sum.Months[11].Month != "2026-11" {
		t.Fatalf("months: got %d, from %s", len(sum.Months), sum.Months[0].Month)
	}
	if oct := sum.Months[10]; oct.Month != "2026-10" || oct.SpentMinor != 700 {
		t.Errorf("October: %+v", oct)
	}
}

func TestCategoryValidation(t *testing.T) {
	ctx := context.Background()
	s := newTestService(NewMemoryStore(), &clock{})

	saved, err := s.SaveCategories(ctx, append(clone(defaultCategories), Category{"c-1", transaction.KindIncome, "  Gifts ", 3}))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range saved {
		if c.ID == "c-1" && c.Name != "Gifts" {
			t.Errorf("name not trimmed: %q", c.Name)
		}
	}

	bad := map[string][]Category{
		"catch-all removed": without(defaultCategories, "other"),
		"duplicate name":    append(clone(defaultCategories), Category{"c-2", transaction.KindExpense, "groceries", 8}),
		"color taken":       append(clone(defaultCategories), Category{"c-2", transaction.KindExpense, "Travel", 1}),
		"duplicate id":      append(clone(defaultCategories), Category{"bills", transaction.KindIncome, "Bills", 3}),
		"bad id":            append(clone(defaultCategories), Category{"Bad Id", transaction.KindIncome, "X", 3}),
		"nine of a kind": append(clone(defaultCategories),
			Category{"c-2", transaction.KindExpense, "Travel", 8}, Category{"c-3", transaction.KindExpense, "Pets", 9}),
	}
	for name, list := range bad {
		var ve ValidationError
		if _, err := s.SaveCategories(ctx, list); !errors.As(err, &ve) {
			t.Errorf("%s: got %v, want a ValidationError", name, err)
		}
	}
}

func clone(list []Category) []Category { return append([]Category(nil), list...) }

func without(list []Category, id string) []Category {
	var out []Category
	for _, c := range list {
		if c.ID != id {
			out = append(out, c)
		}
	}
	return out
}
