// Package xtypes implements XTypes TypeInformation computation for DDS type matching.
// This is used at code-generation time to produce serialized TypeInformation byte
// sequences that are embedded in generated Go code.
//
// Based on DDS-XTypes v1.3 (OMG Document formal/2020-02-04), Section 7.3.
package xtypes

// TypeKind identifies the kind of a type in the XTypes type system.
// Values are from dds-xtypes_typeobject.idl.
type TypeKind = byte

// Primitive TypeKind constants.
const (
	TK_NONE     TypeKind = 0x00
	TK_BOOLEAN  TypeKind = 0x01
	TK_BYTE     TypeKind = 0x02
	TK_INT16    TypeKind = 0x03
	TK_INT32    TypeKind = 0x04
	TK_INT64    TypeKind = 0x05
	TK_UINT16   TypeKind = 0x06
	TK_UINT32   TypeKind = 0x07
	TK_UINT64   TypeKind = 0x08
	TK_FLOAT32  TypeKind = 0x09
	TK_FLOAT64  TypeKind = 0x0a
	TK_FLOAT128 TypeKind = 0x0b
	TK_INT8     TypeKind = 0x0c
	TK_UINT8    TypeKind = 0x0d
	TK_CHAR8    TypeKind = 0x10
	TK_CHAR16   TypeKind = 0x11
)

// Constructed type TypeKind constants.
const (
	TK_STRING8    TypeKind = 0x20
	TK_STRING16   TypeKind = 0x21
	TK_ALIAS      TypeKind = 0x30
	TK_ENUM       TypeKind = 0x40
	TK_BITMASK    TypeKind = 0x41
	TK_ANNOTATION TypeKind = 0x50
	TK_STRUCTURE  TypeKind = 0x51
	TK_UNION      TypeKind = 0x52
	TK_BITSET     TypeKind = 0x53
	TK_SEQUENCE   TypeKind = 0x60
	TK_ARRAY      TypeKind = 0x61
	TK_MAP        TypeKind = 0x62
)

// TypeIdentifier discriminator values for parameterized types.
const (
	TI_STRING8_SMALL        byte = 0x70
	TI_STRING8_LARGE        byte = 0x71
	TI_STRING16_SMALL       byte = 0x72
	TI_STRING16_LARGE       byte = 0x73
	TI_PLAIN_SEQUENCE_SMALL byte = 0x80
	TI_PLAIN_SEQUENCE_LARGE byte = 0x81
	TI_PLAIN_ARRAY_SMALL    byte = 0x90
	TI_PLAIN_ARRAY_LARGE    byte = 0x91
	TI_PLAIN_MAP_SMALL      byte = 0xA0
	TI_PLAIN_MAP_LARGE      byte = 0xA1
	TI_STRONGLY_CONNECTED   byte = 0xB0
)

// Equivalence kind discriminators.
const (
	EK_MINIMAL  byte = 0xF1
	EK_COMPLETE byte = 0xF2
	EK_BOTH     byte = 0xF3
)

// EquivalenceHash is a 14-byte hash derived from MD5 of a serialized MinimalTypeObject.
type EquivalenceHash [14]byte

// NameHash is a 4-byte hash of a member/type name, derived from MD5.
type NameHash [4]byte

// TypeIdentifier is a discriminated union identifying a type.
// For primitive types, Discriminator is the TypeKind and Hash is zero.
// For constructed types (struct, union, etc.), Discriminator is EK_MINIMAL
// and Hash contains the equivalence hash.
// For parameterized types (string, sequence, array), Discriminator is the
// TI_xxx constant and the payload varies.
type TypeIdentifier struct {
	Discriminator byte

	// EK_MINIMAL / EK_COMPLETE
	Hash EquivalenceHash

	// TI_STRING8_SMALL / TI_STRING16_SMALL
	StringSBound uint8

	// TI_STRING8_LARGE / TI_STRING16_LARGE
	StringLBound uint32

	// TI_PLAIN_SEQUENCE_SMALL
	SeqSHeader PlainCollectionHeader
	SeqSBound  uint8
	SeqSElemID *TypeIdentifier

	// TI_PLAIN_SEQUENCE_LARGE
	SeqLHeader PlainCollectionHeader
	SeqLBound  uint32
	SeqLElemID *TypeIdentifier

	// TI_PLAIN_ARRAY_SMALL
	ArrSHeader PlainCollectionHeader
	ArrSBounds []uint8
	ArrSElemID *TypeIdentifier

	// TI_PLAIN_ARRAY_LARGE
	ArrLHeader PlainCollectionHeader
	ArrLBounds []uint32
	ArrLElemID *TypeIdentifier
}

