package main

// Account erasure (card openpak/prds#120): DELETE /internal/accounts/{account_id}/data on the
// health listener (container network only, never routed by Traefik) with
// "Authorization: Bearer $ACCOUNT_ERASE_KEY". The account's Switch links come from the account
// core (namespace "switch"; a subject is the BAAS user id or the PID as hex), and every
// pvz_accounts row keyed by one of those BAAS ids or carrying one of those PIDs is deleted.

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	accountv1 "github.com/openpak/account-api/openpak/account/v1"
)

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var errNoCore = errors.New("account core not configured (ACCOUNT_GRPC_ADDR, ACCOUNT_INTERNAL_KEY)")

// switchLinks lists an account's Switch link subjects.
type switchLinks interface {
	SwitchSubjects(ctx context.Context, account string) ([]string, error)
}

type grpcLinks struct{ links accountv1.LinksClient }

// newLinksFromEnv dials the account core, or returns nil when it is not configured.
func newLinksFromEnv() switchLinks {
	addr, key := os.Getenv("ACCOUNT_GRPC_ADDR"), os.Getenv("ACCOUNT_INTERNAL_KEY")
	if addr == "" || key == "" {
		return nil
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any,
			cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			ctx, cancel := context.WithTimeout(metadata.AppendToOutgoingContext(ctx, "x-internal-key", key), 5*time.Second)
			defer cancel()
			return invoker(ctx, method, req, reply, cc, opts...)
		}))
	if err != nil {
		slog.Error("account core", "err", err.Error())
		return nil
	}
	return &grpcLinks{accountv1.NewLinksClient(conn)}
}

func (c *grpcLinks) SwitchSubjects(ctx context.Context, account string) ([]string, error) {
	r, err := c.links.ListAccountLinks(ctx, &accountv1.ListAccountLinksRequest{AccountId: account, Namespace: "switch"})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, l := range r.GetLinks() {
		if s := strings.TrimSpace(l.GetSubjectId()); s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

// erase resolves the account's Switch ids through the core and deletes their rows.
func (s *server) erase(ctx context.Context, account string) (int, error) {
	if s.links == nil {
		return 0, errNoCore
	}
	subjects, err := s.links.SwitchSubjects(ctx, account)
	if err != nil {
		return 0, err
	}
	if len(subjects) == 0 {
		return 0, nil
	}
	var pids []int64
	for _, sub := range subjects {
		if n, err := strconv.ParseUint(sub, 16, 64); err == nil && n != 0 {
			pids = append(pids, int64(n))
		}
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	return s.store.back.erase(ctx, subjects, pids)
}

func (p *pgBackend) erase(ctx context.Context, baas []string, pids []int64) (int, error) {
	if pids == nil {
		pids = []int64{}
	}
	t, err := p.pool.Exec(ctx, `DELETE FROM pvz_accounts WHERE baas_user_id = ANY($1) OR pid = ANY($2)`, baas, pids)
	if err != nil {
		return 0, err
	}
	return int(t.RowsAffected()), nil
}

// eraseHandler serves the erase route. The key is read per request; unset means 503, never open.
func eraseHandler(service string, erase func(ctx context.Context, account string) (int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := os.Getenv("ACCOUNT_ERASE_KEY")
		if key == "" {
			http.Error(w, "erase not configured", http.StatusServiceUnavailable)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, "Bearer ")), []byte(key)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/internal/accounts/"), "/data")
		if !uuidRE.MatchString(id) {
			http.Error(w, "bad account id", http.StatusBadRequest)
			return
		}
		id = strings.ToLower(id)
		n, err := erase(r.Context(), id)
		if err != nil {
			slog.Error("account erase failed", "service", service, "err", err.Error())
			http.Error(w, "erase failed", http.StatusInternalServerError)
			return
		}
		slog.Info("account erased", "account", id, "rows", n, "service", service)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"service\":%q,\"rows\":%d}\n", service, n)
	}
}

// logRequests logs every request on the internal listener (method and path, no headers or key).
func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
		h.ServeHTTP(w, r)
	})
}
