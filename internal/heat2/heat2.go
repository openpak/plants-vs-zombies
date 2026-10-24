// Package heat2 encodes and decodes Blaze TDF values in the Heat2 binary format the game's
// Blaze SDK speaks on its Fire2 connection.
//
// Every struct member is a 4-byte header (three bytes of packed tag, one byte of type) followed
// by its value; a struct ends with a zero byte. Integers are variable length: the first byte
// carries six value bits, a sign bit (0x40) and a continuation bit (0x80); each further byte
// carries seven value bits and a continuation bit. Strings are a length (including the
// terminating NUL) and the bytes; blobs a length and the bytes; lists an element type, a count and
// the elements; maps a key type, a value type, a count and the pairs; unions one byte naming the
// active member (0x7f for none) and, if set, one member headed "VALU".
package heat2

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Heat types (the low five bits of a member header).
const (
	TInt      byte = 0
	TString   byte = 1
	TBlob     byte = 2
	TStruct   byte = 3
	TList     byte = 4
	TMap      byte = 5
	TUnion    byte = 6
	TVariable byte = 7 // a VariableTdf: present byte, TDF id, struct body
	TObjType  byte = 8
	TObjID    byte = 9
	TFloat    byte = 10
	TTime     byte = 11
	TGeneric  byte = 12
	UnionNone byte = 0x7f
)

// Value is one TDF value. Exactly one of its shapes applies, chosen by Type.
type Value struct {
	Type   byte
	Int    int64
	Str    string // TString
	Bytes  []byte // TBlob
	Fields Struct // TStruct
	// TList: Elem and Items. TMap: Key, Elem and Pairs.
	Elem, Key byte
	Items     []Value
	Pairs     [][2]Value
	// TUnion: Index (UnionNone when unset) and the member value.
	Index byte
	Union *Value
	// TObjType / TObjID: Int holds the id.
	Comp, Kind uint16
	Float      float32
	// TVariable: TdfID and Fields (Present false = empty).
	TdfID   int64
	Present bool
}

// Field is one tagged struct member.
type Field struct {
	Tag string
	V   Value
}

// Struct is an ordered member list. Blaze decoders do not require tag order, but the SDK
// writes members sorted by tag value and so do we.
type Struct []Field

// Constructors.
func I(v int64) Value  { return Value{Type: TInt, Int: v} }
func U(v uint64) Value { return Value{Type: TInt, Int: int64(v)} }
func B(v bool) Value {
	if v {
		return I(1)
	}
	return I(0)
}
func S(v string) Value            { return Value{Type: TString, Str: v} }
func Blob(b []byte) Value         { return Value{Type: TBlob, Bytes: b} }
func St(f ...Field) Value         { return Value{Type: TStruct, Fields: f} }
func F(tag string, v Value) Field { return Field{Tag: tag, V: v} }
func List(elem byte, items ...Value) Value {
	return Value{Type: TList, Elem: elem, Items: items}
}
func Map(key, elem byte, pairs ...[2]Value) Value {
	return Value{Type: TMap, Key: key, Elem: elem, Pairs: pairs}
}
func Union(index byte, v Value) Value { return Value{Type: TUnion, Index: index, Union: &v} }
func NoUnion() Value                  { return Value{Type: TUnion, Index: UnionNone} }
func ObjType(comp, kind uint16) Value { return Value{Type: TObjType, Comp: comp, Kind: kind} }
func ObjID(comp, kind uint16, id int64) Value {
	return Value{Type: TObjID, Comp: comp, Kind: kind, Int: id}
}
func Float(f float32) Value { return Value{Type: TFloat, Float: f} }
func Time(us int64) Value   { return Value{Type: TTime, Int: us} }

// Var wraps a struct as a VariableTdf of the given TDF id.
func Var(id int64, f ...Field) Value {
	return Value{Type: TVariable, Present: true, TdfID: id, Fields: f}
}

// StrMap builds a map<string,string> in key order.
func StrMap(m map[string]string) Value {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	v := Value{Type: TMap, Key: TString, Elem: TString}
	for _, k := range keys {
		v.Pairs = append(v.Pairs, [2]Value{S(k), S(m[k])})
	}
	return v
}

