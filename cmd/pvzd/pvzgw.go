package main

// The game's own Blaze components (PvzGw 0x805, PvzGwEconomy 0x80d, PvzGwStats 0x80a): account,
// economy and progression. Requests are answered "owned-everything" where the reply shape allows,
// and every blob the game stores (ByteVault records, stats, loadouts) is kept per player.

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"time"

	h "openpak/plants-vs-zombies/internal/heat2"
)

// TDF ids (EA::TDF type ids, from the game's type descriptions) of economy request/response bodies.
const (
	tdfGetInventoryRequest          = 0x222a8802
	tdfGetMarketplaceGrantsRequest  = 0x27449d9a
	tdfGetMarketplaceGrantsResponse = 0x6c680fc6
)

func empty(s *session, _ h.Struct) (h.Struct, uint32) { return nil, 0 }

func init() {
	for _, cmd := range []uint16{0x02, 0x19, 0x5f, 0x60, 0x70, 0x73, 0x74, 0x75, 0x82} {
		on(compPvzGw, cmd, empty)
	}
	on(compPvzGw, 0x71, pvzUpdateInstalledLicenses)
	on(compPvzGw, 0x11, func(s *session, _ h.Struct) (h.Struct, uint32) { // getClientSettings
		return h.Struct{
			h.F("GENS", h.List(h.TStruct)),
			h.F("PACF", h.St(h.F("LCUV", h.S("")), h.F("PRSF", h.S("")), h.F("PSTA", h.I(0)), h.F("PURL", h.S("")))),
		}, 0
	})
	on(compPvzGw, 0x32, pvzGetByteVault)
	on(compPvzGw, 0x33, pvzUpdateByteVault)
	on(compPvzGw, 0x34, func(s *session, req h.Struct) (h.Struct, uint32) {
		if s.user != nil {
			s.srv.store.update(s.user, func(a *account) { delete(a.Data, "bytevault") })
		}
		return nil, 0
	})
	on(compPvzGw, 0x79, func(s *session, _ h.Struct) (h.Struct, uint32) { // getSignatureToken
		tok := make([]byte, 32)
		rand.Read(tok)
		return h.Struct{h.F("EXPY", h.Time(time.Now().Add(time.Hour).UnixMicro())), h.F("SIGN", h.Blob(tok))}, 0
	})
	on(compPvzGw, 0x5a, func(s *session, _ h.Struct) (h.Struct, uint32) { // getActiveChallenges
		return h.Struct{h.F("AVAL", h.I(0)), h.F("CHIM", h.Map(h.TString, h.TList)), h.F("SKIP", h.I(0))}, 0
	})
	on(compPvzGw, 0x5b, func(s *session, _ h.Struct) (h.Struct, uint32) { // getAvailableChallenges
		return h.Struct{h.F("AVAL", h.I(0)), h.F("AVGP", h.I(0)), h.F("CGCM", h.Map(h.TString, h.TMap))}, 0
	})
	on(compPvzGw, 0x61, func(s *session, _ h.Struct) (h.Struct, uint32) { // getStatRewards
		return h.Struct{h.F("SRIL", h.List(h.TStruct))}, 0
	})
	on(compPvzGw, 0x04, func(s *session, _ h.Struct) (h.Struct, uint32) { // getStoreItemList
		return h.Struct{h.F("SLST", h.List(h.TStruct))}, 0
	})

	// Economy: every call is an AsyncRequest answered at once and then by AsyncResponseNotification.
	on(compPvzGwEconomy, 0x01, econRequest)
	for _, cmd := range []uint16{0x02, 0x09, 0x0a} {
		on(compPvzGwEconomy, cmd, econRequest)
	}

	// Stats: accept updates, answer reads with no values (the game keeps its defaults).
	on(compPvzGwStats, 0xc8, func(s *session, req h.Struct) (h.Struct, uint32) {
		return h.Struct{h.F("STAT", h.List(h.TString))}, 0
	})
	on(compPvzGwStats, 0x64, statsGetUser)
	on(compPvzGwStats, 0x65, statsGetAll)
}

func pvzUpdateInstalledLicenses(s *session, req h.Struct) (h.Struct, uint32) {
	lic := h.Map(h.TString, h.TInt)
	if v, ok := req.Get("INLI"); ok {
		for _, it := range v.Items {
			lic.Pairs = append(lic.Pairs, [2]h.Value{h.S(it.Str), h.B(true)})
		}
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.notifyUser(compPvzGw, 0x02, h.Struct{h.F("LICE", lic)})
	}()
	return nil, 0
}

