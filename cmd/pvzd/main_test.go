package main

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"openpak/plants-vs-zombies/internal/fire2"
	h "openpak/plants-vs-zombies/internal/heat2"
)

func testCert(t *testing.T) tls.Certificate {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"spring18.gosredirector.ea.com"}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func startTest(t *testing.T) (*server, string) {
	st, err := openStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &server{cfg: config{blazeHost: "spring18.gosredirector.ea.com", blazeIP: "10.0.0.1", blazePort: 42230}, store: st}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go s.serveTLS(ln, &tls.Config{Certificates: []tls.Certificate{testCert(t)}}, s.httpMux())
	return s, ln.Addr().String()
}

func dial(t *testing.T, addr string) *tls.Conn {
	c, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRedirector(t *testing.T) {
	_, addr := startTest(t)
	c := dial(t, addr)
	defer c.Close()
	io.WriteString(c, "POST /redirector/getServerInstance HTTP/1.1\r\nHost: spring18.gosredirector.ea.com:42230\r\nContent-Length: 2\r\nConnection: Close\r\n\r\n<>")
	resp, err := http.ReadResponse(bufio.NewReader(c), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	for _, want := range []string{`<address member="0">`, "<valu>", "<ip>167772161</ip>", "<port>42230</port>", "<secure>1</secure>"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("reply lacks %q:\n%s", want, b)
		}
	}
}

func TestBlazePingAndPreAuth(t *testing.T) {
	_, addr := startTest(t)
	c := dial(t, addr)
	defer c.Close()
	// The first frame the game sends: Util.ping, message #1, empty.
	c.Write(fire2.Frame{Component: compUtil, Command: 0x02, MsgNum: 1}.Bytes())
	f, err := fire2.Read(c)
	if err != nil {
		t.Fatal(err)
	}
	if f.Type != fire2.Reply || f.MsgNum != 1 || f.Component != compUtil {
		t.Fatalf("ping reply %v", f)
	}
	body, _ := h.Decode(f.Payload)
	if body.Int("STIM") == 0 {
		t.Fatalf("ping reply body %s", body)
	}
	req := h.Encode(h.Struct{h.F("CINF", h.St(h.F("CLNT", h.S("pvz client"))))})
	c.Write(fire2.Frame{Component: compUtil, Command: 0x07, MsgNum: 2, Payload: req}.Bytes())
	f, err = fire2.Read(c)
	if err != nil {
		t.Fatal(err)
	}
	body, err = h.Decode(f.Payload)
	if err != nil || body.Str("INST") != serviceName || body.Str("PLAT") != "nx" {
		t.Fatalf("preAuth reply %s err %v", body, err)
	}
}
