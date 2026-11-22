package ledger

import "time"

// Calendar math for recurring rules. Dates are calendar days ("YYYY-MM-DD"), not instants: a bill due
// on the 1st is due on the 1st whatever the time zone. Mirrors frontend/cloud-dashboard/src/recurrence.ts.

const dayLayout = "2006-01-02"

// maxOccurrences keeps a bad rule from looping forever (weekly for ~19 years).
const maxOccurrences = 1000

// parseDay parses a calendar day; time.Parse rejects impossible dates such as 2026-02-30.
func parseDay(s string) (time.Time, bool) {
	t, err := time.Parse(dayLayout, s)
	return t, err == nil
}

// addMonths returns the date n months after day, keeping its day of month but never past the end of
// a short month (31 Jan + 1 month = 28 or 29 Feb).
func addMonths(day time.Time, n int) time.Time {
	first := time.Date(day.Year(), day.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return time.Date(first.Year(), first.Month(), min(day.Day(), last), 0, 0, 0, 0, time.UTC)
}

// nth returns occurrence i (0 = start date). It is always computed from the start, so month-end
// clamping never drifts (31 Jan, 28 Feb, 31 Mar, not 28 Mar).
func nth(start time.Time, every int, unit Unit, i int) time.Time {
	k := every * i
	switch unit {
	case UnitWeek:
		return start.AddDate(0, 0, 7*k)
	case UnitYear:
		return addMonths(start, 12*k)
	default:
		return addMonths(start, k)
	}
}

// occurrences lists the rule's days in (after, to], as "YYYY-MM-DD". An empty after means from the start.
func occurrences(r RuleInput, after, to string) []string {
	start, ok := parseDay(r.StartDate)
	if !ok {
		return nil
	}
	last := to
	if r.EndDate != "" && r.EndDate < to {
		last = r.EndDate
	}
	var out []string
	for i := range maxOccurrences {
		day := nth(start, r.Every, r.Unit, i).Format(dayLayout)
		if day > last {
			break
		}
		if day > after {
			out = append(out, day)
		}
	}
	return out
}
