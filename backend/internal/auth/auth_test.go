package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const clientID = "dashboard-client"

type pool struct {
	srv     *httptest.Server
	key     *rsa.PrivateKey
	fetches atomic.Int32
}

func newPool(t *testing.T) *pool {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &pool{key: key}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pool/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		p.fetches.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "use": "sig", "alg": "RS256",
			"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *pool) issuer() string { return p.srv.URL + "/pool" }

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func (p *pool) sign(t *testing.T, hdr map[string]string, cl map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(hdr)
	c, _ := json.Marshal(cl)
	signing := b64(h) + "." + b64(c)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, p.key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + b64(sig)
}

func (p *pool) claims(now time.Time) map[string]any {
	return map[string]any{
		"iss": p.issuer(), "token_use": "access", "client_id": clientID,
		"sub": "owner", "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
}

func TestVerify(t *testing.T) {
	p := newPool(t)
	now := time.Now()
	rs256 := map[string]string{"alg": "RS256", "kid": "k1"}
	with := func(k string, v any) map[string]any {
		c := p.claims(now)
		c[k] = v
		return c
	}

	tests := []struct {
		name  string
		token string
		ok    bool
	}{
		{"valid", p.sign(t, rs256, p.claims(now)), true},
		{"id token", p.sign(t, rs256, with("token_use", "id")), false},
		{"other client", p.sign(t, rs256, with("client_id", "someone-else")), false},
		{"other pool", p.sign(t, rs256, with("iss", "https://cognito-idp.eu-west-3.amazonaws.com/other")), false},
		{"expired", p.sign(t, rs256, with("exp", now.Add(-2*time.Minute).Unix())), false},
		{"no exp", p.sign(t, rs256, with("exp", 0)), false},
		{"issued in the future", p.sign(t, rs256, with("iat", now.Add(time.Hour).Unix())), false},
		{"alg none", b64([]byte(`{"alg":"none","kid":"k1"}`)) + "." + b64(mustJSON(p.claims(now))) + ".", false},
		{"alg HS256", p.sign(t, map[string]string{"alg": "HS256", "kid": "k1"}, p.claims(now)), false},
		{"unknown kid", p.sign(t, map[string]string{"alg": "RS256", "kid": "k2"}, p.claims(now)), false},
		{"tampered claims", tamper(p.sign(t, rs256, p.claims(now)), with("client_id", "x")), false},
		{"garbage", "not-a-token", false},
	}
	v := NewVerifier(p.issuer(), clientID)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Verify(context.Background(), tc.token)
			if (err == nil) != tc.ok {
				t.Fatalf("Verify = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// tamper swaps the claims of a signed token, keeping the original signature.
func tamper(token string, cl map[string]any) string {
	parts := strings.Split(token, ".")
	return parts[0] + "." + b64(mustJSON(cl)) + "." + parts[2]
}

func TestUnknownKidDoesNotRefetchEveryTime(t *testing.T) {
	p := newPool(t)
	v := NewVerifier(p.issuer(), clientID)
	bad := p.sign(t, map[string]string{"alg": "RS256", "kid": "nope"}, p.claims(time.Now()))
	for range 5 {
		_ = v.Verify(context.Background(), bad)
	}
	if n := p.fetches.Load(); n != 1 {
		t.Fatalf("JWKS fetched %d times, want 1", n)
	}
}

func TestMiddleware(t *testing.T) {
	p := newPool(t)
	h := NewVerifier(p.issuer(), clientID).Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tc := range []struct {
		name, auth string
		want       int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"basic", "Basic dXNlcjpwYXNz", http.StatusUnauthorized},
		{"bad token", "Bearer x.y.z", http.StatusUnauthorized},
		{"valid", "Bearer " + p.sign(t, map[string]string{"alg": "RS256", "kid": "k1"}, p.claims(time.Now())), http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/summary", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
