package statement

import "strings"

// DescriptorOf returns the descriptor of a line from its key, so entries imported earlier (whose
// dedup key is "import:" + the line key) can teach how to book new lines from the same counterparty.
// It also reads the keys of the first, one-off import, which ended in the row number instead of a
// repeat count.
func DescriptorOf(key string) (Format, string, bool) {
	format, rest, ok := strings.Cut(key, ":")
	if !ok {
		return "", "", false
	}
	if strings.HasSuffix(rest, ":fee") && Format(format) == Revolut {
		return Revolut, "REVOLUT FEE", true
	}
	// rest is "<when>:<description>:<amount>:<n>"; the description may itself contain colons.
	var when int
	switch Format(format) {
	case SG, Bancolombia:
		when = strings.IndexByte(rest, ':')
	case Revolut:
		when = len("2006-01-02 15:04:05")
		if len(rest) <= when || rest[when] != ':' {
			return "", "", false
		}
	default:
		return "", "", false
	}
	if when < 0 {
		return "", "", false
	}
	desc := rest[when+1:]
	for range 2 { // drop ":<amount>:<n>"
		i := strings.LastIndexByte(desc, ':')
		if i < 0 {
			return "", "", false
		}
		desc = desc[:i]
	}
	if Format(format) == SG {
		return SG, sgDescriptor(desc), true
	}
	return Format(format), descriptor(desc), true
}
