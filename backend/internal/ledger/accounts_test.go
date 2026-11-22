package ledger

import (
	"context"
	"errors"
	"testing"

	"finance/internal/transaction"
)

// testRates: 1 EUR = 1.25 USD = 4000 COP, every day.
type testRates struct{}

var perEUR = map[string]float64{"EUR": 1, "USD": 1.25, "COP": 4000}

func (testRates) Rate(_ context.Context, _, from, to string) (float64, error) {
	f, t := perEUR[from], perEUR[to]
	if f == 0 || t == 0 {
		return 0, errors.New("no rate")
	}
	return t / f, nil
}

var testAccounts = []Account{
	{ID: "sg", Name: "SG", Kind: AccountCash, Currency: "EUR", OpeningMinor: 100000, OpeningDate: "2026-10-01"},
	{ID: "arq", Name: "ARQ", Kind: AccountCash, Currency: "USD", OpeningMinor: 50000, OpeningDate: "2026-10-01"},
	{ID: "bco", Name: "Bancolombia", Kind: AccountCash, Currency: "COP", OpeningMinor: 400000000, OpeningDate: "2026-10-01"},
	{ID: "visa", Name: "Visa", Kind: AccountCredit, Currency: "COP", OpeningMinor: -40000000, OpeningDate: "2026-10-01"},
}

func withAccounts(t *testing.T) (*Service, context.Context) {
	t.Helper()
	c := &clock{}
	c.set("2026-11-22 12:00")
	s := newTestService(NewMemoryStore(), c)
	ctx := context.Background()
	if _, err := s.SaveAccounts(ctx, testAccounts); err != nil {
		t.Fatal(err)
	}
	return s, ctx
}

func mustCreate(t *testing.T, s *Service, in TransactionInput) transaction.Transaction {
	t.Helper()
	tx, err := s.CreateTransaction(context.Background(), in)
	if err != nil {
		t.Fatalf("%+v: %v", in, err)
	}
	return tx
}

func balances(t *testing.T, s *Service) map[string][2]int64 {
	t.Helper()
	nw, err := s.NetWorth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][2]int64{}
	for _, h := range nw.Holdings {
		out[h.ID] = [2]int64{h.BalanceMinor, h.BaseMinor}
	}
	return out
}

func TestAccountsBalancesAndNetWorth(t *testing.T) {
	s, _ := withAccounts(t)

	// A euro card payment on the COP credit card: 12.50 EUR = 50,000.00 COP of debt.
	tx := mustCreate(t, s, TransactionInput{Kind: transaction.KindExpense, AmountMinor: 1250, Currency: "EUR",
		OccurredAt: "2026-10-05T12:00:00Z", Merchant: "Café", Category: "restaurants", Account: "visa"})
	if tx.AccountMinor != -5000000 || tx.BaseMinor != -1250 {
		t.Errorf("account %d base %d, want -5000000 -1250", tx.AccountMinor, tx.BaseMinor)
	}
	// Income in USD on ARQ.
	mustCreate(t, s, TransactionInput{Kind: transaction.KindIncome, AmountMinor: 25000, Currency: "USD",
		OccurredAt: "2026-10-10T12:00:00Z", Merchant: "Client", Category: "salary", Account: "arq"})
	// Paying the card from Bancolombia: same currency, amount received defaults to amount sent.
	pay := mustCreate(t, s, TransactionInput{Kind: transaction.KindTransfer, AmountMinor: 40000000, Currency: "COP",
		OccurredAt: "2026-10-15T12:00:00Z", Merchant: "Pay Visa", Account: "bco", ToAccount: "visa"})
	if pay.ToMinor != 40000000 || pay.AmountMinor != -40000000 {
		t.Errorf("transfer: %+v", pay)
	}
	// EUR to USD with a fee: 100 EUR leaves, 120 USD arrives.
	mustCreate(t, s, TransactionInput{Kind: transaction.KindTransfer, AmountMinor: 10000, Currency: "EUR",
		OccurredAt: "2026-10-20T12:00:00Z", Merchant: "To ARQ", Account: "sg", ToAccount: "arq", ToAmountMinor: 12000})
	// Before the opening day: already part of the opening balance.
	mustCreate(t, s, TransactionInput{Kind: transaction.KindExpense, AmountMinor: 999, Currency: "EUR",
		OccurredAt: "2026-09-30T12:00:00Z", Merchant: "Old", Category: "other", Account: "sg"})
	// No account: changes no balance.
	mustCreate(t, s, TransactionInput{Kind: transaction.KindExpense, AmountMinor: 500, Currency: "EUR",
		OccurredAt: "2026-10-21T12:00:00Z", Merchant: "Cash", Category: "other"})

	want := map[string][2]int64{
		"sg":   {90000, 90000},     // 1000 - 100 EUR
		"arq":  {87000, 69600},     // 500 + 250 + 120 USD = 870 USD = 696 EUR
		"bco":  {360000000, 90000}, // 4,000,000 - 400,000 COP = 3,600,000 COP = 900 EUR
		"visa": {-5000000, -1250},  // -400,000 + 400,000 - 50,000 COP
	}
	got := balances(t, s)
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s: got %v, want %v", id, got[id], w)
		}
	}
}

