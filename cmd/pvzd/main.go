// pvzd serves Plants vs. Zombies: Battle for Neighborville (Switch) on OpenPak: EA's gosredirector,
// the game's Blaze server (Fire2 over TLS) and the Nucleus account endpoints the game signs in with.
//
// The game trusts OpenPak's CA for EA hosts only through the owner-approved CA patch
// (tools/pvz-ca), so every TLS leg here is an ordinary OpenPak-CA leaf.
//
// Listeners:
//   - PVZ_TLS_ADDRS (default ":42230,:44325"): TLS, terminated here. Each connection is sniffed:
//     an HTTP request goes to the HTTP handler (redirector, CA service), anything else is a Blaze
//     Fire2 session. The redirector hands out this same listener as the Blaze server.
//   - PVZ_HTTP_ADDR (default ":23541"): plain HTTP for the hosts Traefik terminates
//     (accounts.ea.com, gateway.ea.com, ...), routed by Host.
//   - PVZ_HEALTH_ADDR (default ":23540"): /health.
package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type config struct {
	tlsAddrs   []string
	httpAddr   string
	healthAddr string
	certFile   string
	keyFile    string
	blazeHost  string // host name the redirector hands out (must be in the leaf's SANs)
	blazeIP    string // IPv4 the redirector hands out
	blazePort  int
	dataDir    string
	nxURL      string
	nxKey      string
	trace      bool
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func loadConfig() config {
	port, _ := strconv.Atoi(env("PVZ_BLAZE_PORT", "42230"))
	return config{
		tlsAddrs:   strings.Split(env("PVZ_TLS_ADDRS", ":42230,:44325"), ","),
		httpAddr:   env("PVZ_HTTP_ADDR", ":23541"),
		healthAddr: env("PVZ_HEALTH_ADDR", ":23540"),
		certFile:   env("PVZ_TLS_CERT", "/certs/pvz.pem"),
		keyFile:    env("PVZ_TLS_KEY", "/certs/pvz.key"),
		blazeHost:  env("PVZ_BLAZE_HOST", "spring18.gosredirector.ea.com"),
		blazeIP:    env("PVZ_BLAZE_IP", "145.241.199.19"),
		blazePort:  port,
		dataDir:    env("PVZ_DATA", "/data"),
		nxURL:      env("NX_INTERNAL_URL", "http://127.0.0.1:20070"),
		nxKey:      os.Getenv("NX_INTERNAL_KEY"),
		trace:      env("PVZ_TRACE", "") != "",
	}
}

func main() {
	cfg := loadConfig()
	st, err := openStore(cfg.dataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	blazeHostForConfig, blazePortForConfig = cfg.blazeHost, strconv.Itoa(cfg.blazePort)
	srv := &server{cfg: cfg, store: st, ids: newIdentityClient(cfg.nxURL, cfg.nxKey)}
	cert, err := tls.LoadX509KeyPair(cfg.certFile, cfg.keyFile)
	if err != nil {
		log.Fatalf("tls: %v", err)
	}
	tcfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		GetConfigForClient: func(h *tls.ClientHelloInfo) (*tls.Config, error) {
			log.Printf("tls hello sni=%q from %s versions=%v", h.ServerName, h.Conn.RemoteAddr(), h.SupportedVersions)
			return nil, nil
		},
		// The client offers ECDHE-RSA AES-GCM/ChaCha20 and plain RSA suites; Go's defaults cover them.
	}
	httpHandler := srv.httpMux()
	for _, a := range cfg.tlsAddrs {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		ln, err := net.Listen("tcp", a)
		if err != nil {
			log.Fatalf("listen %s: %v", a, err)
		}
		log.Printf("tls listener %s", a)
		go srv.serveTLS(ln, tcfg, httpHandler)
	}
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok\n")) })
		log.Fatal(http.ListenAndServe(cfg.healthAddr, mux))
	}()
	log.Printf("http listener %s", cfg.httpAddr)
	log.Fatal((&http.Server{Addr: cfg.httpAddr, Handler: httpHandler, ReadHeaderTimeout: 10 * time.Second}).ListenAndServe())
}

type server struct {
	cfg   config
	store *store
	ids   identityClient
	mu    sync.Mutex
	conns int
}

// serveTLS terminates TLS and routes each connection by its first bytes.
func (s *server) serveTLS(ln net.Listener, tcfg *tls.Config, h http.Handler) {
	httpLn := newChanListener(ln.Addr())
	go (&http.Server{Handler: h, ReadHeaderTimeout: 15 * time.Second}).Serve(httpLn)
	for {
		c, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go func() {
			tc := tls.Server(c, tcfg)
			tc.SetDeadline(time.Now().Add(30 * time.Second))
			if err := tc.Handshake(); err != nil {
				log.Printf("%s -> %s: tls handshake: %v", c.RemoteAddr(), origDst(c), err)
				c.Close()
				return
			}
			br := bufio.NewReader(tc)
			head, err := br.Peek(4)
			if err != nil {
				tc.Close()
				return
			}
			tc.SetDeadline(time.Time{})
			pc := &peekedConn{Conn: tc, r: br}
			if s.cfg.trace {
				log.Printf("%s -> %s: tls %s first=%q", c.RemoteAddr(), origDst(c), tc.ConnectionState().ServerName, head)
			}
			if isHTTP(head) {
				httpLn.ch <- pc
				return
			}
			s.serveBlaze(pc)
		}()
	}
}

func isHTTP(b []byte) bool {
	for _, m := range []string{"GET ", "POST", "PUT ", "HEAD", "DELE", "OPTI", "PATC"} {
		if string(b) == m {
			return true
		}
	}
	return false
}

type peekedConn struct {
	net.Conn
	r *bufio.Reader
}

func (p *peekedConn) Read(b []byte) (int, error) { return p.r.Read(b) }

// chanListener hands already-accepted connections to an http.Server.
type chanListener struct {
	addr net.Addr
	ch   chan net.Conn
	done chan struct{}
}

func newChanListener(a net.Addr) *chanListener {
	return &chanListener{addr: a, ch: make(chan net.Conn), done: make(chan struct{})}
}

func (l *chanListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.ch:
		return c, nil
	case <-l.done:
		return nil, errors.New("closed")
	}
}
func (l *chanListener) Close() error   { close(l.done); return nil }
func (l *chanListener) Addr() net.Addr { return l.addr }
