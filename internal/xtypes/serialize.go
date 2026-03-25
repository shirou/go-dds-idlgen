// All Encoder methods write to a bytes.Buffer and never return errors.
//nolint:errcheck
package xtypes

import (
	"encoding/binary"

	"github.com/shirou/go-dds-idlgen/cdr"
)

// ---------------------------------------------------------------------------
// TypeIdentifier — FINAL union (uint8 discriminator)
// ---------------------------------------------------------------------------

func serializeTypeIdentifier(enc *cdr.Encoder, tid *TypeIdentifier) {
	enc.WriteByte(tid.Discriminator)

	switch d := tid.Discriminator; {
	case isPrimitiveKind(d):
		// No payload for primitive types.

	case d == EK_MINIMAL || d == EK_COMPLETE:
		enc.Write(tid.Hash[:])

	case d == TI_STRING8_SMALL || d == TI_STRING16_SMALL:
		enc.WriteUint8(tid.StringSBound)

	case d == TI_STRING8_LARGE || d == TI_STRING16_LARGE:
		enc.WriteUint32(tid.StringLBound)

	case d == TI_PLAIN_SEQUENCE_SMALL:
		serializePlainCollectionHeader(enc, &tid.SeqSHeader)
		enc.WriteUint8(tid.SeqSBound)
		serializeTypeIdentifier(enc, tid.SeqSElemID)

	case d == TI_PLAIN_SEQUENCE_LARGE:
		serializePlainCollectionHeader(enc, &tid.SeqLHeader)
		enc.WriteUint32(tid.SeqLBound)
		serializeTypeIdentifier(enc, tid.SeqLElemID)

	case d == TI_PLAIN_ARRAY_SMALL:
		serializePlainCollectionHeader(enc, &tid.ArrSHeader)
		enc.WriteUint32(uint32(len(tid.ArrSBounds)))
		for _, b := range tid.ArrSBounds {
			enc.WriteUint8(b)
		}
		serializeTypeIdentifier(enc, tid.ArrSElemID)

	case d == TI_PLAIN_ARRAY_LARGE:
		serializePlainCollectionHeader(enc, &tid.ArrLHeader)
		enc.WriteUint32(uint32(len(tid.ArrLBounds)))
		for _, b := range tid.ArrLBounds {
			enc.WriteUint32(b)
		}
		serializeTypeIdentifier(enc, tid.ArrLElemID)
	}
}

func serializePlainCollectionHeader(enc *cdr.Encoder, h *PlainCollectionHeader) {
	enc.WriteByte(h.EquivKind)
	enc.WriteUint16(h.ElementFlags)
}

// ---------------------------------------------------------------------------
// Minimal TypeObject serialization — matching Cyclone DDS layout
//
// Cyclone DDS ops analysis:
//   TypeObject:             DLC (DHEADER wrapper)
//   MinimalTypeObject:      FINAL union (no DLC)
//   MinimalStructType:      NO DLC (flat)
//   MinimalStructHeader:    DLC (DHEADER)
//   MinimalStructMember:    DLC (DHEADER)
//   CommonStructMember:     NO DLC (flat)
//   MinimalMemberDetail:    NO DLC (flat)
//   MinimalTypeDetail:      empty (RTS)
//   MinimalUnionType:       NO DLC (flat)
//   MinimalUnionHeader:     DLC (DHEADER, empty)
//   MinimalDiscriminatorMember: DLC (DHEADER)
//   MinimalUnionMember:     DLC (DHEADER)
//   CommonUnionMember:      NO DLC (flat)
//   MinimalEnumType:        NO DLC (flat)
//   MinimalEnumHeader:      DLC (DHEADER)
//   MinimalEnumLiteral:     DLC (DHEADER)
//   CommonEnumLiteral:      DLC (DHEADER) — Cyclone DDS specific
//
// Sequences of DLC struct elements get their own DHEADER wrapper.
// ---------------------------------------------------------------------------

// serializeTypeObject writes a TypeObject (DLC → DHEADER) wrapping a
// MinimalTypeObject or CompleteTypeObject.
func serializeTypeObject(enc *cdr.Encoder, disc byte, writeContent func()) {
	dh := enc.BeginDHeader()
	enc.WriteByte(disc) // TypeObject discriminator (EK_MINIMAL or EK_COMPLETE)
	writeContent()
	enc.FinishDHeader(dh)
}