// Tag packs a tag of up to four characters ('A'..'Z', '0'..'9', ...) into its 24-bit form.
func Tag(s string) uint32 {
	var t uint32
	s = strings.ToUpper(s)
	for i := 0; i < 4; i++ {
		var c uint32
		if i < len(s) {
			c = uint32(s[i]-0x20) & 0x3f
		}
		t |= c << (18 - 6*uint(i))
	}
	return t
}

// TagString unpacks a 24-bit tag.
func TagString(t uint32) string {
	var b []byte
	for i := 0; i < 4; i++ {
		c := (t >> (18 - 6*uint(i))) & 0x3f
		if c != 0 {
			b = append(b, byte(c)+0x20)
		}
	}
	return string(b)
}

// Encode writes a struct body (members and the terminator is NOT included: a top-level TDF has
// no terminator).
func Encode(s Struct) []byte {
	var b bytes.Buffer
	encFields(&b, s)
	return b.Bytes()
}

func sorted(s Struct) Struct {
	out := append(Struct(nil), s...)
	sort.SliceStable(out, func(i, j int) bool { return Tag(out[i].Tag) < Tag(out[j].Tag) })
	return out
}

func encFields(b *bytes.Buffer, s Struct) {
	for _, f := range sorted(s) {
		t := Tag(f.Tag)
		b.Write([]byte{byte(t >> 16), byte(t >> 8), byte(t), f.V.Type & 0x1f})
		encValue(b, f.V)
	}
}

func putVar(b *bytes.Buffer, v int64) {
	neg := v < 0
	u := uint64(v)
	if neg {
		u = uint64(-v)
	}
	first := byte(u & 0x3f)
	if neg {
		first |= 0x40
	}
	u >>= 6
	if u == 0 {
		b.WriteByte(first)
		return
	}
	b.WriteByte(first | 0x80)
	for {
		c := byte(u & 0x7f)
		u >>= 7
		if u == 0 {
			b.WriteByte(c)
			return
		}
		b.WriteByte(c | 0x80)
	}
}

func encValue(b *bytes.Buffer, v Value) {
	switch v.Type {
	case TInt, TTime:
		putVar(b, v.Int)
	case TString:
		putVar(b, int64(len(v.Str)+1))
		b.WriteString(v.Str)
		b.WriteByte(0)
	case TBlob:
		putVar(b, int64(len(v.Bytes)))
		b.Write(v.Bytes)
	case TStruct:
		encFields(b, v.Fields)
		b.WriteByte(0)
	case TList:
		b.WriteByte(v.Elem)
		putVar(b, int64(len(v.Items)))
		for _, it := range v.Items {
			it.Type = v.Elem
			encValue(b, it)
		}
	case TMap:
		b.WriteByte(v.Key)
		b.WriteByte(v.Elem)
		putVar(b, int64(len(v.Pairs)))
		for _, p := range v.Pairs {
			k, e := p[0], p[1]
			k.Type, e.Type = v.Key, v.Elem
			encValue(b, k)
			encValue(b, e)
		}
	case TUnion:
		b.WriteByte(v.Index)
		if v.Index != UnionNone && v.Union != nil {
			t := Tag("VALU")
			b.Write([]byte{byte(t >> 16), byte(t >> 8), byte(t), v.Union.Type & 0x1f})
			encValue(b, *v.Union)
		}
	case TVariable:
		if !v.Present {
			b.WriteByte(0)
			return
		}
		b.WriteByte(1)
		putVar(b, v.TdfID)
		encFields(b, v.Fields)
		b.WriteByte(0)
	case TObjType:
		putVar(b, int64(v.Comp))
		putVar(b, int64(v.Kind))
	case TObjID:
		putVar(b, int64(v.Comp))
		putVar(b, int64(v.Kind))
		putVar(b, v.Int)
	case TFloat:
		u := math.Float32bits(v.Float)
		b.Write([]byte{byte(u >> 24), byte(u >> 16), byte(u >> 8), byte(u)})
	default:
		panic(fmt.Sprintf("heat2: cannot encode type %d", v.Type))
	}
}

// ErrShort means the buffer ended inside a value.
var ErrShort = errors.New("heat2: truncated")

