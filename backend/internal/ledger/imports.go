package ledger

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"

	"finance/internal/statement"
	"finance/internal/transaction"
)

// Statement import in two steps. PreviewImport parses a statement and suggests an entry for every
// line; the dashboard shows them, the person adjusts what they want, and Import books the chosen
// lines. The file itself is never stored.
//
// Suggestions come from earlier imports: a line whose counterparty (statement.Line.Descriptor) was
// booked before gets the same type, merchant, category and, for transfers, the same other account.

const (
	maxImportFile  = 2 << 20 // bytes, before base64
	maxImportBatch = 200     // lines per Import call
	// duplicateDays is how far apart a statement line and an existing entry with the same amount can
	// be and still be taken for the same movement (card payments post a few days late).
	duplicateDays = 3
)

func importID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "i-" + hex.EncodeToString(sum[:13])
}

// PreviewImport parses a base64 statement for account.
func (s *Service) PreviewImport(ctx context.Context, in ImportPreviewInput) (ImportPreview, error) {
	accounts, err := s.store.Accounts(ctx)
	if err != nil {
		return ImportPreview{}, err
	}
	acct := findAccount(accounts, in.Account)
	if acct == nil {
		return ImportPreview{}, ValidationError("Pick the account this statement is for")
	}
	if base64.StdEncoding.DecodedLen(len(in.File)) > maxImportFile {
		return ImportPreview{}, ValidationError("The file is larger than 2 MB")
	}
	data, err := base64.StdEncoding.DecodeString(in.File)
	if err != nil {
		return ImportPreview{}, ValidationError("Invalid file")
	}
	st, err := statement.Parse(data)
	if err != nil {
		return ImportPreview{}, ValidationError(err.Error())
	}
	cats, err := s.Categories(ctx)
	if err != nil {
		return ImportPreview{}, err
	}
	history, err := s.store.TransactionsBetween(ctx, time.Unix(0, 0), s.tomorrow())
	if err != nil {
		return ImportPreview{}, err
	}

	out := ImportPreview{Format: string(st.Format), Account: acct.ID, Skipped: st.Skipped, Lines: []ImportLine{}}
	learned := learn(history, acct.ID)
	byID := map[string]transaction.Transaction{}
	for _, t := range history {
		byID[t.ID] = t
	}
	used := map[string]bool{} // existing entries already matched to a line
	descriptors := make([]string, len(st.Lines))
	for i, l := range st.Lines {
		descriptors[i] = l.Descriptor
		line := ImportLine{
			Key: l.Key, Day: l.Day, AmountMinor: l.AmountMinor, Currency: l.Currency,
			Description: l.Description, Status: ImportNew,
		}
		if t, ok := byID[importID(l.Key)]; ok {
			line.Status, line.Match = ImportDone, &t
			used[t.ID] = true
		}
		s.suggest(&line, learned[string(st.Format)+"|"+l.Descriptor], acct.ID, l.Descriptor, cats)
		out.Lines = append(out.Lines, line)
	}
	// Second pass, so an exact match always wins over a look-alike.
	for i := range out.Lines {
		line := &out.Lines[i]
		switch {
		case line.Status == ImportDone:
		case line.Day < acct.OpeningDate:
			line.Status = ImportBeforeOpening
		case line.Currency == acct.Currency:
			if t, ok := s.lookAlike(*line, acct.ID, history, used); ok {
				line.Status, line.Match = ImportDuplicate, &t
				used[t.ID] = true
				// The first import keyed lines by row number, so its keys differ from today's; an
				// entry it booked from the same counterparty is this line.
				if key, ok := strings.CutPrefix(t.DedupKey, "import:"); ok {
					if f, d, ok := statement.DescriptorOf(key); ok && string(f) == out.Format && d == descriptors[i] {
						line.Status = ImportDone
					}
				}
			}
		}
	}
	return out, nil
}

