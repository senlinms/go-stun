package stun

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/pixelbender/go-stun/mux"
	"io"
)

// ErrIntegrityCheckFailure is returned by Decode when a STUN message contains
// a MESSAGE-INTEGRITY attribute and it does not equal to HMAC-SHA1 sum.
var ErrIntegrityCheckFailure = errors.New("stun: integrity check failure")

// ErrIncorrectFingerprint is returned by Decode when a STUN message contains
// a FINGERPRINT attribute and it does not equal to checksum.
var ErrIncorrectFingerprint = errors.New("stun: incorrect fingerprint")

// ErrFormat is returned by Decode when a buffer is not a valid STUN message.
var ErrFormat = errors.New("stun: incorrect format")

// ErrFormat is returned by ReadMessage when a STUN message was truncated.
var ErrTruncated = errors.New("stun: truncated")

// ErrUnknownAttrs is returned when a STUN message contains unknown comprehension-required attributes.
type ErrUnknownAttrs []uint16

func (e ErrUnknownAttrs) Error() string {
	return fmt.Sprintf("stun: unknown attributes %#v", e.Attributes)
}

// A Decoder reads and decodes STUN messages from a buffer.
type Decoder struct {
	Config *Config
}

// Decode reads STUN message from the buffer.
// Checks MESSAGE-INTEGRITY attribute if GetAuthKey is specified.
// Checks FINGERPRINT attribute if present.
// Returns io.EOF if the buffer size is not enough.
// Returns ErrUnknownAttrs containing unknown comprehension-required STUN attributes.

// func(attrs Attributes) ([]byte, error)
//

func (dec *Decoder) Decode(r mux.Reader, key []byte) (m *Message, err error) {
	var b []byte

	if b, err = r.Next(20); err != nil {
		return
	}
	n := int(be.Uint16(h[2:]))
	d, err := r.Next(n)
	if err != nil {
		return nil, err
	}
	p := 20
	m := &Message{
		Method:      be.Uint16(h),
		Transaction: h[4:20],
		Attributes:  make(Attributes),
	}
	var unk []uint16

	for len(d) > 4 {
		at, n := be.Uint16(d), int(be.Uint16(d[2:])+4)
		// Padding
		s := n
		if mod := n & 3; mod != 0 {
			s = n + 4 - mod
		}
		if len(d) < s {
			return nil, io.EOF
		}
		buf := d[4:n]
		d = d[s:]
		codec := dec.getAttrCodec(at)
		if codec == nil {
			if at < 0x8000 {
				unk = append(unk, at)
			}
			p += s
			continue
		}
		attr, err := codec.Decode(&reader{msg: r.buf, buf: buf})
		if err != nil {
			return nil, err
		}
		m.Attributes[at] = attr
		switch at {
		case AttrMessageIntegrity:
			be.PutUint16(h[2:], uint16(p+4))
			if key == nil {
				key, err = dec.getAuthKey(m.Attributes)
				if err != nil {
					return nil, err
				}
			}
			sum := integrity(r.buf[:p], key)
			if !bytes.Equal(attr.([]byte), sum) {
				return nil, ErrIntegrityCheckFailure
			}
			m.Key = key
			d = nil
		case AttrFingerprint:
			be.PutUint16(h[2:], uint16(p-12))
			crc := fingerprint(r.buf[:p])
			if attr.(uint32) != crc {
				return nil, ErrIncorrectFingerprint
			}
			d = nil
		}
		p += s
	}
	if unk != nil {
		return m, &ErrUnknownAttrs{unk}
	}
	return m, nil
}
