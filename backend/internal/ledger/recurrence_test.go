package ledger

import (
	"slices"
	"testing"
)

func TestOccurrences(t *testing.T) {
	tests := []struct {
		name      string
		rule      RuleInput
		after, to string
		want      []string
	}{
		{
			name: "month end clamps without drifting",
			rule: RuleInput{StartDate: "2026-01-31", Every: 1, Unit: UnitMonth},
			to:   "2026-04-30",
			want: []string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"},
		},
		{
			name: "leap day yearly",
			rule: RuleInput{StartDate: "2024-02-29", Every: 1, Unit: UnitYear},
			to:   "2028-03-01",
			want: []string{"2024-02-29", "2025-02-28", "2026-02-28", "2027-02-28", "2028-02-29"},
		},
		{
			name: "every 2 weeks",
			rule: RuleInput{StartDate: "2026-11-02", Every: 2, Unit: UnitWeek},
			to:   "2026-12-01",
			want: []string{"2026-11-02", "2026-11-16", "2026-11-30"},
		},
		{
			name: "end date is inclusive",
			rule: RuleInput{StartDate: "2026-01-15", EndDate: "2026-03-15", Every: 1, Unit: UnitMonth},
			to:   "2026-12-31",
			want: []string{"2026-01-15", "2026-02-15", "2026-03-15"},
		},
		{
			name:  "only days after the last run",
			rule:  RuleInput{StartDate: "2026-01-01", Every: 1, Unit: UnitMonth},
			after: "2026-03-01",
			to:    "2026-05-01",
			want:  []string{"2026-04-01", "2026-05-01"},
		},
		{
			name: "start in the future",
			rule: RuleInput{StartDate: "2027-01-01", Every: 1, Unit: UnitMonth},
			to:   "2026-11-10",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := occurrences(tt.rule, tt.after, tt.to); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOccurrencesIsBounded(t *testing.T) {
	r := RuleInput{StartDate: "1900-01-01", Every: 1, Unit: UnitWeek}
	if n := len(occurrences(r, "", "2100-01-01")); n != maxOccurrences {
		t.Errorf("got %d occurrences, want the cap of %d", n, maxOccurrences)
	}
}