// serializeMinimalTypeObject writes the MinimalTypeObject FINAL union body.
func serializeMinimalTypeObject(enc *cdr.Encoder, obj *MinimalTypeObject) {
	enc.WriteByte(obj.Kind)
	switch obj.Kind {
	case TOK_STRUCTURE:
		serializeMinimalStructType(enc, obj.StructType)
	case TOK_UNION:
		serializeMinimalUnionType(enc, obj.UnionType)
	case TOK_ENUM:
		serializeMinimalEnumType(enc, obj.EnumType)
	}
}

// serializeMinimalStructType writes MinimalStructType (NO DLC, flat).
func serializeMinimalStructType(enc *cdr.Encoder, st *MinimalStructType) {
	enc.WriteUint16(st.StructFlags)

	// MinimalStructHeader (DLC → DHEADER)
	dh := enc.BeginDHeader()
	serializeTypeIdentifier(enc, &st.Header.BaseType)
	// MinimalTypeDetail: empty
	enc.FinishDHeader(dh)

	// member_seq: sequence<MinimalStructMember> — wrapped in DHEADER
	seqDh := enc.BeginDHeader()
	enc.WriteUint32(uint32(len(st.Members)))
	for i := range st.Members {
		serializeMinimalStructMember(enc, &st.Members[i])
	}
	enc.FinishDHeader(seqDh)
}

// serializeMinimalStructMember writes MinimalStructMember (DLC → DHEADER).
func serializeMinimalStructMember(enc *cdr.Encoder, m *MinimalStructMember) {
	dh := enc.BeginDHeader()
	enc.WriteUint32(m.Common.MemberID)
	enc.WriteUint16(m.Common.MemberFlags)
	serializeTypeIdentifier(enc, &m.Common.TypeID)
	enc.Write(m.Detail.NameHash[:])
	enc.FinishDHeader(dh)
}

// serializeMinimalUnionType writes MinimalUnionType (NO DLC, flat).
func serializeMinimalUnionType(enc *cdr.Encoder, ut *MinimalUnionType) {
	enc.WriteUint16(ut.UnionFlags)

	// MinimalUnionHeader (DLC → DHEADER, empty content)
	uhDh := enc.BeginDHeader()
	// MinimalTypeDetail: empty
	enc.FinishDHeader(uhDh)

	// MinimalDiscriminatorMember (DLC → DHEADER)
	ddDh := enc.BeginDHeader()
	enc.WriteUint16(ut.DiscCommon.MemberFlags)
	serializeTypeIdentifier(enc, &ut.DiscCommon.TypeID)
	enc.FinishDHeader(ddDh)

	// member_seq: sequence<MinimalUnionMember> — wrapped in DHEADER
	seqDh := enc.BeginDHeader()
	enc.WriteUint32(uint32(len(ut.Members)))
	for i := range ut.Members {
		serializeMinimalUnionMember(enc, &ut.Members[i])
	}
	enc.FinishDHeader(seqDh)
}

// serializeMinimalUnionMember writes MinimalUnionMember (DLC → DHEADER).
func serializeMinimalUnionMember(enc *cdr.Encoder, m *MinimalUnionMember) {
	dh := enc.BeginDHeader()
	enc.WriteUint32(m.Common.MemberID)
	enc.WriteUint16(m.Common.MemberFlags)
	serializeTypeIdentifier(enc, &m.Common.TypeID)
	enc.WriteUint32(m.Common.LabelCount)
	for _, label := range m.Common.Labels {
		enc.WriteInt32(label)
	}
	enc.Write(m.Detail.NameHash[:])
	enc.FinishDHeader(dh)
}

// serializeMinimalEnumType writes MinimalEnumType (NO DLC, flat).
func serializeMinimalEnumType(enc *cdr.Encoder, et *MinimalEnumType) {
	enc.WriteUint16(et.EnumFlags)

	// MinimalEnumHeader (DLC → DHEADER)
	ehDh := enc.BeginDHeader()
	enc.WriteUint16(et.Header.BitBound)
	enc.FinishDHeader(ehDh)

	// literal_seq: sequence<MinimalEnumLiteral> — wrapped in DHEADER
	seqDh := enc.BeginDHeader()
	enc.WriteUint32(uint32(len(et.Literals)))
	for i := range et.Literals {
		serializeMinimalEnumLiteral(enc, &et.Literals[i])
	}
	enc.FinishDHeader(seqDh)
}

// serializeMinimalEnumLiteral writes MinimalEnumLiteral (DLC → DHEADER).
func serializeMinimalEnumLiteral(enc *cdr.Encoder, lit *MinimalEnumLiteral) {
	dh := enc.BeginDHeader()
	// CommonEnumLiteral — Cyclone DDS has DLC for this
	serializeCommonEnumLiteral(enc, &lit.Common)
	enc.Write(lit.Detail.NameHash[:])
	enc.FinishDHeader(dh)
}

