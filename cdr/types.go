// Package cdr implements the Common Data Representation (CDR) encoding and
// decoding as specified by the OMG XCDR2 specification for DDS (Data
// Distribution Service). XCDR2 enforces a maximum alignment of 4 bytes.
package cdr

import "encoding/binary"

// ByteOrder represents the byte order used for CDR encoding.
type ByteOrder = binary.ByteOrder

var (
	// LittleEndian is the little-endian byte order.
	LittleEndian ByteOrder = binary.LittleEndian
	// BigEndian is the big-endian byte order.
	BigEndian ByteOrder = binary.BigEndian
)

// ExtensibilityKind describes the extensibility model of a type in XCDR2.
type ExtensibilityKind int

const (
	// FINAL types have a fixed layout and cannot be extended.
	FINAL ExtensibilityKind = iota
	// APPENDABLE types use a DHEADER to allow appending new members.
	APPENDABLE
	// MUTABLE types use EMHEADER/PL_CDR2 to allow reordering and optional members.
	MUTABLE
)

// EncapsulationKind identifies the encapsulation scheme in the RTPS
// serialized payload header.
type EncapsulationKind uint16

const (
	// CDR2_BE is XCDR2 plain CDR big-endian (FINAL types).
	CDR2_BE EncapsulationKind = 0x0006
	// CDR2_LE is XCDR2 plain CDR little-endian (FINAL types).
	CDR2_LE EncapsulationKind = 0x0007
	// DELIMITED_CDR2_BE is XCDR2 delimited CDR big-endian (APPENDABLE types).
	DELIMITED_CDR2_BE EncapsulationKind = 0x0008
	// DELIMITED_CDR2_LE is XCDR2 delimited CDR little-endian (APPENDABLE types).
	DELIMITED_CDR2_LE EncapsulationKind = 0x0009
	// PL_CDR2_BE is XCDR2 parameter-list CDR big-endian (MUTABLE types).
	PL_CDR2_BE EncapsulationKind = 0x000a
	// PL_CDR2_LE is XCDR2 parameter-list CDR little-endian (MUTABLE types).
	PL_CDR2_LE EncapsulationKind = 0x000b
)

// ByteOrder returns the byte order implied by this encapsulation kind.
// Per the DDS-RTPS spec, odd values are LE and even values are BE.
func (k EncapsulationKind) ByteOrder() binary.ByteOrder {
	if k&1 == 1 {
		return binary.LittleEndian
	}
	return binary.BigEndian
}

// GetEncapsulationKind returns the EncapsulationKind for the given
// extensibility using the platform's native byte order.
func GetEncapsulationKind(ext ExtensibilityKind) EncapsulationKind {
	le := isNativeLE()
	switch ext {
	case APPENDABLE:
		if le {
			return DELIMITED_CDR2_LE
		}
		return DELIMITED_CDR2_BE
	case MUTABLE:
		if le {
			return PL_CDR2_LE
		}
		return PL_CDR2_BE
	default: // FINAL
		if le {
			return CDR2_LE
		}
		return CDR2_BE
	}
}

// isNativeLE reports whether the platform uses little-endian byte order.
func isNativeLE() bool {
	var buf [2]byte
	binary.NativeEndian.PutUint16(buf[:], 0x0102)
	return buf[0] == 0x02
}

// Marshaler is the interface implemented by types that can serialize
// themselves into CDR format, including the encapsulation header.
type Marshaler interface {
	MarshalCDR() ([]byte, error)
}

// Unmarshaler is the interface implemented by types that can deserialize
// themselves from CDR format, reading the encapsulation header automatically.
type Unmarshaler interface {
	UnmarshalCDR([]byte) error
}

// CDREncoder is the interface implemented by types that can encode
// themselves into an existing Encoder. Used for nested type serialization.
type CDREncoder interface {
	EncodeCDR(enc *Encoder) error
}

// CDRDecoder is the interface implemented by types that can decode
// themselves from an existing Decoder. Used for nested type deserialization.
type CDRDecoder interface {
	DecodeCDR(dec *Decoder) error
}

// MemberID is a 28-bit identifier for a member in PL_CDR2 (MUTABLE) encoding.
type MemberID = uint32

// DDSKeyField describes a single key field for DDS instance discrimination.
// Offset is the byte offset from the start of the CDR payload (after the
// 4-byte encapsulation header). Size is the serialized size in bytes.
type DDSKeyField struct {
	Offset   uint32
	Size     uint32
	TypeHint KeyTypeHint
}

// KeyTypeHint describes the semantic type of a key field.
type KeyTypeHint byte

const (
	KeyOpaque KeyTypeHint = 0x00
	KeyUUID   KeyTypeHint = 0x01
	KeyInt32  KeyTypeHint = 0x02
	KeyInt64  KeyTypeHint = 0x03
	KeyString KeyTypeHint = 0x04
)

// Keyed is the interface implemented by DDS types that declare whether
// they have key fields. Types with IsKeyed() == true are treated as
// keyed topics in DDS (WITH_KEY).
type Keyed interface {
	IsKeyed() bool
}

// KeyFieldExtractor is the interface for DDS types that can extract key field
// locations from serialized CDR data. The returned offsets are relative to
// the CDR payload start (after the 4-byte encapsulation header).
type KeyFieldExtractor interface {
	ExtractKeyFields(data []byte) ([]DDSKeyField, error)
}

// EMFieldSize computes the field content size from an EMHEADER's length code
// and NEXTINT value, per DDS-XTYPES spec 7.4.3.4.2.
//
//	LC 0: 1 byte, LC 1: 2 bytes, LC 2: 4 bytes, LC 3: 8 bytes
//	LC 4..7: size is encoded in NEXTINT
func EMFieldSize(lc uint8, nextInt uint32) uint32 {
	switch lc {
	case 0:
		return 1
	case 1:
		return 2
	case 2:
		return 4
	case 3:
		return 8
	default:
		return nextInt
	}
}