// learn maps "<format>|<descriptor>" to the latest entry imported into account from that
// counterparty.
func learn(history []transaction.Transaction, account string) map[string]transaction.Transaction {
	out := map[string]transaction.Transaction{}
	for _, t := range history {
		key, ok := strings.CutPrefix(t.DedupKey, "import:")
		if !ok || (t.Account != account && t.ToAccount != account) {
			continue
		}
		format, desc, ok := statement.DescriptorOf(key)
		if !ok || desc == "" {
			continue
		}
		k := string(format) + "|" + desc
		if prev, ok := out[k]; !ok || t.OccurredAt.After(prev.OccurredAt) {
			out[k] = t
		}
	}
	return out
}

// suggest fills the entry fields of line, from prev (the latest entry from the same counterparty)
// when its type fits the line's sign.
func (s *Service) suggest(line *ImportLine, prev transaction.Transaction, account, desc string, cats []Category) {
	in := line.AmountMinor > 0
	switch kind := kindOf(prev); {
	case prev.ID == "":
	case kind == transaction.KindTransfer:
		// The direction comes from the line's sign when it is booked; only the other side is learned.
		other := prev.Account
		if other == account {
			other = prev.ToAccount
		}
		if other != "" && other != account {
			line.Kind, line.Merchant, line.OtherAccount, line.Learned = kind, prev.Merchant, other, true
			return
		}
	case kind == transaction.KindIncome && in, kind == transaction.KindExpense, kind == transaction.KindInvestment && !in:
		line.Kind, line.Merchant, line.Category, line.Learned = kind, prev.Merchant, prev.Category, true
		return
	}
	line.Merchant = titleCase(cmp.Or(desc, line.Description))
	if in {
		line.Kind, line.Category = transaction.KindIncome, defaultCategory(cats, transaction.KindIncome)
	} else {
		line.Kind, line.Category = transaction.KindExpense, defaultCategory(cats, transaction.KindExpense)
	}
}

// defaultCategory is "Other" for expenses and the income category named like "Other income" when
// there is one; otherwise the first of that kind.
func defaultCategory(cats []Category, kind transaction.Kind) string {
	first := ""
	for _, c := range cats {
		if c.Kind != kind {
			continue
		}
		if first == "" {
			first = c.ID
		}
		if c.ID == "other" || strings.Contains(strings.ToLower(c.Name), "other") {
			return c.ID
		}
	}
	return first
}

// lookAlike finds an entry of account with the line's exact amount within duplicateDays, e.g. a card
// payment already loaded from Google Wallet, or a transfer recorded from the other account's
// statement. The closest day wins.
func (s *Service) lookAlike(line ImportLine, account string, history []transaction.Transaction, used map[string]bool) (transaction.Transaction, bool) {
	day, ok := parseDay(line.Day)
	if !ok {
		return transaction.Transaction{}, false
	}
	var best transaction.Transaction
	bestGap := duplicateDays + 1
	for _, t := range history {
		if used[t.ID] {
			continue
		}
		same := (t.Account == account && t.AccountMinor == line.AmountMinor) ||
			(t.ToAccount == account && t.ToMinor == line.AmountMinor)
		if !same {
			continue
		}
		d, _ := parseDay(t.OccurredAt.In(s.loc).Format(dayLayout))
		gap := int(d.Sub(day).Hours() / 24)
		if gap < 0 {
			gap = -gap
		}
		if gap < bestGap || (gap == bestGap && t.ID < best.ID) {
			best, bestGap = t, gap
		}
	}
	return best, bestGap <= duplicateDays
}

func titleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// Import books the chosen lines into account. A line imported before is counted, not added again.
func (s *Service) Import(ctx context.Context, in ImportInput) (ImportResult, error) {
	if len(in.Lines) == 0 || len(in.Lines) > maxImportBatch {
		return ImportResult{}, ValidationError("Send between 1 and 200 lines at a time")
	}
	cats, err := s.Categories(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	accounts, err := s.store.Accounts(ctx)
	if err != nil {
		return ImportResult{}, err
	}
	acct := findAccount(accounts, in.Account)
	if acct == nil {
		return ImportResult{}, ValidationError("Unknown account")
	}
	// Check and convert every line before booking any, so a bad line never leaves half a batch behind.
	var todo []transaction.Transaction
	for _, e := range in.Lines {
		tin, err := s.importInput(ctx, e, *acct, accounts)
		if err != nil {
			return ImportResult{}, err
		}
		f, when, err := validateTransaction(tin, cats, accounts, s.tomorrow())
		if err != nil {
			return ImportResult{}, ValidationError(e.Merchant + ": " + err.Error())
		}
		t, err := s.build(ctx, importID(e.Key), f, when, transaction.SourceManual, "import:"+e.Key)
		if err != nil {
			return ImportResult{}, err
		}
		todo = append(todo, t)
	}
	var res ImportResult
	for _, t := range todo {
		switch err := s.store.InsertTransaction(ctx, t); {
		case errors.Is(err, ErrDuplicate):
			res.AlreadyThere++
		case err != nil:
			return res, err
		default:
			res.Added++
		}
	}
	return res, nil
}

// importInput turns a confirmed line into a transaction input. The amount's sign comes from the
// statement: money in on an expense makes it a refund, and a transfer's direction follows the sign.
func (s *Service) importInput(ctx context.Context, e ImportEntry, acct Account, accounts []Account) (TransactionInput, error) {
	if !validImportKey(e.Key) {
		return TransactionInput{}, ValidationError("Invalid line key")
	}
	day, ok := parseDay(e.Day)
	if !ok {
		return TransactionInput{}, ValidationError("Invalid date")
	}
	if e.Day < acct.OpeningDate {
		return TransactionInput{}, ValidationError("A line is dated before the account's opening date")
	}
	if e.AmountMinor == 0 {
		return TransactionInput{}, ValidationError("A line has no amount")
	}
	amount := e.AmountMinor
	in := amount > 0
	if !in {
		amount = -amount
	}
	noon := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, s.loc)
	out := TransactionInput{
		Kind: e.Kind, AmountMinor: amount, Currency: e.Currency, OccurredAt: noon.Format(time.RFC3339),
		Merchant: e.Merchant, Category: e.Category, Note: e.Note, Account: acct.ID,
	}
	switch e.Kind {
	case transaction.KindExpense:
		out.Refund = in
	case transaction.KindIncome:
		if !in {
			return out, ValidationError(e.Merchant + ": income must be money coming in")
		}
	case transaction.KindInvestment:
		if in {
			return out, ValidationError(e.Merchant + ": an investment must be money going out")
		}
	case transaction.KindTransfer:
		other := findAccount(accounts, e.OtherAccount)
		if other == nil {
			return out, ValidationError(e.Merchant + ": pick the other account of the transfer")
		}
		out.Category = ""
		if !in {
			out.ToAccount = other.ID
			break
		}
		// Money arrived from other: the transfer leaves other, in its currency.
		left, err := s.convert(ctx, e.Day, amount, e.Currency, other.Currency)
		if err != nil {
			return out, err
		}
		out.Account, out.ToAccount = other.ID, acct.ID
		out.AmountMinor, out.Currency, out.ToAmountMinor = max(left, 1), other.Currency, amount
	}
	if e.Kind != transaction.KindTransfer && e.OtherAccount != "" {
		return out, ValidationError("Only transfers have another account")
	}
	return out, nil
}

func validImportKey(key string) bool {
	format, _, ok := strings.Cut(key, ":")
	if !ok || len(key) > 600 || strings.ContainsFunc(key, unicode.IsControl) {
		return false
	}
	return slices.Contains([]statement.Format{statement.SG, statement.Revolut, statement.Bancolombia}, statement.Format(format))
}
