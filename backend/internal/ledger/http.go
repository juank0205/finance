package ledger

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"

	"finance/internal/transaction"
)

// maxBody is far above any real request (a full category list is ~1 KB).
const maxBody = 64 << 10

var monthRe = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

type items[T any] struct {
	Items []T `json:"items"`
}

// NewHandler serves the /v1 API. Errors are {"error": "..."}; the dashboard shows the message.
func NewHandler(s *Service) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/summary", func(w http.ResponseWriter, r *http.Request) {
		if month, ok := monthParam(w, r); ok {
			v, err := s.Summary(r.Context(), month)
			reply(w, http.StatusOK, v, err)
		}
	})

	mux.HandleFunc("GET /v1/transactions", func(w http.ResponseWriter, r *http.Request) {
		if month, ok := monthParam(w, r); ok {
			v, err := s.Transactions(r.Context(), month)
			reply(w, http.StatusOK, items[transaction.Transaction]{v}, err)
		}
	})
	mux.HandleFunc("POST /v1/transactions", func(w http.ResponseWriter, r *http.Request) {
		var in TransactionInput
		if decode(w, r, &in) {
			v, err := s.CreateTransaction(r.Context(), in)
			reply(w, http.StatusCreated, v, err)
		}
	})
	mux.HandleFunc("PUT /v1/transactions/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in TransactionInput
		if decode(w, r, &in) {
			v, err := s.UpdateTransaction(r.Context(), r.PathValue("id"), in)
			reply(w, http.StatusOK, v, err)
		}
	})
	mux.HandleFunc("DELETE /v1/transactions/{id}", func(w http.ResponseWriter, r *http.Request) {
		reply(w, http.StatusNoContent, nil, s.DeleteTransaction(r.Context(), r.PathValue("id")))
	})

	mux.HandleFunc("GET /v1/recurring", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Rules(r.Context())
		reply(w, http.StatusOK, items[Rule]{v}, err)
	})
	mux.HandleFunc("POST /v1/recurring", func(w http.ResponseWriter, r *http.Request) {
		var in RuleInput
		if decode(w, r, &in) {
			v, err := s.CreateRule(r.Context(), in)
			reply(w, http.StatusCreated, v, err)
		}
	})
	mux.HandleFunc("PUT /v1/recurring/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in RuleInput
		if decode(w, r, &in) {
			v, err := s.UpdateRule(r.Context(), r.PathValue("id"), in)
			reply(w, http.StatusOK, v, err)
		}
	})
	mux.HandleFunc("DELETE /v1/recurring/{id}", func(w http.ResponseWriter, r *http.Request) {
		reply(w, http.StatusNoContent, nil, s.DeleteRule(r.Context(), r.PathValue("id")))
	})

	mux.HandleFunc("GET /v1/net-worth", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.NetWorth(r.Context())
		reply(w, http.StatusOK, v, err)
	})

	mux.HandleFunc("GET /v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Accounts(r.Context())
		reply(w, http.StatusOK, items[Account]{v}, err)
	})
	mux.HandleFunc("PUT /v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		var in items[Account]
		if decode(w, r, &in) {
			v, err := s.SaveAccounts(r.Context(), in.Items)
			reply(w, http.StatusOK, items[Account]{v}, err)
		}
	})

	mux.HandleFunc("GET /v1/categories", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.Categories(r.Context())
		reply(w, http.StatusOK, items[Category]{v}, err)
	})
	mux.HandleFunc("PUT /v1/categories", func(w http.ResponseWriter, r *http.Request) {
		var in items[Category]
		if decode(w, r, &in) {
			v, err := s.SaveCategories(r.Context(), in.Items)
			reply(w, http.StatusOK, items[Category]{v}, err)
		}
	})

	return mux
}

func monthParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	m := r.URL.Query().Get("month")
	if !monthRe.MatchString(m) {
		writeError(w, http.StatusBadRequest, "month must be YYYY-MM")
		return "", false
	}
	return m, true
}

// decode reads exactly one JSON object with no unknown fields. Requiring the JSON content type also
// means a plain HTML form on another site can never post here.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil || dec.Decode(&struct{}{}) != io.EOF {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return false
	}
	return true
}

func reply(w http.ResponseWriter, status int, v any, err error) {
	var ve ValidationError
	switch {
	case err == nil:
	case errors.Is(err, ErrReadOnly):
		writeError(w, http.StatusConflict, err.Error())
		return
	case errors.As(err, &ve):
		writeError(w, http.StatusBadRequest, ve.Error())
		return
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "Not found")
		return
	default:
		slog.Error("request failed", "err", err)
		writeError(w, http.StatusInternalServerError, "Something went wrong")
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding response", "err", err)
	}
}
