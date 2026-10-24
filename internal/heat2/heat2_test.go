package heat2

import (
	"bytes"
	"testing"
)

func TestTag(t *testing.T) {
	// "CERT" packs to 0x8e5cb4 (from the game's member table).
	if got := Tag("CERT"); got != 0x8e5cb4 {
		t.Fatalf("Tag(CERT) = %#x", got)
	}
	if TagString(0x8e5cb4) != "CERT" || TagString(Tag("CNT")) != "CNT" {
		t.Fatal("TagString round trip")
	}
}

func TestVarint(t *testing.T) {
	for _, c := range []struct {
		v   int64
		enc []byte
	}{{0, []byte{0}}, {1, []byte{1}}, {63, []byte{0x3f}}, {64, []byte{0x80, 0x01}}, {-1, []byte{0x41}}, {8191, []byte{0xbf, 0x7f}}} {
		var b bytes.Buffer
		putVar(&b, c.v)
		if !bytes.Equal(b.Bytes(), c.enc) {
			t.Errorf("putVar(%d) = %x, want %x", c.v, b.Bytes(), c.enc)
		}
		d := &dec{b: c.enc}
		if got, _ := d.varint(); got != c.v {
			t.Errorf("varint(%x) = %d", c.enc, got)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	s := Struct{
		F("STIM", I(1700000000)),
		F("NAME", S("pvz")),
		F("BLOB", Blob([]byte{1, 2, 3})),
		F("SUB", St(F("A", I(-5)), F("B", S("x")))),
		F("LIST", List(TString, S("a"), S("b"))),
		F("MAP", StrMap(map[string]string{"k": "v"})),
		F("ADDR", Union(0, St(F("IP", I(1))))),
		F("NONE", NoUnion()),
		F("OBJ", ObjID(0x7802, 2, 99)),
		F("FLT", Float(1.5)),
	}
	out, err := Decode(Encode(s))
	if err != nil {
		t.Fatal(err)
	}
	if out.Int("STIM") != 1700000000 || out.Str("NAME") != "pvz" || out.Sub("SUB").Int("A") != -5 {
		t.Fatalf("decoded %s", out)
	}
	if v, _ := out.Get("ADDR"); v.Union == nil || v.Union.Fields.Int("IP") != 1 {
		t.Fatalf("union %s", out)
	}
	if v, _ := out.Get("MAP"); len(v.Pairs) != 1 || v.Pairs[0][1].Str != "v" {
		t.Fatalf("map %s", out)
	}
}
