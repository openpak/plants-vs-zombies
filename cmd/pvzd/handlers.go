package main

import (
	"log"
	"strings"
	"time"

	h "openpak/plants-vs-zombies/internal/heat2"
)

const (
	platformNX    = 21 // Blaze::ClientPlatformType nx
	personaActive = 2  // Nucleus::PersonaStatus ACTIVE
	serviceName   = "plantsvszombies-gw3-nx"
	personaNS     = "cem_ea_id"
)

// Components this server answers (PreAuthResponse.componentIds).
var offeredComponents = []uint16{
	compAuth, compGameManager, compRedirector, compStats, compUtil, 0x000a, 0x000b, 0x000e, 0x000f,
	compAssociation, 0x001b, compGameReport, compByteVault, compAchievements,
	compQuarter, compPvzGw, compPvzGwStats, compPvzGwEconomy, compUserSessions,
}

func init() {
	// Util
	on(compUtil, 0x02, func(s *session, _ h.Struct) (h.Struct, uint32) { // ping
		return h.Struct{h.F("STIM", h.I(time.Now().Unix()))}, 0
	})
	on(compUtil, 0x07, utilPreAuth)
	on(compUtil, 0x08, utilPostAuth)
	on(compUtil, 0x01, utilFetchClientConfig)
	for _, cmd := range []uint16{0x03, 0x16, 0x17, 0x1b, 0x1c, 0x1d, 0x0b, 0x0e, 0x1a} { // set*/save: accept
		on(compUtil, cmd, func(s *session, _ h.Struct) (h.Struct, uint32) { return nil, 0 })
	}
	on(compUtil, 0x0c, func(s *session, _ h.Struct) (h.Struct, uint32) { // userSettingsLoadAll
		return h.Struct{h.F("SMAP", h.StrMap(nil))}, 0
	})
	on(compUtil, 0x19, func(s *session, _ h.Struct) (h.Struct, uint32) { // getUserOptions
		return userOptions(s), 0
	})

	// Authentication
	on(compAuth, 0x0a, authLogin)
	on(compAuth, 0x46, func(s *session, _ h.Struct) (h.Struct, uint32) { return nil, 0 }) // logout
	on(compAuth, 0x1d, func(s *session, _ h.Struct) (h.Struct, uint32) {                  // listUserEntitlements2
		return h.Struct{h.F("NLST", h.List(h.TStruct))}, 0
	})
	on(compAuth, 0x30, func(s *session, _ h.Struct) (h.Struct, uint32) { // listPersonaEntitlements2
		return h.Struct{h.F("NLST", h.List(h.TStruct))}, 0
	})
	on(compAuth, 0x24, func(s *session, _ h.Struct) (h.Struct, uint32) { // getAuthToken
		return h.Struct{h.F("AUTH", h.S(newToken("QVQx")))}, 0
	})

	// UserSessions: accept updates.
	for _, cmd := range []uint16{0x05, 0x08, 0x14, 0x19, 0x1a, 0x25, 0x37} {
		on(compUserSessions, cmd, func(s *session, _ h.Struct) (h.Struct, uint32) { return nil, 0 })
	}
}

// Set from the server config at start (main), used in the BlazeSDK config section.
var blazeHostForConfig, blazePortForConfig = "spring18.gosredirector.ea.com", "42230"

func utilFetchClientConfig(s *session, req h.Struct) (h.Struct, uint32) {
	return h.Struct{h.F("CONF", h.StrMap(clientConfig(req.Str("CFID"))))}, 0
}

