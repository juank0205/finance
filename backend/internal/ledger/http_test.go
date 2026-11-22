package ledger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestHandler() http.Handler {
	c := &clock{}
	c.set("2026-11-10 12:00")
	return NewHandler(newTestService(NewMemoryStore(), c))
}

func do(t *testing.T, h http.Handler, method, path, contentType, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out map[string]any
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s %s: response is not a JSON object: %q", method, path, rec.Body.String())
		}
	}
	return rec.Code, out
}

const entry = `{"kind":"expense","amount_minor":1250,"currency":"EUR","occurred_at":"2026-11-10T09:00:00Z",` +
	`"merchant":"Fresh Market","category":"groceries"}`

func TestHTTPCreateAndList(t *testing.T) {
	h := newTestHandler()

	code, body := do(t, h, "GET", "/v1/transactions?month=2026-11", "", "")
	if code != 200 || len(body["items"].([]any)) != 0 {
		t.Fatalf("empty month: %d %v", code, body)
	}

	code, created := do(t, h, "POST", "/v1/transactions", "application/json; charset=utf-8", entry)
	if code != http.StatusCreated || created["amount_minor"] != -1250.0 || created["source"] != "manual" {
		t.Fatalf("create: %d %v", code, created)
	}

	_, body = do(t, h, "GET", "/v1/transactions?month=2026-11", "", "")
	if items := body["items"].([]any); len(items) != 1 {
		t.Fatalf("after create: %v", body)
	}

	code, _ = do(t, h, "DELETE", "/v1/transactions/"+created["id"].(string), "", "")
	if code != http.StatusNoContent {
		t.Errorf("delete: %d", code)
	}
}

func TestHTTPErrors(t *testing.T) {
	h := newTestHandler()
	tests := []struct {
		name, method, path, contentType, body string
		want                                  int
	}{
		{"bad month", "GET", "/v1/summary?month=2026-13", "", "", 400},
		{"missing month", "GET", "/v1/transactions", "", "", 400},
		{"form post", "POST", "/v1/transactions", "application/x-www-form-urlencoded", "kind=expense", 415},
		{"unknown field", "POST", "/v1/transactions", "application/json", `{"kind":"expense","admin":true}`, 400},
		{"two objects", "POST", "/v1/transactions", "application/json", entry + entry, 400},
		{"validation", "POST", "/v1/transactions", "application/json", strings.Replace(entry, "1250", "0", 1), 400},
		{"missing entry", "PUT", "/v1/transactions/nope", "application/json", entry, 404},
		{"missing rule", "DELETE", "/v1/recurring/nope", "", "", 404},
		{"too large", "PUT", "/v1/categories", "application/json", `{"items":"` + strings.Repeat("x", maxBody) + `"}`, 400},
	}
	for _, tt := range tests {
		code, body := do(t, h, tt.method, tt.path, tt.contentType, tt.body)
		if code != tt.want {
			t.Errorf("%s: got %d, want %d", tt.name, code, tt.want)
		}
		if msg, _ := body["error"].(string); msg == "" {
			t.Errorf("%s: no error message in %v", tt.name, body)
		}
	}
}

func TestHTTPCategoriesDefault(t *testing.T) {
	code, body := do(t, newTestHandler(), "GET", "/v1/categories", "", "")
	if code != 200 || len(body["items"].([]any)) != len(defaultCategories) {
		t.Errorf("got %d %v", code, body)
	}
}
