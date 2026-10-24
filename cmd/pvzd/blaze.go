package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"openpak/plants-vs-zombies/internal/fire2"
	h "openpak/plants-vs-zombies/internal/heat2"
)

// Component ids the game's Blaze SDK registers (from its component constructors).
const (
	compAuth         = 0x0001
	compGameManager  = 0x0004
	compRedirector   = 0x0005
	compStats        = 0x0007
	compUtil         = 0x0009
	compAssociation  = 0x0019
	compGameReport   = 0x001c
	compByteVault    = 0x001f
	compAchievements = 0x0021
	compQuarter      = 0x0804
	compPvzGw        = 0x0805
	compPvzGwStats   = 0x080a
	compPvzGwEconomy = 0x080d
	compUserSessions = 0x7802
)

// Blaze error codes are (code << 16) | component. Only the generic ones are used here.
const errSystem = 0x00010000 // ERR_SYSTEM

type session struct {
	srv    *server
	conn   net.Conn
	w      *bufio.Writer
	wmu    sync.Mutex
	id     uint64
	user   *account // set after login
	key    string
	notify uint32
}

var sessionSeq atomic.Uint64

type handler func(s *session, req h.Struct) (h.Struct, uint32)

func (s *server) serveBlaze(c net.Conn) {
	sess := &session{srv: s, conn: c, w: bufio.NewWriter(c), id: sessionSeq.Add(1)}
	log.Printf("blaze %d: open from %s", sess.id, c.RemoteAddr())
	defer func() {
		c.Close()
		log.Printf("blaze %d: closed", sess.id)
	}()
	for {
		c.SetReadDeadline(time.Now().Add(5 * time.Minute))
		f, err := fire2.Read(c)
		if err != nil {
			if err == io.EOF {
				log.Printf("blaze %d: peer closed", sess.id)
			} else {
				log.Printf("blaze %d: read: %v", sess.id, err)
			}
			return
		}
		sess.handle(f)
	}
}

func (sess *session) handle(f fire2.Frame) {
	switch f.Type {
	case fire2.Ping:
		sess.send(fire2.Frame{Component: f.Component, Command: f.Command, MsgNum: f.MsgNum, Type: fire2.PingReply})
		return
	case fire2.Message:
	default:
		log.Printf("blaze %d: ignoring frame %v", sess.id, f)
		return
	}
	req, err := h.Decode(f.Payload)
	name := commandName(f.Component, f.Command)
	if err != nil {
		log.Printf("blaze %d: %s: decode: %v (%x)", sess.id, name, err, clipb(f.Payload, 256))
	}
	if sess.srv.cfg.trace {
		log.Printf("blaze %d: <- %s #%d %s", sess.id, name, f.MsgNum, req)
	}
	hd := handlers[uint32(f.Component)<<16|uint32(f.Command)]
	var resp h.Struct
	var code uint32
	if hd == nil {
		log.Printf("blaze %d: unhandled %s (%#x/%#x) req=%s", sess.id, name, f.Component, f.Command, req)
	} else {
		resp, code = hd(sess, req)
	}
	out := fire2.Frame{Component: f.Component, Command: f.Command, MsgNum: f.MsgNum, Type: fire2.Reply}
	if code != 0 {
		out.Type = fire2.ErrorReply
		out.Metadata = h.Encode(h.Struct{h.F("ERRC", h.I(int64(int32(code))))})
	}
	out.Payload = h.Encode(resp)
	if sess.srv.cfg.trace {
		log.Printf("blaze %d: -> %s #%d code=%#x %s", sess.id, name, f.MsgNum, code, resp)
	}
	sess.send(out)
}

func (sess *session) send(f fire2.Frame) {
	sess.wmu.Lock()
	defer sess.wmu.Unlock()
	sess.w.Write(f.Bytes())
	if err := sess.w.Flush(); err != nil {
		log.Printf("blaze %d: write: %v", sess.id, err)
	}
}

// notifyUser sends an unsolicited notification.
func (sess *session) notifyUser(comp, cmd uint16, body h.Struct) {
	sess.send(fire2.Frame{Component: comp, Command: cmd, Type: fire2.Notification, Payload: h.Encode(body)})
	if sess.srv.cfg.trace {
		log.Printf("blaze %d: => notify %s %s", sess.id, notificationName(comp, cmd), body)
	}
}

func clipb(b []byte, n int) []byte {
	if len(b) > n {
		return b[:n]
	}
	return b
}

var handlers = map[uint32]handler{}

func on(comp, cmd uint16, fn handler) { handlers[uint32(comp)<<16|uint32(cmd)] = fn }
