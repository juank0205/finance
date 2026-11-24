package ledger

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"finance/internal/transaction"
)

// ErrReadOnly rejects edits to imported transactions (Google Wallet, statements): the source of truth
// for those is the bank, so only manual and recurring entries can be changed.
var ErrReadOnly = ValidationError("Only manual and recurring entries can be changed")

// Service implements the dashboard API on top of a Store.
type Service struct {
	store    Store
	loc      *time.Location // the person's time zone: decides which day and month a moment falls in
	currency string
	rates    Rates // nil: only amounts in currency work
	now      func() time.Time

	mu             sync.Mutex
	materializedOn string // day the recurring rules were last brought up to date in this process
}

func NewService(store Store, loc *time.Location, currency string, rates Rates) *Service {
	return &Service{store: store, loc: loc, currency: currency, rates: rates, now: time.Now}
}

func (s *Service) today() string { return s.now().In(s.loc).Format(dayLayout) }

// tomorrow is the first moment of the next local day; anything from then on is in the future.
func (s *Service) tomorrow() time.Time {
	n := s.now().In(s.loc)
	return time.Date(n.Year(), n.Month(), n.Day()+1, 0, 0, 0, 0, s.loc)
}

// monthStart parses "YYYY-MM" as the first moment of that month in the person's time zone.
func (s *Service) monthStart(month string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01", month, s.loc)
	if err != nil {
		return time.Time{}, ValidationError("Invalid month")
	}
	return t, nil
}

func addMonth(t time.Time, n int) time.Time {
	return time.Date(t.Year(), t.Month()+time.Month(n), 1, 0, 0, 0, 0, t.Location())
}

// --- Categories ------------------------------------------------------------------------------------

func (s *Service) Categories(ctx context.Context) ([]Category, error) {
	list, ok, err := s.store.Categories(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return slices.Clone(defaultCategories), nil
	}
	return list, nil
}

func (s *Service) SaveCategories(ctx context.Context, list []Category) ([]Category, error) {
	clean, err := validateCategories(list)
	if err != nil {
		return nil, err
	}
	if err := s.store.SaveCategories(ctx, clean); err != nil {
		return nil, err
	}
	return clean, nil
}

// --- Transactions ----------------------------------------------------------------------------------

func signed(kind transaction.Kind, positive int64) int64 {
	if kind == transaction.KindIncome {
		return positive
	}
	return -positive
}

// kindOf covers transactions from before kinds existed: the sign decides.
func kindOf(t transaction.Transaction) transaction.Kind {
	if t.Kind != "" {
		return t.Kind
	}
	if t.AmountMinor < 0 {
		return transaction.KindExpense
	}
	return transaction.KindIncome
}

func editable(t transaction.Transaction) bool {
	return t.Source == transaction.SourceManual || t.Source == transaction.SourceRecurring
}

// nonNil makes empty lists encode as [] instead of null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// newID returns prefix plus 128 random bits.
func newID(prefix string) string { return prefix + strings.ToLower(rand.Text()) }

// Transactions returns one month's transactions, newest first.
func (s *Service) Transactions(ctx context.Context, month string) ([]transaction.Transaction, error) {
	from, err := s.monthStart(month)
	if err != nil {
		return nil, err
	}
	if err := s.ensureMaterialized(ctx); err != nil {
		return nil, err
	}
	txs, err := s.store.TransactionsBetween(ctx, from, addMonth(from, 1))
	if err != nil {
		return nil, err
	}
	slices.SortFunc(txs, func(a, b transaction.Transaction) int { return b.OccurredAt.Compare(a.OccurredAt) })
	return nonNil(txs), nil
}

// check validates in against the current categories and accounts.
func (s *Service) check(ctx context.Context, in TransactionInput) (txFields, time.Time, error) {
	cats, err := s.Categories(ctx)
	if err != nil {
		return txFields{}, time.Time{}, err
	}
	accounts, err := s.store.Accounts(ctx)
	if err != nil {
		return txFields{}, time.Time{}, err
	}
	return validateTransaction(in, cats, accounts, s.tomorrow())
}

func (s *Service) CreateTransaction(ctx context.Context, in TransactionInput) (transaction.Transaction, error) {
	f, when, err := s.check(ctx, in)
	if err != nil {
		return transaction.Transaction{}, err
	}
	id := newID("m-")
	t, err := s.build(ctx, id, f, when, transaction.SourceManual, "manual-"+id)
	if err != nil {
		return t, err
	}
	return t, s.store.InsertTransaction(ctx, t)
}

// ImportTransaction adds an entry from a bank statement. Its id comes from key (unique per statement
// line), so loading the same statement again adds nothing and fails with ErrDuplicate. It is a manual
// entry, so it can still be recategorized or deleted.
func (s *Service) ImportTransaction(ctx context.Context, in TransactionInput, key string) (transaction.Transaction, error) {
	f, when, err := s.check(ctx, in)
	if err != nil {
		return transaction.Transaction{}, err
	}
	t, err := s.build(ctx, importID(key), f, when, transaction.SourceManual, "import:"+key)
	if err != nil {
		return t, err
	}
	return t, s.store.InsertTransaction(ctx, t)
}

