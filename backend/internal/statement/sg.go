package statement

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// Société Générale CSV: Latin-1, ";" separated, a summary line, then
//
//	Date de l'opération;Libellé;Détail de l'écriture;Montant de l'opération;Devise
//	20/11/2026;CARTE X4539 16/11 ;CARTE X4539 16/11 Urban Soccer COMMERCE ELECTRONIQUE 1106...;-7,50;EUR
//
// The key is "sg:<posting day>:<detail>:<amount>:<n>", the same as the first import used.

var (
	sgCardRe     = regexp.MustCompile(`^CARTE X\d{4} (\d{2})/(\d{2}) `)
	sgWithdrawRe = regexp.MustCompile(`^CARTE X\d{4} RETRAIT DAB`)
	// The counterparty of a transfer or direct debit, up to the next labelled field or reference.
	sgPartyRe = regexp.MustCompile(`(?:DE|POUR): (.+?)(?: (?:DATE|MOTIF|REF|ID|MDT|LIB|CHEZ):|\s+\d{2} \d{2} |\s+\d{5,}|$)`)
)

func parseSG(text string) (Statement, error) {
	r := csv.NewReader(strings.NewReader(text))
	r.Comma, r.FieldsPerRecord, r.LazyQuotes = ';', -1, true
	st := Statement{Format: SG}
	keys := keyer{}
	header := false
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Statement{}, fmt.Errorf("Société Générale file: %w", err)
		}
		if !header {
			header = len(rec) >= 5 && strings.HasPrefix(rec[1], "Libell")
			continue
		}
		if len(rec) < 5 || strings.TrimSpace(rec[0]) == "" {
			continue
		}
		posted, err := time.Parse("02/01/2006", strings.TrimSpace(rec[0]))
		if err != nil {
			return Statement{}, fmt.Errorf("Société Générale file: bad date %q", rec[0])
		}
		amount, err := parseDecimal(rec[3], ',')
		if err != nil {
			return Statement{}, fmt.Errorf("Société Générale file: bad amount %q", rec[3])
		}
		currency := strings.TrimSpace(rec[4])
		if currency != "EUR" {
			st.Skipped++
			continue
		}
		detail := collapse(rec[2])
		if detail == "" {
			detail = collapse(rec[1])
		}
		postedDay := posted.Format(time.DateOnly)
		st.Lines = append(st.Lines, Line{
			Key:         keys.key(fmt.Sprintf("sg:%s:%s:%d", postedDay, detail, amount)),
			Day:         sgDay(posted, detail),
			AmountMinor: amount,
			Currency:    currency,
			Description: detail,
			Descriptor:  sgDescriptor(detail),
		})
	}
	if !header {
		return Statement{}, ErrUnknownFormat
	}
	return checkSize(st)
}

// sgDay is the day of a card purchase, which the detail carries as DD/MM and which can be days
// before the posting day; for anything else it is the posting day.
func sgDay(posted time.Time, detail string) string {
	m := sgCardRe.FindStringSubmatch(detail)
	if m == nil {
		return posted.Format(time.DateOnly)
	}
	year := posted.Year()
	if m[2] > posted.Format("01") { // bought in December, posted in January
		year--
	}
	d, err := time.Parse(time.DateOnly, fmt.Sprintf("%d-%s-%s", year, m[2], m[1]))
	if err != nil || d.After(posted) || posted.Sub(d) > 45*24*time.Hour {
		return posted.Format(time.DateOnly)
	}
	return d.Format(time.DateOnly)
}

func sgDescriptor(detail string) string {
	switch {
	case sgWithdrawRe.MatchString(detail):
		return "RETRAIT DAB"
	case sgCardRe.MatchString(detail):
		rest := sgCardRe.ReplaceAllString(detail, "")
		if i := strings.Index(rest, " COMMERCE ELECTRONIQUE"); i >= 0 {
			rest = rest[:i]
		}
		return descriptor(rest)
	}
	if m := sgPartyRe.FindStringSubmatch(detail); m != nil {
		if d := descriptor(m[1]); d != "" {
			return d
		}
	}
	return descriptor(detail)
}
