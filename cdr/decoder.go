package cdr

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Decoder deserializes values from CDR/XCDR2 encoded data.
type Decoder struct {
	data  []byte
	pos   int
	order binary.ByteOrder
}

// NewDecoder creates a Decoder that reads the 4-byte encapsulation header
// from data to determine the byte order, then positions the read cursor
// at the start of the CDR payload.
func NewDecoder(data []byte) (*Decoder, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("cdr: data too short for encapsulation header (%d bytes)", len(data))
	}
	kind := EncapsulationKind(binary.BigEndian.Uint16(data[:2]))
	switch kind {
	case CDR2_LE, CDR2_BE, DELIMITED_CDR2_LE, DELIMITED_CDR2_BE, PL_CDR2_LE, PL_CDR2_BE:
	default:
		return nil, fmt.Errorf("cdr: unknown encapsulation kind 0x%04x", uint16(kind))
	}
	return &Decoder{data: data, pos: 4, order: kind.ByteOrder()}, nil
}

// NewRawDecoder creates a Decoder that reads from data using the given byte
// order without expecting an encapsulation header.
func NewRawDecoder(data []byte, order binary.ByteOrder) *Decoder {
	return &Decoder{data: data, order: order}
}

// align advances the position to the next n-byte boundary. XCDR2 caps
// alignment at 4 bytes.
func (d *Decoder) align(n int) {
	if n > 4 {
		n = 4
	}
	rem := d.pos % n
	if rem != 0 {
		d.pos += n - rem
	}
}

// checkRemaining returns an error if fewer than n bytes remain.
func (d *Decoder) checkRemaining(n int) error {
	if d.pos+n > len(d.data) {
		return fmt.Errorf("cdr: need %d bytes at offset %d, but only %d remain",
			n, d.pos, len(d.data)-d.pos)
	}
	return nil
}

// ReadBool reads a single byte and returns true if it is non-zero.
func (d *Decoder) ReadBool() (bool, error) {
	if err := d.checkRemaining(1); err != nil {
		return false, err
	}
	v := d.data[d.pos] != 0
	d.pos++
	return v, nil
}

// ReadByte reads a single byte.
func (d *Decoder) ReadByte() (byte, error) {
	if err := d.checkRemaining(1); err != nil {
		return 0, err
	}
	v := d.data[d.pos]
	d.pos++
	return v, nil
}

// ReadInt8 reads a signed 8-bit integer.
func (d *Decoder) ReadInt8() (int8, error) {
	b, err := d.ReadByte()
	return int8(b), err
}

// ReadUint8 reads an unsigned 8-bit integer.
func (d *Decoder) ReadUint8() (uint8, error) {
	return d.ReadByte()
}

// ReadInt16 reads a signed 16-bit integer with 2-byte alignment.
func (d *Decoder) ReadInt16() (int16, error) {
	d.align(2)
	if err := d.checkRemaining(2); err != nil {
		return 0, err
	}
	v := d.order.Uint16(d.data[d.pos:])
	d.pos += 2
	return int16(v), nil
}

// ReadUint16 reads an unsigned 16-bit integer with 2-byte alignment.
func (d *Decoder) ReadUint16() (uint16, error) {
	d.align(2)
	if err := d.checkRemaining(2); err != nil {
		return 0, err
	}
	v := d.order.Uint16(d.data[d.pos:])
	d.pos += 2
	return v, nil
}

// ReadInt32 reads a signed 32-bit integer with 4-byte alignment.
func (d *Decoder) ReadInt32() (int32, error) {
	d.align(4)
	if err := d.checkRemaining(4); err != nil {
		return 0, err
	}
	v := d.order.Uint32(d.data[d.pos:])
	d.pos += 4
	return int32(v), nil
}

// ReadUint32 reads an unsigned 32-bit integer with 4-byte alignment.
func (d *Decoder) ReadUint32() (uint32, error) {
	d.align(4)
	if err := d.checkRemaining(4); err != nil {
		return 0, err
	}
	v := d.order.Uint32(d.data[d.pos:])
	d.pos += 4
	return v, nil
}

