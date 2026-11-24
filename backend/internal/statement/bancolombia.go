package statement

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Bancolombia "Movimientos" XLSX: one sheet with the columns
//
//	Fecha | Descripción | Referencia | Valor
//
// Fecha is an Excel date serial such as 46346.95833333333. The export writes local midnight of the
// browser that downloaded it as UTC, so the fraction is 23:00 or 22:00 of the day before; rounding
// to the nearest day gives the real one. Valor is signed pesos with a dot decimal.
//
// The key is "bancolombia:<serial>:<description>:<valor>:<n>".

func parseBancolombia(data []byte) (Statement, error) {
	rows, err := readFirstSheet(data)
	if err != nil {
		return Statement{}, fmt.Errorf("Bancolombia file: %w", err)
	}
	st := Statement{Format: Bancolombia}
	keys := keyer{}
	header := false
	for _, row := range rows {
		cell := func(i int) string {
			if i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		if !header {
			header = cell(0) == "Fecha" && strings.HasPrefix(cell(1), "Descripci") && cell(3) == "Valor"
			continue
		}
		if cell(0) == "" && cell(3) == "" {
			continue
		}
		serial, err := strconv.ParseFloat(cell(0), 64)
		if err != nil || serial < 40000 || serial > 80000 {
			return Statement{}, fmt.Errorf("Bancolombia file: bad date %q", cell(0))
		}
		value, err := strconv.ParseFloat(cell(3), 64)
		if err != nil || math.Abs(value) > 1e13 {
			return Statement{}, fmt.Errorf("Bancolombia file: bad amount %q", cell(3))
		}
		desc := collapse(cell(1))
		st.Lines = append(st.Lines, Line{
			Key:         keys.key(fmt.Sprintf("bancolombia:%s:%s:%s", cell(0), desc, cell(3))),
			Day:         excelDay(serial),
			AmountMinor: int64(math.Round(value * 100)),
			Currency:    "COP",
			Description: desc,
			Descriptor:  descriptor(desc),
		})
	}
	if !header {
		return Statement{}, ErrUnknownFormat
	}
	return checkSize(st)
}

// excelDay turns an Excel serial (days since 1899-12-30) into YYYY-MM-DD, rounded to the nearest day.
func excelDay(serial float64) string {
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	return epoch.AddDate(0, 0, int(math.Round(serial))).Format(time.DateOnly)
}

// parseDecimal reads a signed amount with the given decimal mark into minor units (two decimals).
// Spaces, including the non-breaking ones banks use between thousands, are ignored.
func parseDecimal(s string, mark rune) (int64, error) {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == ' ' || r == ' ' || r == ' ':
			return -1
		case r == mark:
			return '.'
		case r == ',' || r == '.': // the other mark separates thousands
			return -1
		}
		return r
	}, s)
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || math.Abs(f) > 1e13 {
		return 0, fmt.Errorf("bad amount %q", s)
	}
	return int64(math.Round(f * 100)), nil
}
