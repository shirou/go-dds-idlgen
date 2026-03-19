package cdr

import (
	"bytes"
	"encoding/binary"
	"math"
)

// Encoder serializes values into CDR/XCDR2 format.
type Encoder struct {
	buf   bytes.Buffer
	pos   int
	order binary.ByteOrder
}

// NewEncoder returns an Encoder that writes the 4-byte encapsulation header
// for the given kind and then serializes subsequent values in the
// corresponding byte order.
func NewEncoder(kind EncapsulationKind) *Encoder {
	e := &Encoder{order: kind.ByteOrder()}
	// Encapsulation header: 2-byte kind (big-endian) + 2-byte options (zero).
	var hdr [4]byte
	binary.BigEndian.PutUint16(hdr[:2], uint16(kind))
	e.buf.Write(hdr[:])
	e.pos = 4
	return e
}

// NewRawEncoder returns an Encoder that writes data in the given byte order
// without an encapsulation header.
func NewRawEncoder(order binary.ByteOrder) *Encoder {
	return &Encoder{order: order}
}

// align pads the buffer so that the next write starts at an n-byte boundary.
// XCDR2 caps alignment at 4 bytes even for 8-byte types.
func (e *Encoder) align(n int) {
	if n > 4 {
		n = 4
	}
	rem := e.pos % n
	if rem != 0 {
		pad := n - rem
		for range pad {
			e.buf.WriteByte(0)
		}
		e.pos += pad
	}
}

// WriteBool writes a boolean as a single byte (0 or 1).
func (e *Encoder) WriteBool(v bool) error {
	var b byte
	if v {
		b = 1
	}
	e.buf.WriteByte(b)
	e.pos++
	return nil
}

// WriteByte writes a single byte.
func (e *Encoder) WriteByte(v byte) error {
	e.buf.WriteByte(v)
	e.pos++
	return nil
}

// WriteInt8 writes a signed 8-bit integer.
func (e *Encoder) WriteInt8(v int8) error {
	e.buf.WriteByte(byte(v))
	e.pos++
	return nil
}

// WriteUint8 writes an unsigned 8-bit integer.
func (e *Encoder) WriteUint8(v uint8) error {
	e.buf.WriteByte(v)
	e.pos++
	return nil
}

// WriteInt16 writes a signed 16-bit integer with 2-byte alignment.
func (e *Encoder) WriteInt16(v int16) error {
	e.align(2)
	var tmp [2]byte
	e.order.PutUint16(tmp[:], uint16(v))
	e.buf.Write(tmp[:])
	e.pos += 2
	return nil
}

// WriteUint16 writes an unsigned 16-bit integer with 2-byte alignment.
func (e *Encoder) WriteUint16(v uint16) error {
	e.align(2)
	var tmp [2]byte
	e.order.PutUint16(tmp[:], v)
	e.buf.Write(tmp[:])
	e.pos += 2
	return nil
}

// WriteInt32 writes a signed 32-bit integer with 4-byte alignment.
func (e *Encoder) WriteInt32(v int32) error {
	e.align(4)
	var tmp [4]byte
	e.order.PutUint32(tmp[:], uint32(v))
	e.buf.Write(tmp[:])
	e.pos += 4
	return nil
}

// WriteUint32 writes an unsigned 32-bit integer with 4-byte alignment.
func (e *Encoder) WriteUint32(v uint32) error {
	e.align(4)
	var tmp [4]byte
	e.order.PutUint32(tmp[:], v)
	e.buf.Write(tmp[:])
	e.pos += 4
	return nil
}

// WriteInt64 writes a signed 64-bit integer. XCDR2 caps alignment at 4.
func (e *Encoder) WriteInt64(v int64) error {
	e.align(4)
	var tmp [8]byte
	e.order.PutUint64(tmp[:], uint64(v))
	e.buf.Write(tmp[:])
	e.pos += 8
	return nil
}

