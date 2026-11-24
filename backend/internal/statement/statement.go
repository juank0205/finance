// Package statement reads bank statement exports into lines the ledger can import. It knows the
// Société Générale CSV, the Revolut CSV and the Bancolombia XLSX; Parse tells them apart by content.
//
// Every line has a key that is unique within the file and stable across exports of overlapping
// periods, so loading a statement twice, or two statements that overlap, adds each line once.
package statement

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Format names a statement layout. It is also the first part of every key.
type Format string

const (
	SG          Format = "sg"
	Revolut     Format = "revolut"
	Bancolombia Format = "bancolombia"
)

// MaxLines caps a statement; a year of a busy account is well under it.
const MaxLines = 2000

// Line is one movement on the statement.
type Line struct {
	Key         string // "<format>:..."; unique within the file
	Day         string // YYYY-MM-DD the money moved (for card payments, the day of the purchase)
	AmountMinor int64  // signed: negative = money out of the account
	Currency    string // ISO 4217
	Description string // the bank's text, whitespace collapsed
	// Descriptor names the counterparty without dates, amounts or reference numbers, e.g.
	// "URBAN SOCCER". Lines with the same descriptor are the same kind of movement.
	Descriptor string
}

// Statement is a parsed file.
type Statement struct {
	Format Format
	Lines  []Line
	// Skipped counts rows that are not completed movements (pending Revolut payments, other
	// currencies on an SG export).
	Skipped int
}

// ErrUnknownFormat means the file is none of the supported exports.
var ErrUnknownFormat = errors.New("Unrecognized file: upload a Société Générale CSV, a Revolut CSV or a Bancolombia XLSX")

// Parse detects the format of data and reads it.
func Parse(data []byte) (Statement, error) {
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return parseBancolombia(data)
	}
	text := decodeText(data)
	switch {
	case strings.Contains(text, "Type,Product,Started Date,Completed Date,Description,Amount"):
		return parseRevolut(text)
	case strings.Contains(text, "Libellé;Détail de l'écriture;Montant de l'opération"):
		return parseSG(text)
	}
	return Statement{}, ErrUnknownFormat
}

// decodeText returns data as a string: UTF-8 as is (without a byte order mark), anything else read
// as Latin-1, which is what Société Générale exports.
func decodeText(data []byte) string {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if utf8.Valid(data) {
		return string(data)
	}
	runes := make([]rune, len(data))
	for i, b := range data {
		runes[i] = rune(b)
	}
	return string(runes)
}

// keyer numbers repeated keys, so two identical movements on the same day stay two lines.
type keyer map[string]int

func (k keyer) key(base string) string {
	k[base]++
	return fmt.Sprintf("%s:%d", base, k[base])
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// descriptor upper-cases s and drops every word that contains a digit (dates, references, card
// numbers), so it only names the counterparty.
func descriptor(s string) string {
	var keep []string
	for _, w := range strings.Fields(strings.ToUpper(s)) {
		if !strings.ContainsFunc(w, unicode.IsDigit) {
			keep = append(keep, w)
		}
	}
	return strings.Join(keep, " ")
}

func checkSize(st Statement) (Statement, error) {
	if len(st.Lines) > MaxLines {
		return Statement{}, fmt.Errorf("The statement has more than %d lines; export a shorter period", MaxLines)
	}
	return st, nil
}