// clientConfig returns a config section. Unknown sections are empty, which the SDK treats as
// "use defaults".
func clientConfig(section string) map[string]string {
	switch section {
	case "IdentityParams":
		// Extra query parameters the SDK's console login appends, one "&key=value" per entry, to
		// <nucleusConnect>/connect/auth?response_type=code.
		return map[string]string{
			"redirect_uri": "http://127.0.0.1/success",
			"display":      "console2/welcome",
		}
	case "", "BlazeSDK":
		return map[string]string{
			"pingPeriod": "15s",
			// Where the console login finds Nucleus.
			"nucleusConnect":        "https://accounts.ea.com",
			"nucleusConnectTrusted": "https://accounts.ea.com",
			"nucleusProxy":          "https://gateway.ea.com",
			"nucleusPortal":         "https://signin.ea.com",
			// The ByteVault component lives on its own connection; it is this server too.
			"bytevaultHostname":     blazeHostForConfig,
			"bytevaultPort":         blazePortForConfig,
			"bytevaultSecure":       "true",
			"connIdleTimeout":       "90s",
			"defaultRequestTimeout": "60s",
		}
	}
	return map[string]string{}
}

func utilPreAuth(s *session, req h.Struct) (h.Struct, uint32) {
	ci := req.Sub("CINF")
	log.Printf("blaze %d: preAuth client=%q version=%q sku=%q env=%q", s.id, ci.Str("CLNT"), ci.Str("CVER"), ci.Str("CSKU"), ci.Str("ENV"))
	comps := h.List(h.TInt)
	for _, c := range offeredComponents {
		comps.Items = append(comps.Items, h.I(int64(c)))
	}
	return h.Struct{
		h.F("ASRC", h.S("303107")),
		h.F("CIDS", comps),
		h.F("CLID", h.S("PVZ_GW3_NX")),
		h.F("CONF", h.St(h.F("CONF", h.StrMap(clientConfig(""))))),
		h.F("ESRC", h.S("303107")),
		h.F("INST", h.S(serviceName)),
		h.F("MAID", h.I(int64(s.id))),
		h.F("MINR", h.B(false)),
		h.F("NASP", h.S(personaNS)),
		h.F("PILD", h.S("")),
		h.F("PLAT", h.S("nx")),
		h.F("QOSS", h.St(
			h.F("CQFR", h.Time(30_000_000)),
			h.F("CQRR", h.Time(300_000_000)),
			h.F("LTPS", h.Map(h.TString, h.TStruct)),
			h.F("QCNF", h.St(h.F("DPSP", h.I(17502)), h.F("QCA", h.S(s.srv.cfg.blazeHost)), h.F("QCP", h.I(17502)), h.F("QPR", h.S("pvz_nx_profile")))),
			h.F("SQRR", h.Time(300_000_000)),
		)),
		h.F("RSRC", h.S("303107")),
		h.F("SVER", h.S("Blaze 15.1.1.8.2 (OpenPak pvzd)")),
	}, 0
}

func userOptions(s *session) h.Struct {
	var id int64
	if s.user != nil {
		id = s.user.BlazeID
	}
	return h.Struct{h.F("TMOP", h.I(0)), h.F("UID", h.I(id))}
}

func utilPostAuth(s *session, _ h.Struct) (h.Struct, uint32) {
	return h.Struct{
		h.F("TELE", h.St(h.F("ADRS", h.S("")), h.F("DISA", h.S("*")), h.F("PORT", h.I(0)))),
		h.F("TICK", h.St(h.F("ADRS", h.S("")), h.F("PORT", h.I(0)), h.F("SKEY", h.S("")))),
		h.F("UROP", h.St(userOptions(s)...)),
	}, 0
}