// PlainCollectionHeader is the header for plain collection TypeIdentifiers.
type PlainCollectionHeader struct {
	EquivKind    byte   // EK_MINIMAL, EK_COMPLETE, or EK_BOTH
	ElementFlags uint16 // CollectionElementFlag
}

// MemberFlag bit values.
const (
	MemberFlagTryConstruct1    uint16 = 0x0001
	MemberFlagTryConstruct2    uint16 = 0x0002
	MemberFlagIsExternal       uint16 = 0x0004
	MemberFlagIsOptional       uint16 = 0x0008
	MemberFlagIsMustUnderstand uint16 = 0x0010
	MemberFlagIsKey            uint16 = 0x0020
	MemberFlagIsDefault        uint16 = 0x0040
)

// StructTypeFlag bit values (bitmask positions from XTypes spec).
const (
	TypeFlagNone         uint16 = 0x0000
	TypeFlagIsFinal      uint16 = 0x0001 // @position(0)
	TypeFlagIsAppendable uint16 = 0x0002 // @position(1)
	TypeFlagIsMutable    uint16 = 0x0004 // @position(2)
	TypeFlagIsNested     uint16 = 0x0008 // @position(3)
	TypeFlagIsAutoIDHash uint16 = 0x0010 // @position(4)
)

// MinimalStructType represents a struct type in the minimal type representation.
type MinimalStructType struct {
	StructFlags uint16
	Header      MinimalStructHeader
	Members     []MinimalStructMember
}

// MinimalStructHeader is the header of a MinimalStructType.
type MinimalStructHeader struct {
	BaseType TypeIdentifier // TK_NONE if no base type
	// MinimalTypeDetail is empty in minimal representation
}

// MinimalStructMember represents a single member in a MinimalStructType.
type MinimalStructMember struct {
	Common CommonStructMember
	Detail MinimalMemberDetail
}

// CommonStructMember holds the common part of a struct member.
type CommonStructMember struct {
	MemberID    uint32
	MemberFlags uint16
	TypeID      TypeIdentifier
}

// MinimalMemberDetail holds the detail for a member in minimal representation.
type MinimalMemberDetail struct {
	NameHash NameHash
}

// MinimalUnionType represents a union type in the minimal type representation.
type MinimalUnionType struct {
	UnionFlags uint16
	Header     MinimalUnionHeader
	DiscCommon CommonDiscriminatorMember
	Members    []MinimalUnionMember
}

// MinimalUnionHeader is the header of a MinimalUnionType.
type MinimalUnionHeader struct {
	// MinimalTypeDetail is empty in minimal representation
}

// CommonDiscriminatorMember holds the discriminator info of a union.
type CommonDiscriminatorMember struct {
	MemberFlags uint16
	TypeID      TypeIdentifier
}

// MinimalUnionMember represents a single case/member in a MinimalUnionType.
type MinimalUnionMember struct {
	Common CommonUnionMember
	Detail MinimalMemberDetail
}

// CommonUnionMember holds the common part of a union member.
type CommonUnionMember struct {
	MemberID    uint32
	MemberFlags uint16
	TypeID      TypeIdentifier
	LabelCount  uint32
	Labels      []int32 // case label values
}

// MinimalEnumType represents an enum type in the minimal type representation.
type MinimalEnumType struct {
	EnumFlags uint16
	Header    MinimalEnumHeader
	Literals  []MinimalEnumLiteral
}

// MinimalEnumHeader is the header of a MinimalEnumType.
type MinimalEnumHeader struct {
	// CommonEnumHeader
	BitBound uint16
}

// MinimalEnumLiteral represents a single enum value.
type MinimalEnumLiteral struct {
	Common CommonEnumLiteral
	Detail MinimalMemberDetail
}

// CommonEnumLiteral holds the common part of an enum literal.
type CommonEnumLiteral struct {
	Value uint32
	Flags uint16 // EnumLiteralFlag
}

// TypeObjectKind discriminator for MinimalTypeObject union.
const (
	TOK_STRUCTURE byte = TK_STRUCTURE
	TOK_UNION     byte = TK_UNION
	TOK_ENUM      byte = TK_ENUM
)

