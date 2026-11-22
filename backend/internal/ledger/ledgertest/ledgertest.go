// Package ledgertest holds the behaviour every ledger.Store must have, so the in-memory store and
// DynamoDB are tested by the same cases.
package ledgertest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"finance/internal/ledger"
	"finance/internal/transaction"
)

// Run checks a Store. newStore must return an empty store each time it is called.
func Run(t *testing.T, newStore func(t *testing.T) ledger.Store) {
	ctx := context.Background()
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	tx := func(id, when string) transaction.Transaction {
		return transaction.Transaction{
			ID: id, Kind: transaction.KindExpense, OccurredAt: at(when), AmountMinor: -1250, Currency: "EUR",
			Merchant: "Fresh Market", Category: "groceries", Source: transaction.SourceManual, DedupKey: "manual-" + id,
		}
	}

	t.Run("categories", func(t *testing.T) {
		s := newStore(t)
		if _, ok, err := s.Categories(ctx); err != nil || ok {
			t.Fatalf("empty store: ok=%v err=%v", ok, err)
		}
		want := []ledger.Category{{ID: "other", Kind: transaction.KindExpense, Name: "Other", Color: 7}}
		if err := s.SaveCategories(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, ok, err := s.Categories(ctx)
		if err != nil || !ok || !slices.Equal(got, want) {
			t.Errorf("got %v ok=%v err=%v, want %v", got, ok, err, want)
		}
	})

	t.Run("accounts", func(t *testing.T) {
		s := newStore(t)
		if got, err := s.Accounts(ctx); err != nil || len(got) != 0 {
			t.Fatalf("empty store: %v err=%v", got, err)
		}
		want := []ledger.Account{
			{ID: "sg", Name: "SG", Kind: ledger.AccountCash, Currency: "EUR", OpeningMinor: 338832, OpeningDate: "2026-10-01"},
			{ID: "visa", Name: "Visa", Kind: ledger.AccountCredit, Currency: "COP", OpeningMinor: -1, OpeningDate: "2026-10-01"},
		}
		if err := s.SaveAccounts(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := s.Accounts(ctx)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("got %v err=%v, want %v", got, err, want)
		}
	})

	t.Run("transaction round trip", func(t *testing.T) {
		s := newStore(t)
		in := tx("m-1", "2026-11-09T18:30:00.123456789Z")
		in.Note = "weekly shop"
		in.CardLast4 = "4821"
		in.Account, in.AccountMinor, in.BaseMinor = "visa", -5000000, -1250
		in.ToAccount, in.ToMinor, in.Refund = "sg", 1, true
		if err := s.InsertTransaction(ctx, in); err != nil {
			t.Fatal(err)
		}
		got, err := s.Transaction(ctx, "m-1")
		if err != nil {
			t.Fatal(err)
		}
		if !got.OccurredAt.Equal(in.OccurredAt) {
			t.Errorf("time: got %v, want %v", got.OccurredAt, in.OccurredAt)
		}
		got.OccurredAt = in.OccurredAt
		if got != in {
			t.Errorf("got %+v, want %+v", got, in)
		}
		if err := s.InsertTransaction(ctx, in); !errors.Is(err, ledger.ErrDuplicate) {
			t.Errorf("second insert: got %v, want ErrDuplicate", err)
		}
		if _, err := s.Transaction(ctx, "nope"); !errors.Is(err, ledger.ErrNotFound) {
			t.Errorf("missing: got %v, want ErrNotFound", err)
		}
	})

	t.Run("replace and delete", func(t *testing.T) {
		s := newStore(t)
		in := tx("m-1", "2026-11-09T10:00:00Z")
		if err := s.ReplaceTransaction(ctx, in); !errors.Is(err, ledger.ErrNotFound) {
			t.Errorf("replace missing: got %v, want ErrNotFound", err)
		}
		if err := s.InsertTransaction(ctx, in); err != nil {
			t.Fatal(err)
		}
		in.AmountMinor = -999
		in.OccurredAt = at("2026-10-01T10:00:00Z") // moves to another month
		if err := s.ReplaceTransaction(ctx, in); err != nil {
			t.Fatal(err)
		}
		oct, err := s.TransactionsBetween(ctx, at("2026-10-01T00:00:00Z"), at("2026-11-01T00:00:00Z"))
		if err != nil || len(oct) != 1 || oct[0].AmountMinor != -999 {
			t.Errorf("after replace: %+v err=%v", oct, err)
		}
		nov, err := s.TransactionsBetween(ctx, at("2026-11-01T00:00:00Z"), at("2026-12-01T00:00:00Z"))
		if err != nil || len(nov) != 0 {
			t.Errorf("old month still has it: %+v err=%v", nov, err)
		}
		if err := s.DeleteTransaction(ctx, "m-1"); err != nil {
			t.Fatal(err)
		}
		if err := s.DeleteTransaction(ctx, "m-1"); !errors.Is(err, ledger.ErrNotFound) {
			t.Errorf("second delete: got %v, want ErrNotFound", err)
		}
		if _, err := s.Transaction(ctx, "m-1"); !errors.Is(err, ledger.ErrNotFound) {
			t.Errorf("after delete: got %v, want ErrNotFound", err)
		}
	})

	t.Run("range is from inclusive, to exclusive", func(t *testing.T) {
		s := newStore(t)
		for id, when := range map[string]string{
			"before": "2026-10-31T23:59:59.999999999Z",
			"first":  "2026-11-01T00:00:00Z",
			"middle": "2026-11-15T12:00:00Z",
			"last":   "2026-11-30T23:59:59.999999999Z",
			"after":  "2026-12-01T00:00:00Z",
		} {
			if err := s.InsertTransaction(ctx, tx(id, when)); err != nil {
				t.Fatal(err)
			}
		}
		got, err := s.TransactionsBetween(ctx, at("2026-11-01T00:00:00Z"), at("2026-12-01T00:00:00Z"))
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, g := range got {
			ids = append(ids, g.ID)
		}
		slices.Sort(ids)
		if want := []string{"first", "last", "middle"}; !slices.Equal(ids, want) {
			t.Errorf("got %v, want %v", ids, want)
		}
	})

	t.Run("time zones are compared as instants", func(t *testing.T) {
		s := newStore(t)
		// 00:30 on 1 November in Paris is 23:30 on 31 October in UTC.
		if err := s.InsertTransaction(ctx, tx("m-1", "2026-11-01T00:30:00+01:00")); err != nil {
			t.Fatal(err)
		}
		paris := time.FixedZone("CET", 3600)
		got, err := s.TransactionsBetween(ctx, time.Date(2026, 11, 1, 0, 0, 0, 0, paris), time.Date(2026, 12, 1, 0, 0, 0, 0, paris))
		if err != nil || len(got) != 1 {
			t.Errorf("got %+v err=%v", got, err)
		}
	})

	t.Run("many transactions", func(t *testing.T) {
		s := newStore(t)
		const n = 120 // ten years of a monthly rule
		for i := range n {
			when := time.Date(2016, time.Month(1+i), 1, 11, 0, 0, 0, time.UTC).Format(time.RFC3339)
			if err := s.InsertTransaction(ctx, tx(fmt.Sprintf("o-%03d", i), when)); err != nil {
				t.Fatal(err)
			}
		}
		got, err := s.TransactionsBetween(ctx, at("2016-01-01T00:00:00Z"), at("2027-01-01T00:00:00Z"))
		if err != nil || len(got) != n {
			t.Errorf("got %d err=%v, want %d", len(got), err, n)
		}
	})

	t.Run("rules", func(t *testing.T) {
		s := newStore(t)
		r := ledger.Rule{
			ID: "r-1",
			RuleInput: ledger.RuleInput{
				Kind: transaction.KindExpense, AmountMinor: 1299, Currency: "EUR", Merchant: "StreamBox",
				Category: "subscriptions", StartDate: "2026-01-05", Every: 1, Unit: ledger.UnitMonth,
			},
			Through: "2026-11-10",
		}
		if _, err := s.Rule(ctx, "r-1"); !errors.Is(err, ledger.ErrNotFound) {
			t.Errorf("missing: got %v, want ErrNotFound", err)
		}
		if err := s.PutRule(ctx, r); err != nil {
			t.Fatal(err)
		}
		r.EndDate = "2026-12-31"
		if err := s.PutRule(ctx, r); err != nil {
			t.Fatal(err)
		}
		got, err := s.Rule(ctx, "r-1")
		if err != nil || got != r {
			t.Errorf("got %+v err=%v, want %+v", got, err, r)
		}
		if err := s.PutRule(ctx, ledger.Rule{ID: "r-2", RuleInput: r.RuleInput}); err != nil {
			t.Fatal(err)
		}
		all, err := s.Rules(ctx)
		if err != nil || len(all) != 2 {
			t.Errorf("list: got %d err=%v", len(all), err)
		}
		if err := s.DeleteRule(ctx, "r-1"); err != nil {
			t.Fatal(err)
		}
		if err := s.DeleteRule(ctx, "r-1"); !errors.Is(err, ledger.ErrNotFound) {
			t.Errorf("second delete: got %v, want ErrNotFound", err)
		}
	})

	t.Run("kinds of item stay apart", func(t *testing.T) {
		s := newStore(t)
		if err := s.InsertTransaction(ctx, tx("same", "2026-11-01T10:00:00Z")); err != nil {
			t.Fatal(err)
		}
		if err := s.PutRule(ctx, ledger.Rule{ID: "same", RuleInput: ledger.RuleInput{Every: 1, Unit: ledger.UnitMonth}}); err != nil {
			t.Fatal(err)
		}
		if err := s.DeleteRule(ctx, "same"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Transaction(ctx, "same"); err != nil {
			t.Errorf("deleting a rule removed a transaction with the same id: %v", err)
		}
	})
}
