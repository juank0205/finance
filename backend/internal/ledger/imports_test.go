package ledger

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"finance/internal/transaction"
)

const sgHeader = "Date de l'opération;Libellé;Détail de l'écriture;Montant de l'opération;Devise\n"

func sgStatement(lines ...string) string {
	return base64.StdEncoding.EncodeToString([]byte(sgHeader + strings.Join(lines, "\n") + "\n"))
}

func preview(t *testing.T, s *Service, file string) ImportPreview {
	t.Helper()
	p, err := s.PreviewImport(context.Background(), ImportPreviewInput{Account: "sg", File: file})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// booking books a previewed line as suggested, with optional changes.
func booking(l ImportLine, change func(*ImportEntry)) ImportEntry {
	e := ImportEntry{Key: l.Key, Day: l.Day, AmountMinor: l.AmountMinor, Currency: l.Currency,
		Kind: l.Kind, Merchant: l.Merchant, Category: l.Category, OtherAccount: l.OtherAccount}
	if change != nil {
		change(&e)
	}
	return e
}

func TestImportPreviewAndBook(t *testing.T) {
	s, ctx := withAccounts(t)
	// Already loaded from Google Wallet, two days before the card line posts.
	wallet := mustCreate(t, s, TransactionInput{Kind: transaction.KindExpense, AmountMinor: 1278, Currency: "EUR",
		OccurredAt: "2026-11-03T12:00:00Z", Merchant: "Uber Eats", Category: "restaurants", Account: "sg"})

	file := sgStatement(
		"20/11/2026;CARTE X1111 16/11 ;CARTE X1111 16/11 Urban Soccer COMMERCE ELECTRONIQUE 1106;-7,50;EUR",
		"20/11/2026;VIR INST RE 6824;VIR INST RE 6824 DE: M. JOHN DOE DATE: 20/11/2026 00:44 MOTIF: Uber;12,50;EUR",
		"12/11/2026;000001 VIR INSTANT;000001 VIR INSTANTANE EMIS LOGITEL POUR: Juan Example 12 11 SG 01842;-100,00;EUR",
		"10/11/2026;VIR RECU;VIR RECU 1234 DE: BANCOLOMBIA SA MOTIF: Ahorros;50,00;EUR",
		"05/11/2026;CARTE X1111 05/11 ;CARTE X1111 05/11 UBER * EATS PENDING;-12,78;EUR",
		"30/09/2026;CARTE X1111 30/09 ;CARTE X1111 30/09 OLD SHOP;-1,00;EUR",
	)
	p := preview(t, s, file)
	if p.Format != "sg" || len(p.Lines) != 6 {
		t.Fatalf("format %q, %d lines", p.Format, len(p.Lines))
	}
	soccer, payback, toArq, fromBco, uber, old := p.Lines[0], p.Lines[1], p.Lines[2], p.Lines[3], p.Lines[4], p.Lines[5]
	if soccer.Status != ImportNew || soccer.Kind != transaction.KindExpense || soccer.Category != "other" ||
		soccer.Merchant != "Urban Soccer" || soccer.Day != "2026-11-16" || soccer.Learned {
		t.Errorf("first sight of a merchant: %+v", soccer)
	}
	if payback.Kind != transaction.KindIncome {
		t.Errorf("money in defaults to income: %+v", payback)
	}
	if uber.Status != ImportDuplicate || uber.Match == nil || uber.Match.ID != wallet.ID {
		t.Errorf("Wallet entry two days apart: %+v", uber)
	}
	if old.Status != ImportBeforeOpening {
		t.Errorf("before opening: %+v", old)
	}

	res, err := s.Import(ctx, ImportInput{Account: "sg", Lines: []ImportEntry{
		booking(soccer, func(e *ImportEntry) { e.Category = "restaurants" }),
		// A friend paying back: an expense with money in is a refund.
		booking(payback, func(e *ImportEntry) {
			e.Kind, e.Merchant, e.Category = transaction.KindExpense, "Payback: John", "restaurants"
		}),
		booking(toArq, func(e *ImportEntry) {
			e.Kind, e.Merchant, e.Category, e.OtherAccount = transaction.KindTransfer, "SG to ARQ", "", "arq"
		}),
		booking(fromBco, func(e *ImportEntry) {
			e.Kind, e.Merchant, e.Category, e.OtherAccount = transaction.KindTransfer, "From Bancolombia", "", "bco"
		}),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 4 || res.AlreadyThere != 0 {
		t.Errorf("result %+v", res)
	}

	// 1000 - 12.78 (Wallet) - 7.50 + 12.50 - 100 + 50 = 942.22 EUR; ARQ got 100 EUR = 125 USD;
	// Bancolombia sent 50 EUR = 200,000 COP.
	b := balances(t, s)
	if b["sg"][0] != 94222 || b["arq"][0] != 50000+12500 || b["bco"][0] != 400000000-20000000 {
		t.Errorf("balances %v", b)
	}

	// Booking the same lines again adds nothing.
	res, err = s.Import(ctx, ImportInput{Account: "sg", Lines: []ImportEntry{booking(soccer, nil)}})
	if err != nil || res.Added != 0 || res.AlreadyThere != 1 {
		t.Errorf("again: %+v %v", res, err)
	}

	// The next statement overlaps the first and has new lines from known counterparties.
	next := preview(t, s, sgStatement(
		"27/11/2026;CARTE X1111 25/11 ;CARTE X1111 25/11 Urban Soccer COMMERCE ELECTRONIQUE 9999;-7,50;EUR",
		"26/11/2026;000001 VIR INSTANT;000001 VIR INSTANTANE EMIS LOGITEL POUR: Juan Example 26 11 SG 01842;-40,00;EUR",
		"20/11/2026;CARTE X1111 16/11 ;CARTE X1111 16/11 Urban Soccer COMMERCE ELECTRONIQUE 1106;-7,50;EUR",
	))
	learnedSoccer, learnedTransfer, again := next.Lines[0], next.Lines[1], next.Lines[2]
	if !learnedSoccer.Learned || learnedSoccer.Category != "restaurants" || learnedSoccer.Status != ImportNew {
		t.Errorf("learned card line: %+v", learnedSoccer)
	}
	if !learnedTransfer.Learned || learnedTransfer.Kind != transaction.KindTransfer || learnedTransfer.OtherAccount != "arq" ||
		learnedTransfer.Merchant != "SG to ARQ" {
		t.Errorf("learned transfer: %+v", learnedTransfer)
	}
	if again.Status != ImportDone || again.Match == nil {
		t.Errorf("overlapping line: %+v", again)
	}
}

func TestImportRejectsBadLinesWithoutBookingAny(t *testing.T) {
	s, ctx := withAccounts(t)
	good := ImportEntry{Key: "sg:2026-11-08:BAKERY:-700:1", Day: "2026-11-08", AmountMinor: -700, Currency: "EUR",
		Kind: transaction.KindExpense, Merchant: "Bakery", Category: "restaurants"}
	for name, bad := range map[string]func(*ImportEntry){
		"unknown format":    func(e *ImportEntry) { e.Key = "other:1" },
		"before opening":    func(e *ImportEntry) { e.Day = "2026-09-30" },
		"income going out":  func(e *ImportEntry) { e.Kind, e.Category = transaction.KindIncome, "salary" },
		"transfer no other": func(e *ImportEntry) { e.Kind, e.Category = transaction.KindTransfer, "" },
		"other on expense":  func(e *ImportEntry) { e.OtherAccount = "arq" },
		"future":            func(e *ImportEntry) { e.Day = "2026-12-01" },
		"no amount":         func(e *ImportEntry) { e.AmountMinor = 0 },
	} {
		e := good
		e.Key = "sg:2026-11-09:" + name + ":-1:1"
		bad(&e)
		if _, err := s.Import(ctx, ImportInput{Account: "sg", Lines: []ImportEntry{good, e}}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if txs := mustTransactions(t, s, "2026-11"); len(txs) != 0 {
		t.Errorf("a rejected batch booked %d entries", len(txs))
	}
	if _, err := s.Import(ctx, ImportInput{Account: "nope", Lines: []ImportEntry{good}}); err == nil {
		t.Error("unknown account accepted")
	}
}

func TestImportPreviewRejects(t *testing.T) {
	s, ctx := withAccounts(t)
	for name, in := range map[string]ImportPreviewInput{
		"no account": {File: sgStatement()},
		"not base64": {Account: "sg", File: "!!!"},
		"not a file": {Account: "sg", File: base64.StdEncoding.EncodeToString([]byte("hello"))},
		"too large":  {Account: "sg", File: strings.Repeat("A", maxImportFile*2)},
	} {
		if _, err := s.PreviewImport(ctx, in); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// The first import keyed Revolut lines by row number; a re-upload recognizes those entries.
func TestImportRecognizesFirstImportKeys(t *testing.T) {
	s, ctx := withAccounts(t)
	if _, err := s.ImportTransaction(ctx, TransactionInput{Kind: transaction.KindExpense, AmountMinor: 1000, Currency: "EUR",
		OccurredAt: "2026-10-07T10:00:00Z", Merchant: "Izly", Category: "restaurants", Account: "sg"},
		"revolut:2026-10-07 11:58:43:Izly:-1000:7"); err != nil {
		t.Fatal(err)
	}
	file := "Type,Product,Started Date,Completed Date,Description,Amount,Fee,Currency,State,Balance\n" +
		"Card Payment,Current,2026-10-07 11:58:43,2026-10-08 12:49:47,Izly,-10.00,0.00,EUR,COMPLETED,0.00\n"
	p, err := s.PreviewImport(ctx, ImportPreviewInput{Account: "sg", File: base64.StdEncoding.EncodeToString([]byte(file))})
	if err != nil {
		t.Fatal(err)
	}
	if l := p.Lines[0]; l.Status != ImportDone || !l.Learned || l.Category != "restaurants" {
		t.Errorf("%+v", l)
	}
}
