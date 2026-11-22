package ledger

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"finance/internal/transaction"
)

// maxAccounts keeps the net worth card readable.
const maxAccounts = 12

func validateAccounts(list, old []Account) ([]Account, error) {
	if len(list) > maxAccounts {
		return nil, ValidationError(fmt.Sprintf("At most %d accounts", maxAccounts))
	}
	ids := map[string]bool{}
	names := map[string]bool{}
	out := make([]Account, 0, len(list))
	for _, a := range list {
		if !categoryID.MatchString(a.ID) || ids[a.ID] {
			return nil, ValidationError("Every account needs its own id")
		}
		name := strings.TrimSpace(a.Name)
		if name == "" || utf8.RuneCountInString(name) > 30 {
			return nil, ValidationError("Every account needs a name of up to 30 characters")
		}
		if names[strings.ToLower(name)] {
			return nil, ValidationError("Duplicate name: " + name)
		}
		if a.Kind != AccountCash && a.Kind != AccountCredit {
			return nil, ValidationError("Unknown account type")
		}
		if !currencyRe.MatchString(a.Currency) {
			return nil, ValidationError("Unknown currency")
		}
		// Entries keep their amount in the account's currency, so changing it would garble them.
		if i := slices.IndexFunc(old, func(o Account) bool { return o.ID == a.ID }); i >= 0 && old[i].Currency != a.Currency {
			return nil, ValidationError("An account's currency cannot change; add a new account instead")
		}
		if a.OpeningMinor < -maxAmountMinor || a.OpeningMinor > maxAmountMinor {
			return nil, ValidationError("Opening balance is too large")
		}
		if _, ok := parseDay(a.OpeningDate); !ok {
			return nil, ValidationError("Invalid opening date")
		}
		ids[a.ID], names[strings.ToLower(name)] = true, true
		a.Name = name
		out = append(out, a)
	}
	return out, nil
}

func (s *Service) Accounts(ctx context.Context) ([]Account, error) {
	list, err := s.store.Accounts(ctx)
	return nonNil(list), err
}

// SaveAccounts replaces the account list. Entries tied to a removed account stay, but no longer
// count toward any balance.
func (s *Service) SaveAccounts(ctx context.Context, list []Account) ([]Account, error) {
	old, err := s.store.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	clean, err := validateAccounts(list, old)
	if err != nil {
		return nil, err
	}
	return clean, s.store.SaveAccounts(ctx, clean)
}

func findAccount(list []Account, id string) *Account {
	if i := slices.IndexFunc(list, func(a Account) bool { return a.ID == id }); i >= 0 {
		return &list[i]
	}
	return nil
}

// convert turns minor units of from into to at day's rate. Every supported currency has 2 decimals.
func (s *Service) convert(ctx context.Context, day string, minor int64, from, to string) (int64, error) {
	if from == to || minor == 0 {
		return minor, nil
	}
	if s.rates == nil {
		return 0, ValidationError("Only " + s.currency + " amounts work without exchange rates")
	}
	r, err := s.rates.Rate(ctx, day, from, to)
	if err != nil {
		slog.Warn("exchange rate", "day", day, "from", from, "to", to, "err", err)
		return 0, ValidationError(fmt.Sprintf("No %s to %s exchange rate right now; try again later", from, to))
	}
	return int64(math.Round(float64(minor) * r)), nil
}

// baseAmount is t's amount in the summary currency. Entries from before currencies were converted
// are all in it.
func baseAmount(t transaction.Transaction, base string) int64 {
	if t.BaseMinor == 0 && t.Currency == base {
		return t.AmountMinor
	}
	return t.BaseMinor
}

// accountAmount is what t moved in its account, in the account's currency.
func accountAmount(t transaction.Transaction, a Account) int64 {
	if t.AccountMinor == 0 && t.Currency == a.Currency {
		return t.AmountMinor
	}
	return t.AccountMinor
}

// NetWorth is every account's balance: its opening balance plus the entries tied to it from the
// opening day on, converted to the summary currency at today's rate.
func (s *Service) NetWorth(ctx context.Context) (NetWorth, error) {
	out := NetWorth{AsOf: s.now().UTC().Format(time.RFC3339), Currency: s.currency, Holdings: []Holding{}}
	accounts, err := s.store.Accounts(ctx)
	if err != nil || len(accounts) == 0 {
		return out, err
	}
	if err := s.ensureMaterialized(ctx); err != nil {
		return out, err
	}

	balance := map[string]int64{}
	earliest := accounts[0].OpeningDate
	for _, a := range accounts {
		balance[a.ID] = a.OpeningMinor
		earliest = min(earliest, a.OpeningDate)
	}
	from, _ := parseDay(earliest)
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, s.loc)
	txs, err := s.store.TransactionsBetween(ctx, from, s.tomorrow())
	if err != nil {
		return out, err
	}
	for _, t := range txs {
		day := t.OccurredAt.In(s.loc).Format(dayLayout)
		if a := findAccount(accounts, t.Account); a != nil && day >= a.OpeningDate {
			balance[a.ID] += accountAmount(t, *a)
		}
		if a := findAccount(accounts, t.ToAccount); a != nil && day >= a.OpeningDate {
			balance[a.ID] += t.ToMinor
		}
	}

	today := s.today()
	for _, a := range accounts {
		h := Holding{ID: a.ID, Name: a.Name, Kind: "cash", Currency: a.Currency, BalanceMinor: balance[a.ID]}
		if a.Kind == AccountCredit {
			h.Kind = "liability"
		}
		if h.BaseMinor, err = s.convert(ctx, today, h.BalanceMinor, a.Currency, s.currency); err != nil {
			return out, err
		}
		out.Holdings = append(out.Holdings, h)
	}
	return out, nil
}