func pvzGetByteVault(s *session, req h.Struct) (h.Struct, uint32) {
	var data []byte
	mime := "application/octet-stream"
	if s.user != nil {
		if v := s.user.Data["bytevault"]; v != "" {
			data, _ = base64.StdEncoding.DecodeString(v)
		}
		if m := s.user.Data["bytevault.mime"]; m != "" {
			mime = m
		}
	}
	now := time.Now().UnixMicro()
	return h.Struct{h.F("GRR", h.St(
		h.F("CREA", h.Time(now)),
		h.F("LOAD", h.St(h.F("DATA", h.Blob(data)), h.F("MIME", h.S(mime)))),
		h.F("LOC", h.S("")),
		h.F("RMOT", h.B(false)),
		h.F("UPDT", h.Time(now)),
	))}, 0
}

func pvzUpdateByteVault(s *session, req h.Struct) (h.Struct, uint32) {
	load := req.Sub("LOAD")
	blob, _ := load.Get("DATA")
	if s.user != nil {
		s.srv.store.update(s.user, func(a *account) {
			if a.Data == nil {
				a.Data = map[string]string{}
			}
			a.Data["bytevault"] = base64.StdEncoding.EncodeToString(blob.Bytes)
			a.Data["bytevault.mime"] = load.Str("MIME")
		})
		log.Printf("blaze %d: bytevault saved %d bytes for %d", s.id, len(blob.Bytes), s.user.BlazeID)
	}
	return h.Struct{h.F("URR", h.St(h.F("LOC", h.S("")), h.F("RCRE", h.B(false)), h.F("RMOT", h.B(false))))}, 0
}

// econRequest acknowledges an AsyncRequest and answers it with an AsyncResponseNotification.
func econRequest(s *session, req h.Struct) (h.Struct, uint32) {
	task := req.Int("TKID")
	inner, _ := req.Get("TDF")
	log.Printf("blaze %d: economy task %d tdf %#x %s", s.id, task, uint64(inner.TdfID), h.Struct(inner.Fields))
	var body h.Value
	switch uint32(inner.TdfID) {
	case tdfGetMarketplaceGrantsRequest:
		body = h.Var(tdfGetMarketplaceGrantsResponse,
			h.F("BLID", h.I(blazeOf(s))), h.F("CURS", h.Map(h.TString, h.TInt)), h.F("IIAS", h.List(h.TString)))
	default:
		body = h.Value{Type: h.TVariable}
	}
	go func() {
		time.Sleep(30 * time.Millisecond)
		s.notifyUser(compPvzGwEconomy, 0x0a, h.Struct{h.F("ERR", h.I(0)), h.F("TDF", body), h.F("TKID", h.I(task))})
	}()
	return nil, 0
}

func blazeOf(s *session) int64 {
	if s.user != nil {
		return s.user.BlazeID
	}
	return 0
}

func statsGetUser(s *session, req h.Struct) (h.Struct, uint32) {
	task := req.Int("TASK")
	go func() {
		time.Sleep(30 * time.Millisecond)
		s.notifyUser(compPvzGwStats, 0x0a, h.Struct{
			h.F("ESVM", h.Map(h.TInt, h.TStruct)),
			h.F("NNFO", h.St(h.F("NNUM", h.I(1)), h.F("TASK", h.I(task)), h.F("TNUM", h.I(1)))),
		})
	}()
	return h.Struct{h.F("STAT", h.List(h.TString))}, 0
}

func statsGetAll(s *session, req h.Struct) (h.Struct, uint32) {
	task := req.Int("TASK")
	eid := req.Int("EID")
	go func() {
		time.Sleep(30 * time.Millisecond)
		s.notifyUser(compPvzGwStats, 0x0b, h.Struct{
			h.F("CSVM", h.Map(h.TString, h.TStruct)),
			h.F("EID", h.I(eid)),
			h.F("NNFO", h.St(h.F("NNUM", h.I(1)), h.F("TASK", h.I(task)), h.F("TNUM", h.I(1)))),
		})
	}()
	return h.Struct{h.F("CATS", h.List(h.TString))}, 0
}