type dec struct {
	b []byte
	p int
}

func (d *dec) byte_() (byte, error) {
	if d.p >= len(d.b) {
		return 0, ErrShort
	}
	c := d.b[d.p]
	d.p++
	return c, nil
}

func (d *dec) varint() (int64, error) {
	c, err := d.byte_()
	if err != nil {
		return 0, err
	}
	neg := c&0x40 != 0
	u := uint64(c & 0x3f)
	shift := uint(6)
	for c&0x80 != 0 {
		if c, err = d.byte_(); err != nil {
			return 0, err
		}
		u |= uint64(c&0x7f) << shift
		shift += 7
	}
	if neg {
		return -int64(u), nil
	}
	return int64(u), nil
}

// Decode reads a top-level TDF (members until the buffer ends).
func Decode(b []byte) (Struct, error) {
	d := &dec{b: b}
	return d.fields(true)
}

func (d *dec) fields(top bool) (Struct, error) {
	var s Struct
	for {
		if d.p >= len(d.b) {
			if top {
				return s, nil
			}
			return s, ErrShort
		}
		if d.b[d.p] == 0 {
			d.p++
			if top {
				continue // stray terminator
			}
			return s, nil
		}
		if d.p+4 > len(d.b) {
			return s, ErrShort
		}
		t := uint32(d.b[d.p])<<16 | uint32(d.b[d.p+1])<<8 | uint32(d.b[d.p+2])
		typ := d.b[d.p+3] & 0x1f
		d.p += 4
		v, err := d.value(typ)
		if err != nil {
			return s, err
		}
		s = append(s, Field{Tag: TagString(t), V: v})
	}
}

func (d *dec) value(typ byte) (Value, error) {
	v := Value{Type: typ}
	var err error
	switch typ {
	case TInt, TTime:
		v.Int, err = d.varint()
	case TString:
		var n int64
		if n, err = d.varint(); err != nil {
			return v, err
		}
		if n < 0 || d.p+int(n) > len(d.b) {
			return v, ErrShort
		}
		raw := d.b[d.p : d.p+int(n)]
		d.p += int(n)
		v.Str = strings.TrimRight(string(raw), "\x00")
	case TBlob:
		var n int64
		if n, err = d.varint(); err != nil {
			return v, err
		}
		if n < 0 || d.p+int(n) > len(d.b) {
			return v, ErrShort
		}
		v.Bytes = append([]byte(nil), d.b[d.p:d.p+int(n)]...)
		d.p += int(n)
	case TStruct:
		v.Fields, err = d.fields(false)
	case TList:
		if v.Elem, err = d.byte_(); err != nil {
			return v, err
		}
		var n int64
		if n, err = d.varint(); err != nil {
			return v, err
		}
		for i := int64(0); i < n; i++ {
			it, err := d.value(v.Elem)
			if err != nil {
				return v, err
			}
			v.Items = append(v.Items, it)
		}
	case TMap:
		if v.Key, err = d.byte_(); err != nil {
			return v, err
		}
		if v.Elem, err = d.byte_(); err != nil {
			return v, err
		}
		var n int64
		if n, err = d.varint(); err != nil {
			return v, err
		}
		for i := int64(0); i < n; i++ {
			k, err := d.value(v.Key)
			if err != nil {
				return v, err
			}
			e, err := d.value(v.Elem)
			if err != nil {
				return v, err
			}
			v.Pairs = append(v.Pairs, [2]Value{k, e})
		}
	case TUnion:
		if v.Index, err = d.byte_(); err != nil {
			return v, err
		}
		if v.Index != UnionNone {
			if d.p+4 > len(d.b) {
				return v, ErrShort
			}
			ut := d.b[d.p+3] & 0x1f
			d.p += 4
			u, err := d.value(ut)
			if err != nil {
				return v, err
			}
			v.Union = &u
		}
	case TVariable:
		var p byte
		if p, err = d.byte_(); err != nil || p == 0 {
			return v, err
		}
		v.Present = true
		if v.TdfID, err = d.varint(); err != nil {
			return v, err
		}
		v.Fields, err = d.fields(false)
	case TObjType:
		var c, k int64
		if c, err = d.varint(); err == nil {
			k, err = d.varint()
		}
		v.Comp, v.Kind = uint16(c), uint16(k)
	case TObjID:
		var c, k int64
		if c, err = d.varint(); err == nil {
			if k, err = d.varint(); err == nil {
				v.Int, err = d.varint()
			}
		}
		v.Comp, v.Kind = uint16(c), uint16(k)
	case TFloat:
		if d.p+4 > len(d.b) {
			return v, ErrShort
		}
		u := uint32(d.b[d.p])<<24 | uint32(d.b[d.p+1])<<16 | uint32(d.b[d.p+2])<<8 | uint32(d.b[d.p+3])
		d.p += 4
		v.Float = math.Float32frombits(u)
	case TGeneric:
		// A generic carries a "valid" byte, then (if valid) the TDF id, a header and the value.
		var valid byte
		if valid, err = d.byte_(); err != nil || valid == 0 {
			return v, err
		}
		if _, err = d.varint(); err != nil {
			return v, err
		}
		if d.p+4 > len(d.b) {
			return v, ErrShort
		}
		ut := d.b[d.p+3] & 0x1f
		d.p += 4
		u, err := d.value(ut)
		if err != nil {
			return v, err
		}
		v.Union = &u
		if _, err = d.byte_(); err != nil { // trailing terminator
			return v, err
		}
	default:
		return v, fmt.Errorf("heat2: unknown type %d at %d", typ, d.p)
	}
	return v, err
}

