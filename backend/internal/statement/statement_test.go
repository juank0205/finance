package statement

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// latin1 encodes s (which must only use Latin-1 characters) the way Société Générale exports it.
func latin1(s string) []byte {
	var b []byte
	for _, r := range s {
		b = append(b, byte(r))
	}
	return b
}

const sgFile = "00012345678;22/05/2026;22/11/2026;6;20/11/2026;1234.56 EUR\r\n" +
	"\r\n" +
	"Date de l'opération;Libellé;Détail de l'écriture;Montant de l'opération;Devise\r\n" +
	"20/11/2026;CARTE X1111 16/11 ;CARTE X1111 16/11 Urban Soccer COMMERCE ELECTRONIQUE 110632400738093IOPD;-7,50;EUR\r\n" +
	"20/11/2026;VIR INST RE 682489;VIR INST RE 682489359092 DE: M. JOHN DOE SMITH DATE: 20/11/2026 00:44 MOTIF: Uber;12,50;EUR\r\n" +
	"18/11/2026;CARTE X1111 RETRAIT DAB 18/11;CARTE X1111 RETRAIT DAB 18/11 12H30 SAINT ETIENNE 00012345;-40,00;EUR\r\n" +
	"04/11/2026;000001 VIR PERM;000001 VIR PERM POUR: S.A.R.L. Example Homes 04 11 SG 01842 MOTIF: Loyer;-1 395,00;EUR\r\n" +
	"03/11/2026;CARTE X1111 03/11 ;CARTE X1111 03/11 UBER * EATS PENDING;-12,78;EUR\r\n" +
	"03/11/2026;CARTE X1111 03/11 ;CARTE X1111 03/11 UBER * EATS PENDING;-12,78;EUR\r\n" +
	"02/11/2026;CARTE X1111 02/11 ;CARTE X1111 02/11 SHOP USA;-3,00;USD\r\n"

func TestSG(t *testing.T) {
	st, err := Parse(latin1(sgFile))
	if err != nil {
		t.Fatal(err)
	}
	if st.Format != SG || st.Skipped != 1 || len(st.Lines) != 6 {
		t.Fatalf("format %s, skipped %d, %d lines", st.Format, st.Skipped, len(st.Lines))
	}
	want := []Line{
		{"sg:2026-11-20:CARTE X1111 16/11 Urban Soccer COMMERCE ELECTRONIQUE 110632400738093IOPD:-750:1", "2026-11-16", -750, "EUR",
			"CARTE X1111 16/11 Urban Soccer COMMERCE ELECTRONIQUE 110632400738093IOPD", "URBAN SOCCER"},
		{"sg:2026-11-20:VIR INST RE 682489359092 DE: M. JOHN DOE SMITH DATE: 20/11/2026 00:44 MOTIF: Uber:1250:1", "2026-11-20", 1250, "EUR",
			"VIR INST RE 682489359092 DE: M. JOHN DOE SMITH DATE: 20/11/2026 00:44 MOTIF: Uber", "M. JOHN DOE SMITH"},
		{"sg:2026-11-18:CARTE X1111 RETRAIT DAB 18/11 12H30 SAINT ETIENNE 00012345:-4000:1", "2026-11-18", -4000, "EUR",
			"CARTE X1111 RETRAIT DAB 18/11 12H30 SAINT ETIENNE 00012345", "RETRAIT DAB"},
		{"sg:2026-11-04:000001 VIR PERM POUR: S.A.R.L. Example Homes 04 11 SG 01842 MOTIF: Loyer:-139500:1", "2026-11-04", -139500, "EUR",
			"000001 VIR PERM POUR: S.A.R.L. Example Homes 04 11 SG 01842 MOTIF: Loyer", "S.A.R.L. EXAMPLE HOMES"},
		{"sg:2026-11-03:CARTE X1111 03/11 UBER * EATS PENDING:-1278:1", "2026-11-03", -1278, "EUR",
			"CARTE X1111 03/11 UBER * EATS PENDING", "UBER * EATS PENDING"},
		{"sg:2026-11-03:CARTE X1111 03/11 UBER * EATS PENDING:-1278:2", "2026-11-03", -1278, "EUR",
			"CARTE X1111 03/11 UBER * EATS PENDING", "UBER * EATS PENDING"},
	}
	for i, w := range want {
		if st.Lines[i] != w {
			t.Errorf("line %d:\n got %+v\nwant %+v", i, st.Lines[i], w)
		}
	}
}

