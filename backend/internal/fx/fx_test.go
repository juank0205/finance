package fx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testClient(t *testing.T, files map[string]string) (*Client, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, ok := files[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.urls = []string{srv.URL + "/%s"}
	c.today = func() string { return "2026-11-22" }
	return c, &calls
}

func TestRate(t *testing.T) {
	c, calls := testClient(t, map[string]string{
		"2026-10-01": `{"date":"2026-10-01","eur":{"usd":1.25,"cop":5000}}`,
		"2026-11-21": `{"date":"2026-11-21","eur":{"usd":1.1,"cop":4400}}`,
	})
	ctx := context.Background()
	cases := []struct {
		day, from, to string
		want          float64
	}{
		{"2026-10-01", "EUR", "USD", 1.25},
		{"2026-10-01", "USD", "EUR", 0.8},
		{"2026-10-01", "USD", "COP", 4000},
		{"2026-10-03", "EUR", "COP", 5000}, // no file for 10-03 or 10-02: falls back to 10-01
		{"2026-11-22", "EUR", "USD", 1.1},  // today not published yet
		{"2026-12-25", "EUR", "USD", 1.1},  // future: latest
		{"2026-10-01", "EUR", "EUR", 1},
	}
	for _, tc := range cases {
		got, err := c.Rate(ctx, tc.day, tc.from, tc.to)
		if err != nil || got != tc.want {
			t.Errorf("Rate(%s, %s, %s) = %v, %v; want %v", tc.day, tc.from, tc.to, got, err, tc.want)
		}
	}
	before := *calls
	if _, err := c.Rate(ctx, "2026-10-01", "EUR", "USD"); err != nil || *calls != before {
		t.Errorf("second lookup of a day should be cached: calls %d -> %d, err %v", before, *calls, err)
	}
	if _, err := c.Rate(ctx, "2026-10-01", "EUR", "XYZ"); err == nil {
		t.Error("unknown currency: want error")
	}
}

func TestRateRejectsImplausibleFile(t *testing.T) {
	c, _ := testClient(t, map[string]string{"2026-11-22": `{"eur":{"usd":110,"cop":4400}}`})
	if _, err := c.Rate(context.Background(), "2026-11-22", "EUR", "COP"); err == nil {
		t.Error("want error for EUR/USD = 110")
	}
}
