// Package fire2 reads and writes Blaze Fire2 frames.
//
// A frame is a 16-byte header, then a metadata TDF, then the payload TDF:
//
//	0  uint32 payload length (big endian)
//	4  uint16 metadata length
//	6  uint16 component
//	8  uint16 command
//	10 uint24 message number
//	13 uint8  message type << 5 | options
//	14 uint16 reserved (0)
package fire2

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Message types (header byte 13, top three bits).
const (
	Message      = 0
	Reply        = 1
	Notification = 2
	ErrorReply   = 3
	Ping         = 4
	PingReply    = 5
)

// HeaderLen is the fixed header size.
const HeaderLen = 16

// MaxFrame bounds a frame we accept.
const MaxFrame = 4 << 20

// Frame is one decoded frame.
type Frame struct {
	Component, Command uint16
	MsgNum             uint32
	Type               byte
	Options            byte
	Metadata           []byte
	Payload            []byte
}

func (f Frame) String() string {
	return fmt.Sprintf("comp=%#x cmd=%#x num=%d type=%d opt=%#x meta=%d payload=%d",
		f.Component, f.Command, f.MsgNum, f.Type, f.Options, len(f.Metadata), len(f.Payload))
}

// Read reads one frame.
func Read(r io.Reader) (Frame, error) {
	var h [HeaderLen]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return Frame{}, err
	}
	pl := binary.BigEndian.Uint32(h[0:4])
	ml := binary.BigEndian.Uint16(h[4:6])
	if pl > MaxFrame {
		return Frame{}, fmt.Errorf("fire2: payload %d too large", pl)
	}
	f := Frame{
		Component: binary.BigEndian.Uint16(h[6:8]),
		Command:   binary.BigEndian.Uint16(h[8:10]),
		MsgNum:    uint32(h[10])<<16 | uint32(h[11])<<8 | uint32(h[12]),
		Type:      h[13] >> 5,
		Options:   h[13] & 0x1f,
	}
	body := make([]byte, int(ml)+int(pl))
	if _, err := io.ReadFull(r, body); err != nil {
		return Frame{}, err
	}
	f.Metadata, f.Payload = body[:ml], body[ml:]
	return f, nil
}

// Bytes encodes a frame.
func (f Frame) Bytes() []byte {
	out := make([]byte, HeaderLen, HeaderLen+len(f.Metadata)+len(f.Payload))
	binary.BigEndian.PutUint32(out[0:4], uint32(len(f.Payload)))
	binary.BigEndian.PutUint16(out[4:6], uint16(len(f.Metadata)))
	binary.BigEndian.PutUint16(out[6:8], f.Component)
	binary.BigEndian.PutUint16(out[8:10], f.Command)
	out[10], out[11], out[12] = byte(f.MsgNum>>16), byte(f.MsgNum>>8), byte(f.MsgNum)
	out[13] = f.Type<<5 | f.Options&0x1f
	out = append(out, f.Metadata...)
	return append(out, f.Payload...)
}
