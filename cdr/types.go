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
	// CDR2_LE is XCDR2 plain CDR little-endian (FINAL types).
	CDR2_LE EncapsulationKind = 0x0006
	// CDR2_BE is XCDR2 plain CDR big-endian (FINAL types).
	CDR2_BE EncapsulationKind = 0x0007
	// DELIMITED_CDR2_LE is XCDR2 delimited CDR little-endian (APPENDABLE types).
	DELIMITED_CDR2_LE EncapsulationKind = 0x0008
	// DELIMITED_CDR2_BE is XCDR2 delimited CDR big-endian (APPENDABLE types).
	DELIMITED_CDR2_BE EncapsulationKind = 0x0009
	// PL_CDR2_LE is XCDR2 parameter-list CDR little-endian (MUTABLE types).
	PL_CDR2_LE EncapsulationKind = 0x000a
	// PL_CDR2_BE is XCDR2 parameter-list CDR big-endian (MUTABLE types).
	PL_CDR2_BE EncapsulationKind = 0x000b
)

// MemberID is a 28-bit identifier for a member in PL_CDR2 (MUTABLE) encoding.
type MemberID = uint32

// PLCDRSentinelHeader is the raw 32-bit EMHEADER value that marks the end
// of a PL_CDR2 member list.
const PLCDRSentinelHeader uint32 = 0x7FFF0002
