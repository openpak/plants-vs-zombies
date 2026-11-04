package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Needs a scratch Postgres: PVZ_TEST_DATABASE_URL; skipped otherwise.
func TestAccountsSurviveRestartAndImport(t *testing.T) {
	url := os.Getenv("PVZ_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("PVZ_TEST_DATABASE_URL unset")
	}
	ctx := context.Background()
	dir := t.TempDir()
	legacy := filepath.Join(dir, "accounts.json")
	os.WriteFile(legacy, []byte(`{"next":1000000042,"accounts":{"pt-old":{"baas_user_id":"pt-old","pid":7,"blaze_id":1000000041,"name":"Old","created":"2026-10-25T10:00:00Z","last_login":"2026-10-26T10:00:00Z","data":{"bytevault":"QQ=="}}}}`), 0o600)

	open := func() *store {
		st, err := openStore(ctx, url, dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(st.back.(*pgBackend).pool.Close)
		return st
	}
	a := open()
	a.back.(*pgBackend).pool.Exec(ctx, `DELETE FROM pvz_accounts WHERE baas_user_id LIKE 'pt-%'`)
	// The first open already imported the file; run it again on a fresh table to see the import itself.
	if _, err := os.Stat(legacy + ".imported-20261104"); err != nil {
		t.Fatalf("legacy file not renamed: %v", err)
	}
	os.Rename(legacy+".imported-20261104", legacy)
	a = open()
	old, first, err := a.get("pt-old", 7, "")
	if err != nil || first || old.BlazeID != 1000000041 || old.Name != "Old" || old.Data["bytevault"] != "QQ==" {
		t.Fatalf("imported account: %+v first=%v err=%v", old, first, err)
	}
	n, first, err := a.get("pt-new", 9, "New")
	if err != nil || !first || n.BlazeID < 1000000042 {
		t.Fatalf("new account: %+v first=%v err=%v", n, first, err)
	}
	if err := a.update(n, func(x *account) { x.Data = map[string]string{"bytevault": "Qg=="} }); err != nil {
		t.Fatal(err)
	}
	b := open() // a restart
	again, first, err := b.get("pt-new", 9, "")
	if err != nil || first || again.BlazeID != n.BlazeID || again.Name != "New" || again.Data["bytevault"] != "Qg==" {
		t.Fatalf("after restart: %+v first=%v err=%v", again, first, err)
	}
	if got := b.byBlazeID(n.BlazeID); got == nil || got.BaasUserID != "pt-new" {
		t.Fatalf("byBlazeID: %+v", got)
	}
	b.back.(*pgBackend).pool.Exec(ctx, `DELETE FROM pvz_accounts WHERE baas_user_id LIKE 'pt-%'`)
}
