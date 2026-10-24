package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
)

// httpMux routes every HTTP request the game makes to EA hosts: the redirector (over the TLS
// listener) and the Nucleus/gateway hosts (plain HTTP behind Traefik). Anything unknown is logged
// in full so the next capture shows what to add.
func (s *server) httpMux() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		host := strings.ToLower(r.Host)
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if s.cfg.trace {
			log.Printf("http %s %s %s%s body=%q", r.RemoteAddr, r.Method, host, r.URL.RequestURI(), clip(body, 2000))
		}
		switch {
		case r.URL.Path == "/redirector/getServerInstance":
			s.redirector(w, r, body)
		case strings.HasPrefix(host, "accounts.") || strings.HasPrefix(host, "signin.") || strings.HasPrefix(host, "gateway."):
			s.nucleus(w, r, host, body)
		default:
			log.Printf("http unhandled %s %s%s headers=%v body=%q", r.Method, host, r.URL.RequestURI(), r.Header, clip(body, 2000))
			http.NotFound(w, r)
		}
	})
}

func clip(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "..."
	}
	return string(b)
}

// redirector answers getServerInstance with this server as the Blaze instance, secure.
// The reply is the ServerInstanceInfo TDF in Blaze's XML form: element names are the member
// names in lower case, and the address union names its active member with member="0"
// (ipAddress) around a <valu> element.
func (s *server) redirector(w http.ResponseWriter, r *http.Request, body []byte) {
	ip := net.ParseIP(s.cfg.blazeIP).To4()
	var ipn uint32
	if ip != nil {
		ipn = binary.BigEndian.Uint32(ip)
	}
	log.Printf("redirector: %s -> %s (%s):%d", r.RemoteAddr, s.cfg.blazeHost, s.cfg.blazeIP, s.cfg.blazePort)
	w.Header().Set("Content-Type", "application/xml")
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<serverinstanceinfo>
  <address member="0">
    <valu>
      <hostname>%s</hostname>
      <ip>%d</ip>
      <port>%d</port>
    </valu>
  </address>
  <secure>1</secure>
  <trialservicename></trialservicename>
  <defaultdnsaddress>0</defaultdnsaddress>
</serverinstanceinfo>
`, s.cfg.blazeHost, ipn, s.cfg.blazePort)
}
