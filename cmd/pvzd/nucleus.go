package main

// Nucleus (accounts.ea.com) as the game's NucleusSDK uses it to turn the console's Nintendo
// account token into an EA auth code for Blaze login. Requests are logged; the shapes follow the
// public OAuth2 conventions Nucleus uses (connect/auth -> code, connect/token, connect/tokeninfo).

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type grant struct {
	acct    *account
	expires time.Time
}

var (
	grantsMu sync.Mutex
	codes    = map[string]grant{} // auth code -> account
	tokens   = map[string]grant{} // access token -> account
)

func newToken(prefix string) string {
	b := make([]byte, 18)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func putGrant(m map[string]grant, k string, a *account, ttl time.Duration) {
	grantsMu.Lock()
	defer grantsMu.Unlock()
	now := time.Now()
	for k, g := range m {
		if now.After(g.expires) {
			delete(m, k)
		}
	}
	m[k] = grant{acct: a, expires: now.Add(ttl)}
}

func takeGrant(m map[string]grant, k string, consume bool) *account {
	grantsMu.Lock()
	defer grantsMu.Unlock()
	g, ok := m[k]
	if !ok || time.Now().After(g.expires) {
		return nil
	}
	if consume {
		delete(m, k)
	}
	return g.acct
}

// findJWT returns the first value that looks like a JWT (a BAAS id_token) in a request.
func findJWT(vals url.Values, r *http.Request) string {
	for _, vs := range vals {
		for _, v := range vs {
			if strings.HasPrefix(v, "eyJ") && strings.Count(v, ".") == 2 {
				return v
			}
		}
	}
	for _, hv := range r.Header {
		for _, v := range hv {
			v = strings.TrimPrefix(v, "Bearer ")
			if strings.HasPrefix(v, "eyJ") && strings.Count(v, ".") == 2 {
				return v
			}
		}
	}
	return ""
}

// accountFromIDToken resolves a console token to this game's account, creating it on first use.
func (s *server) accountFromIDToken(tok string) (*account, bool, error) {
	p, err := s.ids.byIDToken(tok)
	if err != nil {
		return nil, false, err
	}
	return s.store.get(p.BaasUserID, p.PID, p.Nickname)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *server) nucleus(w http.ResponseWriter, r *http.Request, host string, body []byte) {
	vals := r.URL.Query()
	if ct := r.Header.Get("Content-Type"); strings.Contains(ct, "form") {
		if form, err := url.ParseQuery(string(body)); err == nil {
			for k, v := range form {
				vals[k] = append(vals[k], v...)
			}
		}
	}
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	log.Printf("nucleus %s %s%s params=%v", r.Method, host, r.URL.Path, keys)
	switch r.URL.Path {
	case "/connect/auth":
		tok := findJWT(vals, r)
		if tok == "" {
			log.Printf("nucleus connect/auth: no console token in %v", keys)
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
			return
		}
		a, first, err := s.accountFromIDToken(tok)
		if err != nil {
			log.Printf("nucleus connect/auth: identity: %v", err)
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "login_required"})
			return
		}
		code := newToken("QUOx")
		putGrant(codes, code, a, 10*time.Minute)
		log.Printf("nucleus connect/auth: code for blaze %d (%s), first=%v", a.BlazeID, a.Name, first)
		if ru := vals.Get("redirect_uri"); ru != "" && vals.Get("response_type") != "" {
			sep := "?"
			if strings.Contains(ru, "?") {
				sep = "&"
			}
			w.Header().Set("Location", ru+sep+"code="+url.QueryEscape(code))
			writeJSON(w, http.StatusFound, map[string]any{"code": code})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": code})
	case "/connect/token":
		var a *account
		if c := vals.Get("code"); c != "" {
			a = takeGrant(codes, c, true)
		} else if tok := findJWT(vals, r); tok != "" {
			a, _, _ = s.accountFromIDToken(tok)
		}
		if a == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
		at := newToken("QVQx")
		putGrant(tokens, at, a, 4*time.Hour)
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token": at, "token_type": "Bearer", "expires_in": 14400,
			"refresh_token": newToken("UlQx"), "id_token": "",
		})
	case "/connect/tokeninfo":
		at := vals.Get("access_token")
		if at == "" {
			at = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		}
		a := takeGrant(tokens, at, false)
		if a == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_access_token"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"client_id": vals.Get("client_id"), "scope": "basic.identity offline signin basic.entitlement basic.persona",
			"expires_in": 14400, "pid_id": itoa(a.BlazeID), "pid_type": "NX", "user_id": itoa(a.BlazeID),
			"persona_id": a.BlazeID, "is_underage": false, "stopProcess": "OFF",
		})
	default:
		log.Printf("nucleus unhandled %s %s%s headers=%v body=%q", r.Method, host, r.URL.RequestURI(), r.Header, clip(body, 2000))
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not_found"})
	}
}
