package statement

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
)

// A minimal XLSX reader: the first worksheet's cells as text, nothing else. XLSX is a zip of XML
// parts, so the standard library is enough and no spreadsheet dependency is needed.
//
// The file comes from the network, so the reader caps what it inflates: a small zip that expands to
// gigabytes (a zip bomb) fails instead of exhausting the Lambda's memory.

const (
	maxPartBytes = 16 << 20 // one XML part, uncompressed
	maxZipFiles  = 200
	maxColumns   = 64
)

func readFirstSheet(data []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("not a valid XLSX file")
	}
	if len(zr.File) > maxZipFiles {
		return nil, errors.New("not a valid XLSX file")
	}
	parts := map[string]*zip.File{}
	for _, f := range zr.File {
		parts[f.Name] = f
	}
	sheet, err := firstSheetPath(parts)
	if err != nil {
		return nil, err
	}
	var shared []string
	if f, ok := parts["xl/sharedStrings.xml"]; ok {
		if shared, err = readSharedStrings(f); err != nil {
			return nil, err
		}
	}
	f, ok := parts[sheet]
	if !ok {
		return nil, errors.New("the workbook has no worksheet")
	}
	return readSheet(f, shared)
}

// firstSheetPath follows workbook.xml to the first sheet's part, falling back to sheet1.xml.
func firstSheetPath(parts map[string]*zip.File) (string, error) {
	const fallback = "xl/worksheets/sheet1.xml"
	wb, ok1 := parts["xl/workbook.xml"]
	rels, ok2 := parts["xl/_rels/workbook.xml.rels"]
	if !ok1 || !ok2 {
		return fallback, nil
	}
	var book struct {
		Sheets []struct {
			RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	var links struct {
		Rels []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := decodePart(wb, &book); err != nil {
		return "", err
	}
	if err := decodePart(rels, &links); err != nil {
		return "", err
	}
	if len(book.Sheets) == 0 {
		return "", errors.New("the workbook has no worksheet")
	}
	for _, r := range links.Rels {
		if r.ID == book.Sheets[0].RID {
			if strings.HasPrefix(r.Target, "/") {
				return strings.TrimPrefix(r.Target, "/"), nil
			}
			return path.Join("xl", r.Target), nil
		}
	}
	return fallback, nil
}

func open(f *zip.File) (io.ReadCloser, error) {
	if f.UncompressedSize64 > maxPartBytes {
		return nil, errors.New("the file is too large")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, errors.New("not a valid XLSX file")
	}
	// The declared size can lie; the limit holds whatever the zip claims.
	return struct {
		io.Reader
		io.Closer
	}{io.LimitReader(rc, maxPartBytes), rc}, nil
}

func decodePart(f *zip.File, v any) error {
	rc, err := open(f)
	if err != nil {
		return err
	}
	defer rc.Close()
	if err := xml.NewDecoder(rc).Decode(v); err != nil {
		return errors.New("not a valid XLSX file")
	}
	return nil
}

// richText is a shared or inline string: plain <t>, or runs of <r><t>.
type richText struct {
	T    string   `xml:"t"`
	Runs []string `xml:"r>t"`
}

func (s richText) String() string { return s.T + strings.Join(s.Runs, "") }

func readSharedStrings(f *zip.File) ([]string, error) {
	var sst struct {
		Items []richText `xml:"si"`
	}
	if err := decodePart(f, &sst); err != nil {
		return nil, err
	}
	out := make([]string, len(sst.Items))
	for i, it := range sst.Items {
		out[i] = it.String()
	}
	return out, nil
}

func readSheet(f *zip.File, shared []string) ([][]string, error) {
	rc, err := open(f)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	var rows [][]string
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, errors.New("not a valid XLSX file")
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "row" {
			continue
		}
		var row struct {
			Cells []struct {
				Ref    string   `xml:"r,attr"`
				Type   string   `xml:"t,attr"`
				Value  string   `xml:"v"`
				Inline richText `xml:"is"`
			} `xml:"c"`
		}
		if err := dec.DecodeElement(&row, &start); err != nil {
			return nil, errors.New("not a valid XLSX file")
		}
		if len(rows) >= MaxLines+50 {
			return nil, fmt.Errorf("The statement has more than %d lines; export a shorter period", MaxLines)
		}
		var cells []string
		for i, c := range row.Cells {
			col := i
			if c.Ref != "" {
				if col = columnIndex(c.Ref); col < 0 || col >= maxColumns {
					continue
				}
			}
			text := c.Value
			switch c.Type {
			case "s":
				n, err := strconv.Atoi(c.Value)
				if err != nil || n < 0 || n >= len(shared) {
					return nil, errors.New("not a valid XLSX file")
				}
				text = shared[n]
			case "inlineStr":
				text = c.Inline.String()
			}
			for len(cells) <= col {
				cells = append(cells, "")
			}
			cells[col] = text
		}
		rows = append(rows, cells)
	}
}

// columnIndex turns a cell reference such as "C12" into its zero-based column, or -1.
func columnIndex(ref string) int {
	n := 0
	for i, r := range ref {
		switch {
		case r >= 'A' && r <= 'Z':
			n = n*26 + int(r-'A'+1)
			if n > maxColumns {
				return -1
			}
		case i == 0:
			return -1
		default:
			return n - 1
		}
	}
	return n - 1
}
