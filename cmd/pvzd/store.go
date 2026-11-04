package main

// Accounts live in Postgres (PVZ_DATABASE_URL, required; pool of 3): a restart loses nothing. A leftover
// accounts.json in the data dir is imported once and renamed accounts.json.imported-20261104
// (openpak/prds#85).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// account is one OpenPak player as this game knows them. BlazeID doubles as the persona id and
// the Nucleus account id: the game only needs them stable and unique.
type account struct {
	BaasUserID string            `json:"baas_user_id"`
	PID        uint64            `json:"pid"`
	BlazeID    int64             `json:"blaze_id"`
	Name       string            `json:"name"`
	Created    time.Time         `json:"created"`
	LastLogin  time.Time         `json:"last_login"`
	Data       map[string]string `json:"data,omitempty"` // per-game blobs (ByteVault, settings), by key
}

// backend is where accounts are kept: Postgres in production, a map in handler tests.
type backend interface {
	// login creates the account on first sight and refreshes pid, name and last login.
	login(ctx context.Context, baasID string, pid uint64, name string) (a account, first bool, err error)
	saveData(ctx context.Context, baasID string, data map[string]string) error
	byBlaze(ctx context.Context, id int64) (*account, error)
}

type store struct {
	mu   sync.Mutex
	back backend
}

const schema = `CREATE SEQUENCE IF NOT EXISTS pvz_blaze_id START 1000000001;
CREATE TABLE IF NOT EXISTS pvz_accounts (
	baas_user_id text PRIMARY KEY,
	pid          bigint NOT NULL,
	blaze_id     bigint NOT NULL UNIQUE,
	name         text NOT NULL,
	created      timestamptz NOT NULL,
	last_login   timestamptz NOT NULL,
	data         jsonb NOT NULL DEFAULT '{}'
)`

type pgBackend struct{ pool *pgxpool.Pool }

// openStore connects Postgres (retrying while it comes up), creates the schema and imports a legacy
// accounts.json found in dir.
func openStore(ctx context.Context, url, dir string) (*store, error) {
	if url == "" {
		return nil, errors.New("PVZ_DATABASE_URL is required: accounts are kept in Postgres")
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("PVZ_DATABASE_URL: invalid")
	}
	cfg.MaxConns = 3
	var pool *pgxpool.Pool
	for i := 0; ; i++ {
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				break
			}
			pool.Close()
		}
		if i >= 30 {
			return nil, fmt.Errorf("database unreachable: %w", err)
		}
		time.Sleep(2 * time.Second)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	b := &pgBackend{pool}
	if n, err := b.importFile(ctx, filepath.Join(dir, "accounts.json")); err != nil {
		pool.Close()
		return nil, fmt.Errorf("import accounts.json: %w", err)
	} else if n > 0 {
		log.Printf("store: imported %d accounts from accounts.json", n)
	}
	return &store{back: b}, nil
}

// importFile loads a legacy accounts.json once: rows already in the table win, the blaze id sequence
// moves past the file's counter, and the file is renamed so it is not read again.
func (p *pgBackend) importFile(ctx context.Context, path string) (int, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var f struct {
		Next int64               `json:"next"`
		By   map[string]*account `json:"accounts"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	n := 0
	maxID := f.Next
	for id, a := range f.By {
		if a.BlazeID >= maxID {
			maxID = a.BlazeID + 1
		}
		data, _ := json.Marshal(a.Data)
		if a.Data == nil {
			data = []byte("{}")
		}
		tag, err := tx.Exec(ctx, `INSERT INTO pvz_accounts (baas_user_id, pid, blaze_id, name, created, last_login, data)
			VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb) ON CONFLICT DO NOTHING`,
			id, int64(a.PID), a.BlazeID, a.Name, a.Created, a.LastLogin, string(data))
		if err != nil {
			return 0, err
		}
		n += int(tag.RowsAffected())
	}
	if maxID > 0 {
		if _, err := tx.Exec(ctx, `SELECT setval('pvz_blaze_id', GREATEST($1::bigint - 1, COALESCE((SELECT max(blaze_id) FROM pvz_accounts), 0), 1000000000), true)`, maxID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return n, os.Rename(path, path+".imported-20261104")
}

const cols = `baas_user_id, pid, blaze_id, name, created, last_login, data`

func scanAccount(row pgx.Row) (account, error) {
	var a account
	var pid int64
	var data []byte
	if err := row.Scan(&a.BaasUserID, &pid, &a.BlazeID, &a.Name, &a.Created, &a.LastLogin, &data); err != nil {
		return a, err
	}
	a.PID = uint64(pid)
	if err := json.Unmarshal(data, &a.Data); err != nil {
		return a, err
	}
	if len(a.Data) == 0 {
		a.Data = nil
	}
	return a, nil
}

func (p *pgBackend) login(ctx context.Context, baasID string, pid uint64, name string) (account, bool, error) {
	var a account
	var pidv int64
	var data []byte
	var first bool
	err := p.pool.QueryRow(ctx, `WITH n AS (SELECT nextval('pvz_blaze_id') AS id)
		INSERT INTO pvz_accounts (baas_user_id, pid, blaze_id, name, created, last_login)
		SELECT $1, $2, n.id, COALESCE(NULLIF($3,''), 'Player' || (n.id % 100000)), now(), now() FROM n
		ON CONFLICT (baas_user_id) DO UPDATE SET pid = EXCLUDED.pid, last_login = now(),
			name = CASE WHEN $3 <> '' THEN $3 ELSE pvz_accounts.name END
		RETURNING `+cols+`, (xmax = 0)`, baasID, int64(pid), name).
		Scan(&a.BaasUserID, &pidv, &a.BlazeID, &a.Name, &a.Created, &a.LastLogin, &data, &first)
	if err != nil {
		return a, false, err
	}
	a.PID = uint64(pidv)
	_ = json.Unmarshal(data, &a.Data)
	if len(a.Data) == 0 {
		a.Data = nil
	}
	return a, first, nil
}

func (p *pgBackend) saveData(ctx context.Context, baasID string, data map[string]string) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if data == nil {
		raw = []byte("{}")
	}
	_, err = p.pool.Exec(ctx, `UPDATE pvz_accounts SET data = $2::jsonb WHERE baas_user_id = $1`, baasID, string(raw))
	return err
}

func (p *pgBackend) byBlaze(ctx context.Context, id int64) (*account, error) {
	a, err := scanAccount(p.pool.QueryRow(ctx, `SELECT `+cols+` FROM pvz_accounts WHERE blaze_id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func dbCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// get returns (creating if needed) the account for an OpenPak player.
func (s *store) get(baasID string, pid uint64, name string) (*account, bool, error) {
	ctx, cancel := dbCtx()
	defer cancel()
	a, first, err := s.back.login(ctx, baasID, pid, name)
	if err != nil {
		return nil, false, err
	}
	return &a, first, nil
}

// update runs fn on an account under the lock and writes its data back.
func (s *store) update(a *account, fn func(a *account)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(a)
	ctx, cancel := dbCtx()
	defer cancel()
	return s.back.saveData(ctx, a.BaasUserID, a.Data)
}

func (s *store) byBlazeID(id int64) *account {
	ctx, cancel := dbCtx()
	defer cancel()
	a, err := s.back.byBlaze(ctx, id)
	if err != nil {
		log.Printf("store: lookup failed: %v", err)
		return nil
	}
	return a
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }
