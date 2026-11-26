// Package auth checks the Cognito access token on every API request.
//
// The dashboard signs in through the Cognito hosted page and sends the access token in the
// X-Finance-Token header. Not in Authorization: CloudFront reaches the Lambda Function URL with
// Origin Access Control, which signs every request with SigV4 and needs that header for itself.
// The check needs no network call per request: the token is an RS256 JWT signed with one of the
// user pool's public keys (its JWKS), fetched once and cached.
//
// The pool allows no self sign-up, so every valid token belongs to the owner of the ledger.
package auth

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// leeway absorbs clock differences between Cognito and Lambda.
const leeway = time.Minute

// refetchEvery limits how often an unknown key id makes the verifier download the JWKS again, so
// tokens with made-up key ids cannot turn every request into a call to Cognito.
const refetchEvery = 5 * time.Minute

// Verifier checks access tokens issued by one Cognito user pool for one app client.
type Verifier struct {
	issuer   string
	clientID string
	jwksURL  string
	client   *http.Client
	now      func() time.Time

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

// NewVerifier returns a verifier for the pool whose issuer is
// https://cognito-idp.<region>.amazonaws.com/<pool id>, accepting tokens of clientID only.
func NewVerifier(issuer, clientID string) *Verifier {
	issuer = strings.TrimSuffix(issuer, "/")
	return &Verifier{
		issuer:   issuer,
		clientID: clientID,
		jwksURL:  issuer + "/.well-known/jwks.json",
		client:   &http.Client{Timeout: 5 * time.Second},
		now:      time.Now,
	}
}

// TokenHeader carries the Cognito access token, the bare JWT.
const TokenHeader = "X-Finance-Token"

// Middleware rejects requests without a valid access token with 401.
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get(TokenHeader)
		if token == "" {
			unauthorized(w, "sign in required")
			return
		}
		if err := v.Verify(r.Context(), token); err != nil {
			slog.Info("rejected token", "err", err)
			unauthorized(w, "sign in required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

type header struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

type claims struct {
	Iss      string `json:"iss"`
	TokenUse string `json:"token_use"`
	ClientID string `json:"client_id"`
	Exp      int64  `json:"exp"`
	Iat      int64  `json:"iat"`
}

// Verify returns nil when token is a current access token of this pool and client.
func (v *Verifier) Verify(ctx context.Context, token string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("not a JWT")
	}
	var h header
	if err := decodePart(parts[0], &h); err != nil {
		return fmt.Errorf("header: %w", err)
	}
	// Only RS256: never let the token pick "none" or an HMAC algorithm.
	if h.Alg != "RS256" {
		return fmt.Errorf("algorithm %q not allowed", h.Alg)
	}
	key, err := v.key(ctx, h.Kid)
	if err != nil {
		return err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fmt.Errorf("signature: %w", err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig); err != nil {
		return errors.New("bad signature")
	}

	// Claims are read only after the signature holds.
	var c claims
	if err := decodePart(parts[1], &c); err != nil {
		return fmt.Errorf("claims: %w", err)
	}
	now := v.now()
	switch {
	case c.Iss != v.issuer:
		return fmt.Errorf("issuer %q", c.Iss)
	case c.TokenUse != "access":
		return fmt.Errorf("token_use %q", c.TokenUse)
	case c.ClientID != v.clientID:
		return fmt.Errorf("client_id %q", c.ClientID)
	case c.Exp == 0 || now.After(time.Unix(c.Exp, 0).Add(leeway)):
		return errors.New("expired")
	case c.Iat != 0 && time.Unix(c.Iat, 0).After(now.Add(leeway)):
		return errors.New("issued in the future")
	}
	return nil
}

func decodePart(s string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.NewDecoder(bytes.NewReader(b)).Decode(v)
}

// key returns the public key with id kid, downloading the JWKS on first use and again (rate
// limited) when Cognito has rotated to a key the cache does not know.
func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	if v.keys != nil && v.now().Sub(v.fetched) < refetchEvery {
		return nil, fmt.Errorf("unknown key id %q", kid)
	}
	keys, err := v.fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching JWKS: %w", err)
	}
	v.keys, v.fetched = keys, v.now()
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown key id %q", kid)
}

func (v *Verifier) fetch(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, res.Body, 64<<10)).Decode(&set); err != nil {
		return nil, err
	}
	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil || len(e) == 0 || len(e) > 4 {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return nil, errors.New("no RSA signing keys")
	}
	return keys, nil
}