// MinimalTypeObject is a discriminated union wrapping type-specific objects.
type MinimalTypeObject struct {
	Kind       byte // TOK_STRUCTURE, TOK_UNION, TOK_ENUM
	StructType *MinimalStructType
	UnionType  *MinimalUnionType
	EnumType   *MinimalEnumType
}

// TypeIdWithSize pairs a TypeIdentifier with the serialized size of its TypeObject.
type TypeIdWithSize struct {
	TypeID                   TypeIdentifier
	TypeObjectSerializedSize uint32
}

// TypeInformation is the top-level structure sent during DDS discovery.
type TypeInformation struct {
	Minimal  TypeIdWithSizeSeq
	Complete TypeIdWithSizeSeq // typically empty
}

// TypeIdWithSizeSeq holds the type mapping for one representation (minimal or complete).
// Corresponds to TypeIdentifierWithDependencies in the XTypes spec.
type TypeIdWithSizeSeq struct {
	TypeIDWithSize   TypeIdWithSize
	DependentTypeIDs []TypeIdWithSize
}

// CompleteTypeObject is a discriminated union wrapping complete type-specific objects.
type CompleteTypeObject struct {
	Kind       byte // TOK_STRUCTURE, TOK_UNION, TOK_ENUM
	StructType *CompleteStructType
	UnionType  *CompleteUnionType
	EnumType   *CompleteEnumType
}

// CompleteStructType represents a struct type in the complete type representation.
// XTypes extensibility: APPENDABLE.
type CompleteStructType struct {
	StructFlags uint16
	Header      CompleteStructHeader
	Members     []CompleteStructMember
}

// CompleteStructHeader is the header of a CompleteStructType.
// XTypes extensibility: APPENDABLE.
type CompleteStructHeader struct {
	BaseType TypeIdentifier
	Detail   CompleteTypeDetail
}

// CompleteTypeDetail holds the type name and optional annotations.
// XTypes extensibility: FINAL.
type CompleteTypeDetail struct {
	TypeName string // QualifiedTypeName
	// ann_builtin and ann_custom are optional, encoded as absent (false)
}

// CompleteStructMember represents a single member in a CompleteStructType.
// XTypes extensibility: APPENDABLE.
type CompleteStructMember struct {
	Common CommonStructMember
	Detail CompleteMemberDetail
}

// CompleteMemberDetail holds the member name and optional annotations.
// XTypes extensibility: FINAL.
type CompleteMemberDetail struct {
	Name string // MemberName
	// ann_builtin and ann_custom are optional, encoded as absent (false)
}

// CompleteUnionType represents a union type in the complete type representation.
// XTypes extensibility: APPENDABLE.
type CompleteUnionType struct {
	UnionFlags uint16
	Header     CompleteUnionHeader
	DiscMember CompleteDiscriminatorMember
	Members    []CompleteUnionMember
}

// CompleteUnionHeader is the header of a CompleteUnionType.
// XTypes extensibility: APPENDABLE.
type CompleteUnionHeader struct {
	Detail CompleteTypeDetail
}

// CompleteDiscriminatorMember wraps CommonDiscriminatorMember for complete representation.
// XTypes extensibility: APPENDABLE.
type CompleteDiscriminatorMember struct {
	Common CommonDiscriminatorMember
	// ann_builtin and ann_custom are optional, encoded as absent (false)
}

// CompleteUnionMember represents a single case/member in a CompleteUnionType.
// XTypes extensibility: APPENDABLE.
type CompleteUnionMember struct {
	Common CommonUnionMember
	Detail CompleteMemberDetail
}

// CompleteEnumType represents an enum type in the complete type representation.
// XTypes extensibility: APPENDABLE.
type CompleteEnumType struct {
	EnumFlags uint16
	Header    CompleteEnumHeader
	Literals  []CompleteEnumLiteral
}

// CompleteEnumHeader is the header of a CompleteEnumType.
// XTypes extensibility: APPENDABLE.
type CompleteEnumHeader struct {
	BitBound uint16
	Detail   CompleteTypeDetail
}

// CompleteEnumLiteral represents a single enum value in complete representation.
// XTypes extensibility: APPENDABLE.
type CompleteEnumLiteral struct {
	Common CommonEnumLiteral
	Detail CompleteMemberDetail
}