// authLogin signs the console in. The credential is an auth code this server's Nucleus issued,
// or (if the game passes it straight through) the console's BAAS id_token itself.
func authLogin(s *session, req h.Struct) (h.Struct, uint32) {
	code := req.Str("AUTH")
	var a *account
	first := false
	if code != "" {
		a = takeGrant(codes, code, true)
	}
	if a == nil {
		tok := ""
		if strings.HasPrefix(code, "eyJ") {
			tok = code
		} else if v, ok := req.Get("EXTB"); ok && strings.HasPrefix(string(v.Bytes), "eyJ") {
			tok = strings.TrimRight(string(v.Bytes), "\x00")
		}
		if tok != "" {
			var err error
			a, first, err = s.srv.accountFromIDToken(tok)
			if err != nil {
				log.Printf("blaze %d: login: identity: %v", s.id, err)
			}
		}
	}
	if a == nil {
		log.Printf("blaze %d: login refused (no usable credential): %s", s.id, req)
		return nil, 0x00010000 | compAuth // generic auth failure
	}
	s.user = a
	s.key = newToken("")
	now := time.Now().Unix()
	log.Printf("blaze %d: login blaze=%d name=%q first=%v", s.id, a.BlazeID, a.Name, first)
	platform := h.St(
		h.F("EAID", h.St()),
		h.F("EXID", h.St(h.F("SWID", h.S(a.BaasUserID)))),
		h.F("PLAT", h.I(platformNX)),
	)
	resp := h.Struct{
		h.F("ANON", h.B(false)),
		h.F("SESS", h.St(
			h.F("1CON", h.B(first)),
			h.F("AIDS", platform),
			h.F("BUID", h.I(a.BlazeID)),
			h.F("FRST", h.B(first)),
			h.F("GEO", h.B(false)),
			h.F("KEY", h.S(s.key)),
			h.F("LLOG", h.I(now)),
			h.F("MAIL", h.S("")),
			h.F("PDTL", h.St(
				h.F("DSNM", h.S(a.Name)),
				h.F("LAST", h.I(now)),
				h.F("PID", h.I(a.BlazeID)),
				h.F("STAS", h.I(personaActive)),
				h.F("XREF", h.I(0)),
			)),
			h.F("UID", h.I(a.BlazeID)),
		)),
		h.F("SPAM", h.B(true)),
		h.F("UNDR", h.B(false)),
	}
	go s.announce(platform, first)
	return resp, 0
}

// announce tells the client about its own user session, as the Blaze server does after login.
func (s *session) announce(platform h.Value, first bool) {
	time.Sleep(50 * time.Millisecond)
	a := s.user
	now := time.Now().Unix()
	ident := h.St(
		h.F("AID", h.I(a.BlazeID)),
		h.F("AIDS", platform),
		h.F("ALOC", h.I(0x656e5553)), // enUS
		h.F("CNTY", h.I(0x5553)),
		h.F("ID", h.I(a.BlazeID)),
		h.F("NAME", h.S(a.Name)),
		h.F("NASP", h.S(personaNS)),
		h.F("ORIG", h.I(0)),
		h.F("PIDI", h.I(a.BlazeID)),
		h.F("PLAT", h.I(platformNX)),
	)
	ext := h.St(
		h.F("ADDR", h.NoUnion()),
		h.F("BPS", h.S("")),
		h.F("CTY", h.S("")),
		h.F("DMAP", h.Map(h.TInt, h.TInt)),
		h.F("HWFG", h.I(0)),
		h.F("ISP", h.S("")),
		h.F("PSLM", h.List(h.TInt)),
		h.F("TZ", h.S("")),
		h.F("UATT", h.I(0)),
		h.F("ULST", h.List(h.TObjID)),
	)
	s.notifyUser(compUserSessions, 0x02, h.Struct{h.F("DATA", ext), h.F("USER", ident)})
	s.notifyUser(compUserSessions, 0x08, h.Struct{
		h.F("1CON", h.B(first)),
		h.F("AIDS", platform),
		h.F("ALOC", h.I(0x656e5553)),
		h.F("BUID", h.I(a.BlazeID)),
		h.F("CGID", h.ObjID(compUserSessions, 2, a.BlazeID)),
		h.F("CNTY", h.I(0x5553)),
		h.F("DSNM", h.S(a.Name)),
		h.F("FRST", h.B(first)),
		h.F("KEY", h.S(s.key)),
		h.F("LAST", h.I(now)),
		h.F("LLOG", h.I(now)),
		h.F("MAIL", h.S("")),
		h.F("NASP", h.S(personaNS)),
		h.F("PID", h.I(a.BlazeID)),
		h.F("PLAT", h.I(platformNX)),
		h.F("UID", h.I(a.BlazeID)),
		h.F("USTP", h.I(0)),
		h.F("XREF", h.I(0)),
	})
}