// WriteUint64 writes an unsigned 64-bit integer. XCDR2 caps alignment at 4.
func (e *Encoder) WriteUint64(v uint64) error {
	e.align(4)
	var tmp [8]byte
	e.order.PutUint64(tmp[:], v)
	e.buf.Write(tmp[:])
	e.pos += 8
	return nil
}

// WriteFloat32 writes a 32-bit IEEE 754 float with 4-byte alignment.
func (e *Encoder) WriteFloat32(v float32) error {
	return e.WriteUint32(math.Float32bits(v))
}

// WriteFloat64 writes a 64-bit IEEE 754 float. XCDR2 caps alignment at 4.
func (e *Encoder) WriteFloat64(v float64) error {
	return e.WriteUint64(math.Float64bits(v))
}

// WriteString writes a CDR string: 4-byte aligned uint32 length (including
// the null terminator), followed by the string bytes and a null byte.
func (e *Encoder) WriteString(v string) error {
	length := uint32(len(v) + 1) // include null terminator
	if err := e.WriteUint32(length); err != nil {
		return err
	}
	e.buf.WriteString(v)
	e.buf.WriteByte(0) // null terminator
	e.pos += len(v) + 1
	return nil
}

// BeginDHeader writes a placeholder uint32 for the DHEADER (used by
// APPENDABLE types) and returns the buffer position of the placeholder so
// that FinishDHeader can patch it later.
func (e *Encoder) BeginDHeader() int {
	e.align(4)
	start := e.pos
	var tmp [4]byte
	e.buf.Write(tmp[:]) // placeholder
	e.pos += 4
	return start
}

// FinishDHeader patches the DHEADER at the given start position with the
// number of bytes written since the placeholder.
func (e *Encoder) FinishDHeader(start int) {
	size := uint32(e.pos - start - 4)
	raw := e.buf.Bytes()
	e.order.PutUint32(raw[start:start+4], size)
}

// WriteEMHeader writes an EMHEADER (and optional NEXTINT) for a MUTABLE
// type member.
//
// The EMHEADER is a 32-bit value laid out as:
//
//	bit  31:    must_understand flag (M)
//	bits 30-28: length code (LC)
//	bits 27-0:  member ID
//
// LC values encode the serialized member size:
//
//	0 = 1 byte, 1 = 2 bytes, 2 = 4 bytes, 3 = 8 bytes
//	4 = NEXTINT (a following uint32 holds the actual size)
//	5 = 4 + NEXTINT,  6 = 4 + NEXTINT * 4,  7 = 4 + NEXTINT * 8
//
// If the data size matches one of the fixed LC values (1, 2, 4, 8), the
// compact form is used. Otherwise LC=4 is used with an explicit NEXTINT.
func (e *Encoder) WriteEMHeader(mustUnderstand bool, memberID uint32, dataSize int) error {
	var lc uint32
	needNextInt := false

	switch dataSize {
	case 1:
		lc = 0
	case 2:
		lc = 1
	case 4:
		lc = 2
	case 8:
		lc = 3
	default:
		lc = 4
		needNextInt = true
	}

	var m uint32
	if mustUnderstand {
		m = 1
	}

	header := (m << 31) | (lc << 28) | (memberID & 0x0FFFFFFF)
	if err := e.WriteUint32(header); err != nil {
		return err
	}

	if needNextInt {
		if err := e.WriteUint32(uint32(dataSize)); err != nil {
			return err
		}
	}

	return nil
}

// ByteOrder returns the byte order used by this encoder.
func (e *Encoder) ByteOrder() binary.ByteOrder {
	return e.order
}

// Write writes raw bytes to the encoder buffer.
func (e *Encoder) Write(p []byte) (int, error) {
	n, err := e.buf.Write(p)
	e.pos += n
	return n, err
}

// Bytes returns the encoded byte slice.
func (e *Encoder) Bytes() []byte {
	return e.buf.Bytes()
}

// Reset clears the buffer and resets the position counter.
func (e *Encoder) Reset() {
	e.buf.Reset()
	e.pos = 0
}