// UpdateTransaction edits a manual or recurring entry. An edited occurrence keeps its dedup key, so
// it stays tied to its rule.
func (s *Service) UpdateTransaction(ctx context.Context, id string, in TransactionInput) (transaction.Transaction, error) {
	old, err := s.store.Transaction(ctx, id)
	if err != nil {
		return transaction.Transaction{}, err
	}
	if !editable(old) {
		return transaction.Transaction{}, ErrReadOnly
	}
	f, when, err := s.check(ctx, in)
	if err != nil {
		return transaction.Transaction{}, err
	}
	t, err := s.build(ctx, id, f, when, old.Source, old.DedupKey)
	if err != nil {
		return t, err
	}
	return t, s.store.ReplaceTransaction(ctx, t)
}

// DeleteTransaction removes a manual or recurring entry. A deleted occurrence is never re-created,
// because its rule only creates days after Rule.Through.
func (s *Service) DeleteTransaction(ctx context.Context, id string) error {
	old, err := s.store.Transaction(ctx, id)
	if err != nil {
		return err
	}
	if !editable(old) {
		return ErrReadOnly
	}
	return s.store.DeleteTransaction(ctx, id)
}

// build turns checked fields into a transaction, converting the amount at the day's rate into the
// summary currency and into each account's currency.
func (s *Service) build(ctx context.Context, id string, f txFields, when time.Time, source transaction.Source, dedup string) (transaction.Transaction, error) {
	t := toTransaction(id, f.entryFields, when, source, dedup)
	if f.refund {
		t.AmountMinor, t.Refund = f.amount, true
	}
	day := when.In(s.loc).Format(dayLayout)
	var err error
	if t.BaseMinor, err = s.convert(ctx, day, t.AmountMinor, t.Currency, s.currency); err != nil {
		return t, err
	}
	if f.from != nil {
		t.Account = f.from.ID
		if t.AccountMinor, err = s.convert(ctx, day, t.AmountMinor, t.Currency, f.from.Currency); err != nil {
			return t, err
		}
	}
	if f.to != nil {
		t.ToAccount, t.ToMinor = f.to.ID, f.toAmount
		if t.ToMinor == 0 {
			if t.ToMinor, err = s.convert(ctx, day, f.amount, t.Currency, f.to.Currency); err != nil {
				return t, err
			}
		}
	}
	return t, nil
}

func toTransaction(id string, f entryFields, when time.Time, source transaction.Source, dedup string) transaction.Transaction {
	return transaction.Transaction{
		ID:          id,
		Kind:        f.kind,
		OccurredAt:  when,
		AmountMinor: signed(f.kind, f.amount),
		Currency:    f.currency,
		Merchant:    f.merchant,
		Category:    f.category,
		Note:        f.note,
		Source:      source,
		DedupKey:    dedup,
	}
}

// --- Recurring rules -------------------------------------------------------------------------------

func (s *Service) Rules(ctx context.Context) ([]Rule, error) {
	if err := s.ensureMaterialized(ctx); err != nil {
		return nil, err
	}
	rules, err := s.store.Rules(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(rules, func(a, b Rule) int {
		return cmp.Compare(strings.ToLower(a.Merchant), strings.ToLower(b.Merchant))
	})
	return nonNil(rules), nil
}

// CreateRule saves a rule and creates its occurrences up to today, including past ones.
func (s *Service) CreateRule(ctx context.Context, in RuleInput) (Rule, error) {
	cats, err := s.Categories(ctx)
	if err != nil {
		return Rule{}, err
	}
	clean, err := validateRule(in, cats, s.today())
	if err != nil {
		return Rule{}, err
	}
	// Saved before its occurrences: if creating them fails halfway, the next run finishes the job
	// instead of leaving entries without a rule.
	r := Rule{ID: newID("r-"), RuleInput: clean}
	if err := s.store.PutRule(ctx, r); err != nil {
		return Rule{}, err
	}
	return r, s.materialize(ctx, &r, s.today())
}

// UpdateRule changes a rule from now on. Entries already created stay as they are.
func (s *Service) UpdateRule(ctx context.Context, id string, in RuleInput) (Rule, error) {
	r, err := s.store.Rule(ctx, id)
	if err != nil {
		return Rule{}, err
	}
	cats, err := s.Categories(ctx)
	if err != nil {
		return Rule{}, err
	}
	clean, err := validateRule(in, cats, s.today())
	if err != nil {
		return Rule{}, err
	}
	r.RuleInput = clean
	if err := s.store.PutRule(ctx, r); err != nil {
		return Rule{}, err
	}
	return r, s.materialize(ctx, &r, s.today())
}

// DeleteRule stops a rule. Entries already created stay.
func (s *Service) DeleteRule(ctx context.Context, id string) error {
	if _, err := s.store.Rule(ctx, id); err != nil {
		return err
	}
	return s.store.DeleteRule(ctx, id)
}

// ensureMaterialized creates the due occurrences of every rule, at most once per day per process.
// Every read goes through it, so no scheduled job is needed: the entries appear when the dashboard
// asks for them. Running it twice is harmless; dedup keys stop duplicates.
func (s *Service) ensureMaterialized(ctx context.Context) error {
	today := s.today()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.materializedOn == today {
		return nil
	}
	rules, err := s.store.Rules(ctx)
	if err != nil {
		return err
	}
	for i := range rules {
		if err := s.materialize(ctx, &rules[i], today); err != nil {
			return err
		}
	}
	s.materializedOn = today
	return nil
}

// materialize creates r's occurrences after r.Through up to today, then moves Through to today.
func (s *Service) materialize(ctx context.Context, r *Rule, today string) error {
	if r.Through >= today {
		return nil
	}
	for _, day := range occurrences(r.RuleInput, r.Through, today) {
		d, _ := parseDay(day)
		t := transaction.Transaction{
			ID:   fmt.Sprintf("o-%s-%s", r.ID, day),
			Kind: r.Kind,
			// Noon local time: no time zone shift can move it to a neighbouring day.
			OccurredAt:  time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, s.loc).UTC(),
			AmountMinor: signed(r.Kind, r.AmountMinor),
			Currency:    r.Currency,
			Merchant:    r.Merchant,
			Category:    r.Category,
			Note:        r.Note,
			Source:      transaction.SourceRecurring,
			DedupKey:    fmt.Sprintf("recurring:%s:%s", r.ID, day),
		}
		var err error
		if t.BaseMinor, err = s.convert(ctx, day, t.AmountMinor, t.Currency, s.currency); err != nil {
			return err
		}
		if err := s.store.InsertTransaction(ctx, t); err != nil && !errors.Is(err, ErrDuplicate) {
			return err
		}
	}
	r.Through = today
	return s.store.PutRule(ctx, *r)
}