// ReadInt64 reads a signed 64-bit integer. XCDR2 caps alignment at 4.
func (d *Decoder) ReadInt64() (int64, error) {
	d.align(4)
	if err := d.checkRemaining(8); err != nil {
		return 0, err
	}
	v := d.order.Uint64(d.data[d.pos:])
	d.pos += 8
	return int64(v), nil
}

// ReadUint64 reads an unsigned 64-bit integer. XCDR2 caps alignment at 4.
func (d *Decoder) ReadUint64() (uint64, error) {
	d.align(4)
	if err := d.checkRemaining(8); err != nil {
		return 0, err
	}
	v := d.order.Uint64(d.data[d.pos:])
	d.pos += 8
	return v, nil
}

// ReadFloat32 reads a 32-bit IEEE 754 float with 4-byte alignment.
func (d *Decoder) ReadFloat32() (float32, error) {
	u, err := d.ReadUint32()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(u), nil
}

// ReadFloat64 reads a 64-bit IEEE 754 float. XCDR2 caps alignment at 4.
func (d *Decoder) ReadFloat64() (float64, error) {
	u, err := d.ReadUint64()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(u), nil
}

// ReadString reads a CDR string: a 4-byte aligned uint32 length (including
// the null terminator), followed by the string bytes and a null byte.
func (d *Decoder) ReadString() (string, error) {
	length, err := d.ReadUint32()
	if err != nil {
		return "", fmt.Errorf("cdr: reading string length: %w", err)
	}
	if length == 0 {
		return "", nil
	}
	if err := d.checkRemaining(int(length)); err != nil {
		return "", fmt.Errorf("cdr: reading string data: %w", err)
	}
	// length includes the null terminator; exclude it from the Go string.
	s := string(d.data[d.pos : d.pos+int(length)-1])
	d.pos += int(length)
	return s, nil
}

// ReadDHeader reads the DHEADER uint32 used by APPENDABLE types and returns
// the serialized size of the delimited body.
func (d *Decoder) ReadDHeader() (uint32, error) {
	return d.ReadUint32()
}

// ReadEMHeader reads an EMHEADER (and optional NEXTINT) used by MUTABLE
// types. It returns the must-understand flag, the length code, the member
// ID, and the NEXTINT value (zero when LC < 4).
//
// If the raw header equals PLCDRSentinelHeader the function returns
// immediately with the sentinel member ID and LC=7.
func (d *Decoder) ReadEMHeader() (mustUnderstand bool, lc uint8, memberID uint32, nextInt uint32, err error) {
	raw, err := d.ReadUint32()
	if err != nil {
		return false, 0, 0, 0, fmt.Errorf("cdr: reading EMHEADER: %w", err)
	}

	if raw == PLCDRSentinelHeader {
		// Sentinel: M=0, LC=7, memberID carries the sentinel pattern.
		return false, 7, raw & 0x0FFFFFFF, 0, nil
	}

	mustUnderstand = (raw >> 31) != 0
	lc = uint8((raw >> 28) & 0x7)
	memberID = raw & 0x0FFFFFFF

	if lc >= 4 && lc <= 6 {
		nextInt, err = d.ReadUint32()
		if err != nil {
			return false, 0, 0, 0, fmt.Errorf("cdr: reading NEXTINT for EMHEADER: %w", err)
		}
	}

	return mustUnderstand, lc, memberID, nextInt, nil
}

// IsSentinel reports whether the given raw EMHEADER value is the PL_CDR2
// sentinel that terminates a mutable member list.
func IsSentinel(rawHeader uint32) bool {
	return rawHeader == PLCDRSentinelHeader
}

// Skip advances the read position by n bytes. It returns an error if
// there are not enough bytes remaining.
func (d *Decoder) Skip(n int) error {
	if err := d.checkRemaining(n); err != nil {
		return fmt.Errorf("cdr: skip %d bytes: %w", n, err)
	}
	d.pos += n
	return nil
}

// Remaining returns the number of unread bytes.
func (d *Decoder) Remaining() int {
	r := len(d.data) - d.pos
	if r < 0 {
		return 0
	}
	return r
}

// Pos returns the current read position.
func (d *Decoder) Pos() int {
	return d.pos
}
