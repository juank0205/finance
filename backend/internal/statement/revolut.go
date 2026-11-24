package statement

import (
	"encoding/csv"
	"fmt"
	"strings"
)

// Revolut CSV (English export):
//
//	Type,Product,Started Date,Completed Date,Description,Amount,Fee,Currency,State,Balance
//	Card Payment,Current,2026-10-07 11:58:43,2026-10-08 12:49:47,Izly,-10.00,0.00,EUR,COMPLETED,0.00
//
// Only COMPLETED rows count. A fee becomes its own line, keyed like its row plus ":fee".

func parseRevolut(text string) (Statement, error) {
	r := csv.NewReader(strings.NewReader(text))
	r.FieldsPerRecord = -1
	recs, err := r.ReadAll()
	if err != nil {
		return Statement{}, fmt.Errorf("Revolut file: %w", err)
	}
	st := Statement{Format: Revolut}
	keys := keyer{}
	for _, rec := range recs[1:] {
		if len(rec) < 9 || strings.TrimSpace(strings.Join(rec, "")) == "" {
			continue
		}
		started, desc, currency, state := rec[2], collapse(rec[4]), strings.TrimSpace(rec[7]), rec[8]
		if state != "COMPLETED" {
			st.Skipped++
			continue
		}
		if len(started) < 10 {
			return Statement{}, fmt.Errorf("Revolut file: bad date %q", started)
		}
		amount, err := parseDecimal(rec[5], '.')
		if err != nil {
			return Statement{}, fmt.Errorf("Revolut file: bad amount %q", rec[5])
		}
		fee, err := parseDecimal(rec[6], '.')
		if err != nil {
			return Statement{}, fmt.Errorf("Revolut file: bad fee %q", rec[6])
		}
		day := started[:10]
		key := keys.key(fmt.Sprintf("revolut:%s:%s:%d", started, desc, amount))
		st.Lines = append(st.Lines, Line{
			Key: key, Day: day, AmountMinor: amount, Currency: currency,
			Description: desc, Descriptor: descriptor(desc),
		})
		if fee > 0 {
			st.Lines = append(st.Lines, Line{
				Key: key + ":fee", Day: day, AmountMinor: -fee, Currency: currency,
				Description: "Fee: " + desc, Descriptor: "REVOLUT FEE",
			})
		}
	}
	return checkSize(st)
}