// --- Summary ---------------------------------------------------------------------------------------

type totals struct {
	spent, income, invested int64
	byCategory              []CategoryTotal
}

// total adds up txs in the summary currency base. Expenses in a category that no longer exists count
// as the catch-all; refunds lower spending; transfers are neither spending nor income.
func total(txs []transaction.Transaction, expenseIDs map[string]bool, base string) totals {
	var t totals
	by := map[string]*CategoryTotal{}
	for _, tx := range txs {
		amount := baseAmount(tx, base)
		switch kindOf(tx) {
		case transaction.KindTransfer:
			continue
		case transaction.KindIncome:
			t.income += amount
		case transaction.KindInvestment:
			t.invested -= amount
		default:
			spent := -amount
			t.spent += spent
			id := tx.Category
			if !expenseIDs[id] {
				id = catchAll[transaction.KindExpense]
			}
			c := by[id]
			if c == nil {
				c = &CategoryTotal{Category: id}
				by[id] = c
			}
			c.SpentMinor += spent
			c.Count++
		}
	}
	t.byCategory = make([]CategoryTotal, 0, len(by))
	for _, c := range by {
		t.byCategory = append(t.byCategory, *c)
	}
	slices.SortFunc(t.byCategory, func(a, b CategoryTotal) int {
		return cmp.Or(cmp.Compare(b.SpentMinor, a.SpentMinor), cmp.Compare(a.Category, b.Category))
	})
	return t
}

// Summary returns month's totals and the 12 months ending at it.
func (s *Service) Summary(ctx context.Context, month string) (Summary, error) {
	end, err := s.monthStart(month)
	if err != nil {
		return Summary{}, err
	}
	if err := s.ensureMaterialized(ctx); err != nil {
		return Summary{}, err
	}
	cats, err := s.Categories(ctx)
	if err != nil {
		return Summary{}, err
	}
	expenseIDs := map[string]bool{}
	for _, c := range cats {
		if c.Kind == transaction.KindExpense {
			expenseIDs[c.ID] = true
		}
	}

	first := addMonth(end, -11)
	txs, err := s.store.TransactionsBetween(ctx, first, addMonth(end, 1))
	if err != nil {
		return Summary{}, err
	}
	byMonth := map[string][]transaction.Transaction{}
	for _, tx := range txs {
		m := tx.OccurredAt.In(s.loc).Format("2006-01")
		byMonth[m] = append(byMonth[m], tx)
	}

	out := Summary{Month: month, Currency: s.currency, Months: make([]MonthTotal, 0, 12)}
	for m := first; !m.After(end); m = addMonth(m, 1) {
		key := m.Format("2006-01")
		t := total(byMonth[key], expenseIDs, s.currency)
		out.Months = append(out.Months, MonthTotal{key, t.spent, t.income, t.invested, t.byCategory})
		if key == month {
			out.SpentMinor, out.IncomeMinor, out.InvestedMinor = t.spent, t.income, t.invested
			out.Count = len(byMonth[key])
			out.ByCategory = t.byCategory
		}
	}
	return out, nil
}