func serializeCommonEnumLiteral(enc *cdr.Encoder, c *CommonEnumLiteral) {
	dh := enc.BeginDHeader()
	enc.WriteInt32(int32(c.Value))
	enc.WriteUint16(c.Flags)
	enc.FinishDHeader(dh)
}

// ---------------------------------------------------------------------------
// Complete TypeObject serialization — same structure as minimal but with names
// ---------------------------------------------------------------------------

func serializeCompleteTypeObject(enc *cdr.Encoder, obj *CompleteTypeObject) {
	enc.WriteByte(obj.Kind)
	switch obj.Kind {
	case TOK_STRUCTURE:
		serializeCompleteStructType(enc, obj.StructType)
	case TOK_UNION:
		serializeCompleteUnionType(enc, obj.UnionType)
	case TOK_ENUM:
		serializeCompleteEnumType(enc, obj.EnumType)
	}
}

// serializeCompleteStructType writes CompleteStructType (NO DLC, flat).
func serializeCompleteStructType(enc *cdr.Encoder, st *CompleteStructType) {
	enc.WriteUint16(st.StructFlags)

	// CompleteStructHeader (DLC → DHEADER)
	dh := enc.BeginDHeader()
	serializeTypeIdentifier(enc, &st.Header.BaseType)
	serializeCompleteTypeDetail(enc, &st.Header.Detail)
	enc.FinishDHeader(dh)

	// member_seq wrapped in DHEADER
	seqDh := enc.BeginDHeader()
	enc.WriteUint32(uint32(len(st.Members)))
	for i := range st.Members {
		serializeCompleteStructMember(enc, &st.Members[i])
	}
	enc.FinishDHeader(seqDh)
}

func serializeCompleteTypeDetail(enc *cdr.Encoder, d *CompleteTypeDetail) {
	enc.WriteBool(false) // ann_builtin absent
	enc.WriteBool(false) // ann_custom absent
	enc.WriteString(d.TypeName)
}

func serializeCompleteStructMember(enc *cdr.Encoder, m *CompleteStructMember) {
	dh := enc.BeginDHeader()
	enc.WriteUint32(m.Common.MemberID)
	enc.WriteUint16(m.Common.MemberFlags)
	serializeTypeIdentifier(enc, &m.Common.TypeID)
	serializeCompleteMemberDetail(enc, &m.Detail)
	enc.FinishDHeader(dh)
}

func serializeCompleteMemberDetail(enc *cdr.Encoder, d *CompleteMemberDetail) {
	enc.WriteString(d.Name)
	enc.WriteBool(false) // ann_builtin absent
	enc.WriteBool(false) // ann_custom absent
}

func serializeCompleteUnionType(enc *cdr.Encoder, ut *CompleteUnionType) {
	enc.WriteUint16(ut.UnionFlags)

	// CompleteUnionHeader (DLC → DHEADER)
	uhDh := enc.BeginDHeader()
	serializeCompleteTypeDetail(enc, &ut.Header.Detail)
	enc.FinishDHeader(uhDh)

	// CompleteDiscriminatorMember (DLC → DHEADER)
	ddDh := enc.BeginDHeader()
	enc.WriteUint16(ut.DiscMember.Common.MemberFlags)
	serializeTypeIdentifier(enc, &ut.DiscMember.Common.TypeID)
	enc.WriteBool(false) // ann_builtin
	enc.WriteBool(false) // ann_custom
	enc.FinishDHeader(ddDh)

	// member_seq wrapped in DHEADER
	seqDh := enc.BeginDHeader()
	enc.WriteUint32(uint32(len(ut.Members)))
	for i := range ut.Members {
		serializeCompleteUnionMember(enc, &ut.Members[i])
	}
	enc.FinishDHeader(seqDh)
}

func serializeCompleteUnionMember(enc *cdr.Encoder, m *CompleteUnionMember) {
	dh := enc.BeginDHeader()
	enc.WriteUint32(m.Common.MemberID)
	enc.WriteUint16(m.Common.MemberFlags)
	serializeTypeIdentifier(enc, &m.Common.TypeID)
	enc.WriteUint32(m.Common.LabelCount)
	for _, label := range m.Common.Labels {
		enc.WriteInt32(label)
	}
	serializeCompleteMemberDetail(enc, &m.Detail)
	enc.FinishDHeader(dh)
}

