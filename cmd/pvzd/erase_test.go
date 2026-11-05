package main

import (
	"context"
	"net/http/httptest"
	"testing"
)

const (
	acctA = "11111111-1111-4111-8111-111111111111"
	acctB = "22222222-2222-4222-8222-222222222222"
)

type fakeLinks map[string][]string

func (f fakeLinks) SwitchSubjects(_ context.Context, a string) ([]string, error) { return f[a], nil }

func TestEraseRemovesOnlyThatAccount(t *testing.T) {
	mem := newMemBackend()
	s := &server{store: &store{back: mem}, links: fakeLinks{acctA: {"baasA", "00000000000000ff"}, acctB: {"baasB"}}}
	ctx := context.Background()
	mem.login(ctx, "baasA", 1, "a")
	mem.login(ctx, "baasC", 255, "c") // same console PID as A's second link
	mem.login(ctx, "baasB", 2, "b")
	mem.login(ctx, "baasD", 3, "d")
	n, err := s.erase(ctx, acctA)
	if err != nil || n != 2 {
		t.Fatalf("erase: %d %v", n, err)
	}
	if len(mem.by) != 2 || mem.by["baasB"] == nil || mem.by["baasD"] == nil {
		t.Fatalf("wrong survivors: %v", mem.by)
	}
	if n, err := s.erase(ctx, acctA); err != nil || n != 0 {
		t.Fatalf("second erase: %d %v", n, err)
	}
	if n, err := s.erase(ctx, "33333333-3333-4333-8333-333333333333"); err != nil || n != 0 {
		t.Fatalf("unknown account: %d %v", n, err)
	}
}

func TestEraseNeedsCore(t *testing.T) {
	if _, err := (&server{}).erase(context.Background(), acctA); err == nil {
		t.Fatal("erase without a core must fail")
	}
}

func TestEraseEndpoint(t *testing.T) {
	h := eraseHandler("plants-vs-zombies", func(context.Context, string) (int, error) { return 2, nil })
	call := func(id, auth string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("DELETE", "/internal/accounts/"+id+"/data", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	t.Setenv("ACCOUNT_ERASE_KEY", "")
	if r := call(acctA, "Bearer k"); r.Code != 503 {
		t.Fatalf("unset key: %d", r.Code)
	}
	t.Setenv("ACCOUNT_ERASE_KEY", "k")
	if r := call(acctA, ""); r.Code != 401 {
		t.Fatalf("no auth: %d", r.Code)
	}
	if r := call(acctA, "Bearer x"); r.Code != 401 {
		t.Fatalf("wrong key: %d", r.Code)
	}
	if r := call("nope", "Bearer k"); r.Code != 400 {
		t.Fatalf("bad id: %d", r.Code)
	}
	r := call(acctA, "Bearer k")
	if r.Code != 200 || r.Body.String() != "{\"service\":\"plants-vs-zombies\",\"rows\":2}\n" {
		t.Fatalf("ok: %d %q", r.Code, r.Body.String())
	}
}