func TestSummaryCountsRefundsAndSkipsTransfers(t *testing.T) {
	s, ctx := withAccounts(t)
	mustCreate(t, s, TransactionInput{Kind: transaction.KindExpense, AmountMinor: 4000, Currency: "EUR",
		OccurredAt: "2026-11-05T12:00:00Z", Merchant: "Dinner", Category: "restaurants", Account: "sg"})
	mustCreate(t, s, TransactionInput{Kind: transaction.KindExpense, AmountMinor: 1500, Currency: "EUR",
		OccurredAt: "2026-11-06T12:00:00Z", Merchant: "Friend pays back", Category: "restaurants", Account: "sg", Refund: true})
	mustCreate(t, s, TransactionInput{Kind: transaction.KindExpense, AmountMinor: 4000000, Currency: "COP",
		OccurredAt: "2026-11-07T12:00:00Z", Merchant: "Taxi", Category: "other", Account: "bco"})
	mustCreate(t, s, TransactionInput{Kind: transaction.KindTransfer, AmountMinor: 10000, Currency: "EUR",
		OccurredAt: "2026-11-08T12:00:00Z", Merchant: "To ARQ", Account: "sg", ToAccount: "arq"})

	sum, err := s.Summary(ctx, "2026-11")
	if err != nil {
		t.Fatal(err)
	}
	// 40 - 15 restaurants + 10 EUR of COP taxi; the transfer is not spending.
	if sum.SpentMinor != 3500 || sum.IncomeMinor != 0 || sum.Count != 4 {
		t.Errorf("spent %d income %d count %d, want 3500 0 4", sum.SpentMinor, sum.IncomeMinor, sum.Count)
	}
	if len(sum.ByCategory) != 2 || sum.ByCategory[0] != (CategoryTotal{"restaurants", 2500, 2}) {
		t.Errorf("by category %+v", sum.ByCategory)
	}
}

func TestTransferAndAccountValidation(t *testing.T) {
	s, ctx := withAccounts(t)
	base := TransactionInput{Kind: transaction.KindTransfer, AmountMinor: 100, Currency: "EUR",
		OccurredAt: "2026-11-08T12:00:00Z", Merchant: "Move", Account: "sg", ToAccount: "arq"}
	cases := map[string]func(in *TransactionInput){
		"no from":        func(in *TransactionInput) { in.Account = "" },
		"no to":          func(in *TransactionInput) { in.ToAccount = "" },
		"same account":   func(in *TransactionInput) { in.ToAccount = "sg" },
		"wrong currency": func(in *TransactionInput) { in.Currency = "USD" },
		"category":       func(in *TransactionInput) { in.Category = "other" },
		"refund":         func(in *TransactionInput) { in.Refund = true },
		"unknown acct":   func(in *TransactionInput) { in.Account = "nope" },
		"to on expense": func(in *TransactionInput) {
			in.Kind, in.Category = transaction.KindExpense, "other"
		},
		"refund on income": func(in *TransactionInput) {
			in.Kind, in.Category, in.ToAccount, in.Refund = transaction.KindIncome, "salary", "", true
		},
		"no rate": func(in *TransactionInput) {
			in.Kind, in.Category, in.ToAccount, in.Currency = transaction.KindExpense, "other", "", "GBP"
		},
	}
	for name, mutate := range cases {
		in := base
		mutate(&in)
		var ve ValidationError
		if _, err := s.CreateTransaction(ctx, in); !errors.As(err, &ve) {
			t.Errorf("%s: want ValidationError, got %v", name, err)
		}
	}

	changed := append([]Account(nil), testAccounts...)
	changed[1].Currency = "EUR"
	if _, err := s.SaveAccounts(ctx, changed); err == nil {
		t.Error("changing an account's currency: want error")
	}
	dup := append(append([]Account(nil), testAccounts...), Account{ID: "x", Name: "sg", Kind: AccountCash, Currency: "EUR", OpeningDate: "2026-10-01"})
	if _, err := s.SaveAccounts(ctx, dup); err == nil {
		t.Error("duplicate name: want error")
	}
}

func TestImportIsIdempotent(t *testing.T) {
	s, ctx := withAccounts(t)
	in := TransactionInput{Kind: transaction.KindExpense, AmountMinor: 700, Currency: "EUR",
		OccurredAt: "2026-11-08T12:00:00Z", Merchant: "Bakery", Category: "restaurants", Account: "sg"}
	a, err := s.ImportTransaction(ctx, in, "sg:2026-11-08:1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportTransaction(ctx, in, "sg:2026-11-08:1"); !errors.Is(err, ErrDuplicate) {
		t.Errorf("second import: want ErrDuplicate, got %v", err)
	}
	if a.Source != transaction.SourceManual {
		t.Errorf("imported entries stay editable: source %q", a.Source)
	}
}