func TestSGCardYearBoundary(t *testing.T) {
	file := "Date de l'opération;Libellé;Détail de l'écriture;Montant de l'opération;Devise\n" +
		"05/01/2027;CARTE X1111 30/12 ;CARTE X1111 30/12 SHOP;-1,00;EUR\n"
	st, err := Parse([]byte(file))
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Lines[0].Day; got != "2026-12-30" {
		t.Errorf("day %s, want 2026-12-30", got)
	}
}

func TestRevolut(t *testing.T) {
	file := "Type,Product,Started Date,Completed Date,Description,Amount,Fee,Currency,State,Balance\n" +
		"Card Payment,Current,2026-10-07 11:58:43,2026-10-08 12:49:47,Izly,-10.00,0.00,EUR,COMPLETED,0.00\n" +
		"Deposit,Current,2026-10-16 16:53:49,2026-10-16 16:53:51,Google Pay deposit by *1234,20.00,0.50,EUR,COMPLETED,25.50\n" +
		"Card Payment,Current,2026-10-17 09:00:00,,Cafe,-3.00,0.00,EUR,PENDING,\n"
	st, err := Parse([]byte(file))
	if err != nil {
		t.Fatal(err)
	}
	if st.Format != Revolut || st.Skipped != 1 || len(st.Lines) != 3 {
		t.Fatalf("format %s, skipped %d, %+v", st.Format, st.Skipped, st.Lines)
	}
	want := []Line{
		{"revolut:2026-10-07 11:58:43:Izly:-1000:1", "2026-10-07", -1000, "EUR", "Izly", "IZLY"},
		{"revolut:2026-10-16 16:53:49:Google Pay deposit by *1234:2000:1", "2026-10-16", 2000, "EUR", "Google Pay deposit by *1234", "GOOGLE PAY DEPOSIT BY"},
		{"revolut:2026-10-16 16:53:49:Google Pay deposit by *1234:2000:1:fee", "2026-10-16", -50, "EUR", "Fee: Google Pay deposit by *1234", "REVOLUT FEE"},
	}
	for i, w := range want {
		if st.Lines[i] != w {
			t.Errorf("line %d:\n got %+v\nwant %+v", i, st.Lines[i], w)
		}
	}
}

