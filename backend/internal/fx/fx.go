// Package fx gives daily reference exchange rates from the free, keyless currency API at
// github.com/fawazahmed0/exchange-api (served by jsDelivr, with Cloudflare Pages as a fallback). It
// covers USD, COP and USDC, which the ECB's reference rates do not all have, and keeps every past day.
//
// Rates only change numbers shown in the dashboard, never who can see them, so a wrong rate is a
// display problem; still, each answer is checked for sane values and the response size is capped.
package fx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// maxBody caps a day's file (all currencies against EUR, ~10 KB today).
const maxBody = 1 << 20

// lookback is how many earlier days are tried when a day has no file yet (today before it is
// published) or never got one.
const lookback = 7

// Client fetches and caches rates. The zero value is not usable; call New.
type Client struct {
	http  *http.Client
	urls  []string // fmt patterns taking the day (YYYY-MM-DD)
	today func() string

	mu    sync.Mutex
	cache map[string]map[string]float64 // day -> lower-case code -> units per 1 EUR
}

func New() *Client {
	return &Client{
		http: &http.Client{Timeout: 3 * time.Second}, // two hosts stay under Lambda's 10 s timeout
		urls: []string{
			"https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@%s/v1/currencies/eur.json",
			"https://%s.currency-api.pages.dev/v1/currencies/eur.json",
		},
		today: func() string { return time.Now().UTC().Format(time.DateOnly) },
		cache: map[string]map[string]float64{},
	}
}

// ErrNoRate means no rate could be found for a currency.
var ErrNoRate = errors.New("no exchange rate")

// Rate returns how many units of to one unit of from was worth on day (YYYY-MM-DD). A day without
// published rates uses the closest earlier one; a future day uses the latest.
func (c *Client) Rate(ctx context.Context, day, from, to string) (float64, error) {
	if from == to {
		return 1, nil
	}
	rates, err := c.day(ctx, day)
	if err != nil {
		return 0, err
	}
	f, t := rateOf(rates, from), rateOf(rates, to)
	if f == 0 || t == 0 {
		return 0, fmt.Errorf("%w for %s/%s", ErrNoRate, from, to)
	}
	return t / f, nil
}

func rateOf(rates map[string]float64, code string) float64 {
	if code == "EUR" {
		return 1
	}
	return rates[strings.ToLower(code)]
}

func (c *Client) day(ctx context.Context, day string) (map[string]float64, error) {
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return nil, err
	}
	if today := c.today(); day > today {
		d, _ = time.Parse(time.DateOnly, today)
	}
	var lastErr error
	for i := 0; i <= lookback; i++ {
		key := d.AddDate(0, 0, -i).Format(time.DateOnly)
		c.mu.Lock()
		rates, ok := c.cache[key]
		c.mu.Unlock()
		if ok {
			return rates, nil
		}
		rates, err := c.fetch(ctx, key)
		if err == nil {
			c.mu.Lock()
			c.cache[key] = rates
			c.mu.Unlock()
			return rates, nil
		}
		lastErr = err
		// Only a missing file means an earlier day may help; a host that is down stays down.
		if !errors.Is(err, errMissing) {
			break
		}
	}
	return nil, fmt.Errorf("exchange rates for %s: %w", day, lastErr)
}

var errMissing = errors.New("not published")

// fetch tries each host for day and returns the first good answer. It reports errMissing only when
// every host said the day does not exist.
func (c *Client) fetch(ctx context.Context, day string) (map[string]float64, error) {
	var lastErr error
	missing := true
	for _, pattern := range c.urls {
		rates, err := c.get(ctx, fmt.Sprintf(pattern, day))
		if err == nil {
			return rates, nil
		}
		lastErr = err
		missing = missing && errors.Is(err, errMissing)
	}
	if missing {
		return nil, errMissing
	}
	return nil, lastErr
}

func (c *Client) get(ctx context.Context, url string) (map[string]float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, errMissing
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", url, res.StatusCode)
	}
	var body struct {
		EUR map[string]float64 `json:"eur"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(&body); err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	rates := make(map[string]float64, len(body.EUR))
	for code, r := range body.EUR {
		if r > 0 && !math.IsInf(r, 0) && !math.IsNaN(r) {
			rates[code] = r
		}
	}
	// USD always trades between 0.5 and 2 per euro; anything else is a broken or tampered file.
	if usd := rates["usd"]; usd < 0.5 || usd > 2 {
		return nil, fmt.Errorf("%s: implausible EUR/USD rate %v", url, usd)
	}
	return rates, nil
}