// Get returns the member with the given tag.
func (s Struct) Get(tag string) (Value, bool) {
	for _, f := range s {
		if f.Tag == tag {
			return f.V, true
		}
	}
	return Value{}, false
}

// Str returns a string member or "".
func (s Struct) Str(tag string) string { v, _ := s.Get(tag); return v.Str }

// Int returns an integer member or 0.
func (s Struct) Int(tag string) int64 { v, _ := s.Get(tag); return v.Int }

// Sub returns a struct member's fields.
func (s Struct) Sub(tag string) Struct { v, _ := s.Get(tag); return v.Fields }

// String renders a decoded value for logs.
func (s Struct) String() string {
	var b strings.Builder
	dump(&b, Value{Type: TStruct, Fields: s}, 0)
	return b.String()
}

func dump(b *strings.Builder, v Value, depth int) {
	switch v.Type {
	case TInt, TTime:
		fmt.Fprintf(b, "%d", v.Int)
	case TString:
		fmt.Fprintf(b, "%q", v.Str)
	case TBlob:
		fmt.Fprintf(b, "blob[%d]", len(v.Bytes))
	case TStruct:
		b.WriteString("{")
		for i, f := range v.Fields {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(f.Tag + "=")
			dump(b, f.V, depth+1)
		}
		b.WriteString("}")
	case TList:
		b.WriteString("[")
		for i, it := range v.Items {
			if i > 0 {
				b.WriteString(" ")
			}
			dump(b, it, depth+1)
		}
		b.WriteString("]")
	case TMap:
		b.WriteString("map{")
		for i, p := range v.Pairs {
			if i > 0 {
				b.WriteString(" ")
			}
			dump(b, p[0], depth+1)
			b.WriteString(":")
			dump(b, p[1], depth+1)
		}
		b.WriteString("}")
	case TUnion, TGeneric:
		if v.Union == nil {
			b.WriteString("union<none>")
			return
		}
		fmt.Fprintf(b, "union%d<", v.Index)
		dump(b, *v.Union, depth+1)
		b.WriteString(">")
	case TVariable:
		if !v.Present {
			b.WriteString("var<none>")
			return
		}
		fmt.Fprintf(b, "var%#x", uint64(v.TdfID))
		dump(b, Value{Type: TStruct, Fields: v.Fields}, depth+1)
	case TObjType:
		fmt.Fprintf(b, "objtype(%d/%d)", v.Comp, v.Kind)
	case TObjID:
		fmt.Fprintf(b, "objid(%d/%d/%d)", v.Comp, v.Kind, v.Int)
	case TFloat:
		fmt.Fprintf(b, "%g", v.Float)
	}
}