func serializeCompleteEnumType(enc *cdr.Encoder, et *CompleteEnumType) {
	enc.WriteUint16(et.EnumFlags)

	// CompleteEnumHeader (DLC → DHEADER)
	ehDh := enc.BeginDHeader()
	enc.WriteUint16(et.Header.BitBound)
	serializeCompleteTypeDetail(enc, &et.Header.Detail)
	enc.FinishDHeader(ehDh)

	// literal_seq wrapped in DHEADER
	seqDh := enc.BeginDHeader()
	enc.WriteUint32(uint32(len(et.Literals)))
	for i := range et.Literals {
		serializeCompleteEnumLiteral(enc, &et.Literals[i])
	}
	enc.FinishDHeader(seqDh)
}

func serializeCompleteEnumLiteral(enc *cdr.Encoder, lit *CompleteEnumLiteral) {
	dh := enc.BeginDHeader()
	serializeCommonEnumLiteral(enc, &lit.Common)
	serializeCompleteMemberDetail(enc, &lit.Detail)
	enc.FinishDHeader(dh)
}

// ---------------------------------------------------------------------------
// TypeInformation — MUTABLE (DHEADER + EMHEADER per member)
// ---------------------------------------------------------------------------

const (
	memberIDMinimal  uint32 = 0x1001
	memberIDComplete uint32 = 0x1002
)

func serializeTypeInformation(enc *cdr.Encoder, ti *TypeInformation) {
	outerDh := enc.BeginDHeader()
	writeMutableMember(enc, memberIDMinimal, func() {
		serializeTypeIdWithDeps(enc, &ti.Minimal)
	})
	writeMutableMember(enc, memberIDComplete, func() {
		serializeTypeIdWithDeps(enc, &ti.Complete)
	})
	enc.FinishDHeader(outerDh)
}

func writeMutableMember(enc *cdr.Encoder, memberID uint32, serialize func()) {
	emheader := uint32(4<<28) | (memberID & 0x0FFFFFFF)
	enc.WriteUint32(emheader)
	nextInt := enc.BeginDHeader()
	serialize()
	enc.FinishDHeader(nextInt)
}

func serializeTypeIdWithDeps(enc *cdr.Encoder, s *TypeIdWithSizeSeq) {
	dh := enc.BeginDHeader()
	serializeTypeIdWithSize(enc, &s.TypeIDWithSize)
	enc.WriteInt32(int32(len(s.DependentTypeIDs)))
	seqDh := enc.BeginDHeader()
	enc.WriteUint32(uint32(len(s.DependentTypeIDs)))
	for i := range s.DependentTypeIDs {
		serializeTypeIdWithSize(enc, &s.DependentTypeIDs[i])
	}
	enc.FinishDHeader(seqDh)
	enc.FinishDHeader(dh)
}

func serializeTypeIdWithSize(enc *cdr.Encoder, ts *TypeIdWithSize) {
	dh := enc.BeginDHeader()
	serializeTypeIdentifier(enc, &ts.TypeID)
	enc.WriteUint32(ts.TypeObjectSerializedSize)
	enc.FinishDHeader(dh)
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// SerializeTypeObject serializes a MinimalTypeObject wrapped in a TypeObject
// (with DLC DHEADER) using XCDR2 LE. This matches Cyclone DDS hash computation.
func SerializeTypeObject(obj *MinimalTypeObject) []byte {
	enc := cdr.NewRawEncoder(binary.LittleEndian)
	serializeTypeObject(enc, EK_MINIMAL, func() {
		serializeMinimalTypeObject(enc, obj)
	})
	return enc.Bytes()
}

// SerializeCompleteTypeObject serializes a CompleteTypeObject wrapped in a TypeObject
// (with DLC DHEADER) using XCDR2 LE.
func SerializeCompleteTypeObject(obj *CompleteTypeObject) []byte {
	enc := cdr.NewRawEncoder(binary.LittleEndian)
	serializeTypeObject(enc, EK_COMPLETE, func() {
		serializeCompleteTypeObject(enc, obj)
	})
	return enc.Bytes()
}

// SerializeTypeInformation serializes a TypeInformation to bytes using XCDR2 LE.
func SerializeTypeInformation(ti *TypeInformation) []byte {
	enc := cdr.NewRawEncoder(binary.LittleEndian)
	serializeTypeInformation(enc, ti)
	return enc.Bytes()
}

func isPrimitiveKind(k byte) bool {
	switch k {
	case TK_NONE, TK_BOOLEAN, TK_BYTE,
		TK_INT16, TK_INT32, TK_INT64,
		TK_UINT16, TK_UINT32, TK_UINT64,
		TK_FLOAT32, TK_FLOAT64, TK_FLOAT128,
		TK_INT8, TK_UINT8, TK_CHAR8, TK_CHAR16:
		return true
	}
	return false
}