// xlsxFile builds a one-sheet workbook the way Bancolombia's export does: strings shared, numbers
// inline.
func xlsxFile(t *testing.T, rows [][]string) []byte {
	t.Helper()
	var shared []string
	var sheet strings.Builder
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for r, row := range rows {
		fmt.Fprintf(&sheet, `<row r="%d">`, r+1)
		for c, v := range row {
			ref := fmt.Sprintf("%c%d", 'A'+c, r+1)
			if _, err := fmt.Sscan(v, new(float64)); err == nil {
				fmt.Fprintf(&sheet, `<c r="%s"><v>%s</v></c>`, ref, v)
			} else {
				fmt.Fprintf(&sheet, `<c r="%s" t="s"><v>%d</v></c>`, ref, len(shared))
				shared = append(shared, v)
			}
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	var sst strings.Builder
	sst.WriteString(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	for _, s := range shared {
		fmt.Fprintf(&sst, `<si><t>%s</t></si>`, s)
	}
	sst.WriteString(`</sst>`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Movimientos" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/movs.xml"/></Relationships>`,
		"xl/worksheets/movs.xml": sheet.String(),
		"xl/sharedStrings.xml":   sst.String(),
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestBancolombia(t *testing.T) {
	data := xlsxFile(t, [][]string{
		{"Movimientos"},
		{"Fecha", "Descripción", "Referencia", "Valor"},
		{"46346.95833333333", "PAGO SUC VIRT TC VISA", "", "-2353559"},
		{"46346.95833333333", "TRANSF DE DLOCAL COLOMBIA SAS", "null", "4060158"},
		{"46316.916666666664", "ABONO INTERESES AHORROS", "", "1.62"},
	})
	st, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []Line{
		// 23:00 UTC on Nov 20 is midnight of Nov 21 in Europe/Paris.
		{"bancolombia:46346.95833333333:PAGO SUC VIRT TC VISA:-2353559:1", "2026-11-21", -235355900, "COP", "PAGO SUC VIRT TC VISA", "PAGO SUC VIRT TC VISA"},
		{"bancolombia:46346.95833333333:TRANSF DE DLOCAL COLOMBIA SAS:4060158:1", "2026-11-21", 406015800, "COP", "TRANSF DE DLOCAL COLOMBIA SAS", "TRANSF DE DLOCAL COLOMBIA SAS"},
		// 22:00 UTC before the clocks changed on Oct 25.
		{"bancolombia:46316.916666666664:ABONO INTERESES AHORROS:1.62:1", "2026-10-22", 162, "COP", "ABONO INTERESES AHORROS", "ABONO INTERESES AHORROS"},
	}
	if st.Format != Bancolombia || len(st.Lines) != len(want) {
		t.Fatalf("format %s, %+v", st.Format, st.Lines)
	}
	for i, w := range want {
		if st.Lines[i] != w {
			t.Errorf("line %d:\n got %+v\nwant %+v", i, st.Lines[i], w)
		}
	}
}

func TestRejects(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":       nil,
		"random text": []byte("hello,world\n1,2\n"),
		"broken zip":  []byte("PK\x03\x04garbage"),
	} {
		if _, err := Parse(data); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
	// A zip that is not a Bancolombia export.
	other := xlsxFile(t, [][]string{{"Name", "Age"}, {"Ann", "3"}})
	if _, err := Parse(other); !errors.Is(err, ErrUnknownFormat) {
		t.Errorf("other workbook: %v", err)
	}
}

func TestZipBomb(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("xl/worksheets/sheet1.xml")
	chunk := bytes.Repeat([]byte(" "), 1<<20)
	for range 40 { // 40 MB of spaces compresses to a few KB
		w.Write(chunk)
	}
	zw.Close()
	if buf.Len() > 1<<20 {
		t.Fatalf("bomb is %d bytes", buf.Len())
	}
	if _, err := Parse(buf.Bytes()); err == nil {
		t.Error("parsed a zip bomb")
	}
}

func TestTooManyLines(t *testing.T) {
	var b strings.Builder
	b.WriteString("Type,Product,Started Date,Completed Date,Description,Amount,Fee,Currency,State,Balance\n")
	for i := range MaxLines + 1 {
		fmt.Fprintf(&b, "Card Payment,Current,2026-10-07 11:58:%02d,,Shop %d,-1.00,0.00,EUR,COMPLETED,0\n", i%60, i)
	}
	if _, err := Parse([]byte(b.String())); err == nil {
		t.Error("parsed more than MaxLines")
	}
}

func TestDescriptorOf(t *testing.T) {
	for key, want := range map[string]string{
		"sg:2026-11-20:CARTE X1111 16/11 Urban Soccer COMMERCE ELECTRONIQUE 1106:-750:1":           "sg|URBAN SOCCER",
		"sg:2026-11-20:VIR INST RE 6824 DE: M. JOHN DOE DATE: 20/11/2026 00:44 MOTIF: Uber:1250:1": "sg|M. JOHN DOE",
		"revolut:2026-10-29 09:44:29:Transfer from JOHN DOE:1750:20":                               "revolut|TRANSFER FROM JOHN DOE",
		"revolut:2026-10-16 16:53:49:Google Pay deposit by *1234:2000:3:fee":                       "revolut|REVOLUT FEE",
		"bancolombia:46325.95833333333:ABONO INTERESES AHORROS:1.62:14":                            "bancolombia|ABONO INTERESES AHORROS",
		"bancolombia:46346.95833333333:TRANSFERENCIAS A NEQUI:-32500:1":                            "bancolombia|TRANSFERENCIAS A NEQUI",
	} {
		f, d, ok := DescriptorOf(key)
		if got := string(f) + "|" + d; !ok || got != want {
			t.Errorf("%s: got %q %v, want %q", key, got, ok, want)
		}
	}
	for _, key := range []string{"", "manual-x", "recurring:r1:2026-10-01", "revolut:short:1:2", "sg:2026-11-20"} {
		if _, _, ok := DescriptorOf(key); ok {
			t.Errorf("%q: ok", key)
		}
	}
	// A parsed line and its key agree.
	st, err := Parse(latin1(sgFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range st.Lines {
		if _, d, _ := DescriptorOf(l.Key); d != l.Descriptor {
			t.Errorf("%s: key gives %q, line has %q", l.Key, d, l.Descriptor)
		}
	}
}
