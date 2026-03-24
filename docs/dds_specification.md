# DDS XCDR2 Specification Reference

This document summarizes the key aspects of the OMG DDS-RTPS and DDS-XTypes
specifications relevant to the CDR implementation in this project.

**Sources:**
- OMG DDSI-RTPS v2.5 (OMG Document formal/2022-04-01)
- OMG DDS-XTypes v1.3 (OMG Document formal/2020-02-04)
- Verified against: Wireshark RTPS dissector, eProsima Fast-CDR, CycloneDDS, OpenDDS

---

## 1. Encapsulation Header

The RTPS serialized payload begins with a 4-byte encapsulation header:

| Offset | Size | Description |
|--------|------|-------------|
| 0-1    | 2    | Encapsulation Kind (big-endian uint16, always big-endian regardless of payload byte order) |
| 2-3    | 2    | Options (typically 0x0000) |

The CDR payload follows immediately after byte 3. Alignment of payload fields
is relative to the start of the payload (offset 4), not the start of the header.

## 2. Encapsulation Kind Values

Per DDSI-RTPS v2.5, Table 10.2:

| Identifier       | Value  | Byte Order | CDR Version | Extensibility |
|------------------|--------|------------|-------------|---------------|
| CDR_BE           | 0x0000 | Big-endian | CDR v1      | -             |
| CDR_LE           | 0x0001 | Little-endian | CDR v1   | -             |
| PL_CDR_BE        | 0x0002 | Big-endian | CDR v1      | MUTABLE       |
| PL_CDR_LE        | 0x0003 | Little-endian | CDR v1   | MUTABLE       |
| CDR2_BE          | 0x0006 | Big-endian | CDR v2 (XCDR2) | FINAL      |
| CDR2_LE          | 0x0007 | Little-endian | CDR v2 (XCDR2) | FINAL   |
| D_CDR2_BE        | 0x0008 | Big-endian | CDR v2 (XCDR2) | APPENDABLE |
| D_CDR2_LE        | 0x0009 | Little-endian | CDR v2 (XCDR2) | APPENDABLE |
| PL_CDR2_BE       | 0x000a | Big-endian | CDR v2 (XCDR2) | MUTABLE    |
| PL_CDR2_LE       | 0x000b | Little-endian | CDR v2 (XCDR2) | MUTABLE |

**Convention:** For XCDR2 values (0x0006-0x000b), even values are big-endian
and odd values are little-endian.

**Mapping from extensibility:**
- `@final`      -> CDR2_BE (0x0006) / CDR2_LE (0x0007)
- `@appendable` -> D_CDR2_BE (0x0008) / D_CDR2_LE (0x0009)
- `@mutable`    -> PL_CDR2_BE (0x000a) / PL_CDR2_LE (0x000b)

## 3. XCDR2 Alignment Rules

Per DDS-XTypes v1.3, Section 7.4.3.4:

- XCDR2 enforces a **maximum alignment of 4 bytes**.
- Types that would normally require 8-byte alignment (INT64, UINT64, FLOAT64,
  FLOAT128) are aligned to 4 bytes instead.
- Primitive type alignments: BOOL/BYTE/INT8/UINT8 = 1, INT16/UINT16 = 2,
  INT32/UINT32/FLOAT32 = 4, INT64/UINT64/FLOAT64 = 4 (capped from 8).

## 4. String Encoding

CDR strings are encoded as:

1. **uint32 length** (4-byte aligned): The number of bytes including the null
   terminator. An empty string has length 1 (just the null byte). A truly
   absent/zero-length string may have length 0.
2. **Character data**: The string bytes (UTF-8 or ASCII).
3. **Null terminator**: A single 0x00 byte.

Total wire size = 4 (length) + length_value bytes.

## 5. DHEADER (Delimited Header)

Used by **APPENDABLE** and **MUTABLE** types in XCDR2.

The DHEADER is a single uint32 (4-byte aligned) that contains the serialized
size in bytes of the body that follows. This allows readers to skip the entire
body if the type is unknown.

```
+-------------------+-------------------+
| DHEADER (uint32)  | Body (N bytes)    |
| = N               |                   |
+-------------------+-------------------+
```

For MUTABLE types, the DHEADER wraps the entire member list. The end of the
member list is determined by reaching `start_position + DHEADER_size`.

**Important:** XCDR2 (PL_CDR2) does NOT use a sentinel to terminate the member
list. The sentinel concept (`PID_SENTINEL = 0x3F02`) belongs to XCDR1 (PL_CDR)
only. In XCDR2, the DHEADER size determines where the member list ends.

## 6. EMHEADER (Extended Member Header)

Used by **MUTABLE** types in XCDR2 for each member field.

The EMHEADER is a 32-bit value with the following layout:

```
Bit 31:      M (must_understand flag)
Bits 30-28:  LC (length code, 3 bits)
Bits 27-0:   Member ID (28 bits)
```

### Length Code (LC) Values

| LC | Member Size                    | NEXTINT Present |
|----|-------------------------------|-----------------|
| 0  | 1 byte                         | No              |
| 1  | 2 bytes                        | No              |
| 2  | 4 bytes                        | No              |
| 3  | 8 bytes                        | No              |
| 4  | NEXTINT bytes                  | Yes             |
| 5  | 4 + NEXTINT bytes              | Yes             |
| 6  | 4 + NEXTINT * 4 bytes          | Yes             |
| 7  | 4 + NEXTINT * 8 bytes          | Yes             |

When LC >= 4, a NEXTINT uint32 follows the EMHEADER immediately.

For LC 0-3, the member data follows directly after the EMHEADER.
For LC 4, the member data follows after the NEXTINT.
For LC 5-7, the "4 +" accounts for the NEXTINT itself being part of the
member serialization size.

### Writing Strategy

When encoding a member:
- If the serialized size is exactly 1, 2, 4, or 8 bytes, use the compact
  form (LC 0-3) with no NEXTINT.
- Otherwise, use LC=4 with an explicit NEXTINT containing the size.

## 7. XCDR2 Extensibility Kinds

Per DDS-XTypes v1.3, Section 7.2.2:

### FINAL (`@final`)
- Fixed layout, no extensibility.
- Wire format: Fields are serialized sequentially with no headers.
- Encapsulation: CDR2_BE / CDR2_LE

### APPENDABLE (`@appendable`)
- New fields may be appended at the end.
- Wire format: DHEADER followed by fields serialized sequentially.
- Encapsulation: D_CDR2_BE / D_CDR2_LE

### MUTABLE (`@mutable`)
- Fields may be added, removed, or reordered.
- Wire format: DHEADER followed by a sequence of (EMHEADER + field data) pairs.
- End of members is determined by the DHEADER size (no sentinel).
- Unknown fields are skipped using the size from the EMHEADER/LC.
- Encapsulation: PL_CDR2_BE / PL_CDR2_LE

## 8. Optional Fields

Per DDS-XTypes v1.3, Section 7.4.3:

In **FINAL** and **APPENDABLE** types, optional fields are represented as a
boolean presence flag (1 byte) followed by the field value if present.

In **MUTABLE** types, optional fields are simply omitted from the member list
when not present. Their absence is detected by not encountering their member ID.

## 9. Sequences and Arrays

### Sequence (variable-length)
- Encoded as: uint32 length (number of elements) followed by each element.
- The length field is 4-byte aligned.

### Array (fixed-length)
- Encoded as: each element in order, with no length prefix.
- The array size is known from the type definition.

## 10. Enum Encoding

IDL enums are encoded as int32 (4 bytes, 4-byte aligned) in CDR.

---

## 11. XTypes TypeInformation & TypeObject

Per DDS-XTypes v1.3, Section 7.3. Type Representation.

DDS discovery uses TypeInformation and TypeObject structures to match types
between participants. This section documents the wire format and hash computation
as implemented by **Cyclone DDS 11.0.1**, verified by byte-level comparison.

### 11.1 TypeInformation Wire Format

TypeInformation is the top-level structure exchanged during DDS discovery.

```idl
@extensibility(MUTABLE)
struct TypeInformation {
    @id(0x1001) TypeIdentifierWithDependencies minimal;
    @id(0x1002) TypeIdentifierWithDependencies complete;
};
```

**Wire format (MUTABLE, no encapsulation header):**

```
DHEADER (uint32)            — total content size
  EMHEADER (uint32)         — LC=4, memberID=0x1001 (minimal)
  NEXTINT  (uint32)         — minimal data size
    TypeIdentifierWithDependencies (APPENDABLE, DHEADER)
  EMHEADER (uint32)         — LC=4, memberID=0x1002 (complete)
  NEXTINT  (uint32)         — complete data size
    TypeIdentifierWithDependencies (APPENDABLE, DHEADER)
```

**Key:** The data stored in `dds_topic_descriptor_t.type_information` does NOT
include a CDR encapsulation header. It starts directly with the DHEADER.

### 11.2 TypeIdentifierWithDependencies

```idl
@extensibility(APPENDABLE)
struct TypeIdentifierWithDependencies {
    TypeIdentifierWithSize typeid_with_size;
    long dependent_typeid_count;
    sequence<TypeIdentifierWithSize> dependent_typeids;
};
```

**Wire format:**

```
DHEADER (uint32)
  TypeIdentifierWithSize (APPENDABLE, DHEADER)
  int32 dependent_typeid_count
  DHEADER (uint32)              — wraps the sequence
    uint32 sequence_length
    TypeIdentifierWithSize[0] (APPENDABLE, DHEADER)
    TypeIdentifierWithSize[1] ...
```

### 11.3 TypeIdentifierWithSize

```idl
@extensibility(APPENDABLE)
struct TypeIdentifierWithSize {
    TypeIdentifier type_id;
    uint32 typeobject_serialized_size;
};
```

**Wire format:**

```
DHEADER (uint32)
  TypeIdentifier (FINAL union, uint8 discriminator)
  [padding to 4]
  uint32 typeobject_serialized_size
```

### 11.4 EquivalenceHash Computation

The EquivalenceHash determines type identity for discovery matching.

```
EquivalenceHash = MD5( serialize(TypeObject) )[0:14]
```

**Critical:** Cyclone DDS hashes the **TypeObject** (outer MUTABLE union wrapping
MinimalTypeObject or CompleteTypeObject), not just the MinimalTypeObject directly.

```idl
@extensibility(APPENDABLE)   // Cyclone DDS uses DLC (DHEADER)
union TypeObject switch(uint8) {
    case EK_COMPLETE: CompleteTypeObject complete;
    case EK_MINIMAL:  MinimalTypeObject minimal;
};
```

The serialized bytes that are hashed include:
1. The TypeObject DHEADER (4 bytes)
2. The TypeObject discriminator (EK_MINIMAL=0xF1 or EK_COMPLETE=0xF2)
3. The MinimalTypeObject/CompleteTypeObject FINAL union body

The `typeobject_serialized_size` field in TypeIdentifierWithSize equals the total
serialized TypeObject size **including** the DHEADER.

### 11.5 MinimalTypeObject Serialization Layout

```idl
@extensibility(FINAL)
union MinimalTypeObject switch (uint8) {
    case TK_STRUCTURE: MinimalStructType struct_type;
    case TK_UNION:     MinimalUnionType union_type;
    case TK_ENUM:      MinimalEnumeratedType enumerated_type;
    // ... other cases omitted
};
```

#### Struct Layout (MinimalStructType)

The spec says `@extensibility(APPENDABLE)` for MinimalStructType, but **Cyclone
DDS serializes it without DHEADER** (no DLC in its ops). Sub-types that DO get
DHEADER are marked below.

```
TypeObject:
  DHEADER (uint32)                          — TypeObject DHEADER
  uint8  TypeObject._d = 0xF1              — EK_MINIMAL
  uint8  MinimalTypeObject._d = 0x51       — TK_STRUCTURE
  uint16 struct_flags                      — StructTypeFlag bitmask

  MinimalStructHeader:                     — has DHEADER
    DHEADER (uint32)
    TypeIdentifier base_type               — TK_NONE(0x00) if no base
    MinimalTypeDetail                      — empty (0 bytes)

  member_seq:                              — DHEADER-wrapped sequence
    DHEADER (uint32)
    uint32 sequence_length
    MinimalStructMember[0]:                — each has DHEADER
      DHEADER (uint32)
      uint32 member_id                     — sequential (0, 1, 2, ...)
      uint16 member_flags                  — StructMemberFlag
      TypeIdentifier member_type_id        — FINAL union
      NameHash name_hash                   — octet[4], MD5(name)[0:4]
    MinimalStructMember[1]: ...
```

#### Enum Layout (MinimalEnumeratedType)

```
TypeObject:
  DHEADER (uint32)
  uint8  TypeObject._d = 0xF1
  uint8  MinimalTypeObject._d = 0x40       — TK_ENUM
  uint16 enum_flags                        — IS_FINAL (0x0001)

  MinimalEnumeratedHeader:                 — has DHEADER
    DHEADER (uint32)
    uint16 bit_bound = 32

  literal_seq:                             — DHEADER-wrapped sequence
    DHEADER (uint32)
    uint32 sequence_length
    MinimalEnumeratedLiteral[0]:            — has DHEADER
      DHEADER (uint32)
      CommonEnumeratedLiteral:             — has DHEADER (Cyclone DDS specific)
        DHEADER (uint32)
        int32  value
        uint16 flags                       — first literal: IS_DEFAULT (0x0040)
      NameHash name_hash
    MinimalEnumeratedLiteral[1]: ...
```

### 11.6 DHEADER Rules in Cyclone DDS TypeObject Serialization

The following table shows which types get a DHEADER (DLC) and which don't,
based on analysis of Cyclone DDS 11.0.1 `ddsi_xt_typemap.c` ops arrays:

| Type                       | DLC (DHEADER) | Notes                              |
|----------------------------|:---:|--------------------------------------------|
| TypeObject                 | ✓   | Outer wrapper, DHEADER included in hash    |
| MinimalTypeObject          | ✗   | FINAL union                                |
| MinimalStructType          | ✗   | Flat (despite spec saying APPENDABLE)      |
| MinimalStructHeader        | ✓   |                                            |
| MinimalStructMember        | ✓   | Each element in member_seq                 |
| CommonStructMember         | ✗   | FINAL                                      |
| MinimalMemberDetail        | ✗   | FINAL, just NameHash[4]                    |
| MinimalTypeDetail          | ✗   | Empty (0 bytes)                            |
| MinimalUnionType           | ✗   | Flat                                       |
| MinimalUnionHeader         | ✓   | Empty content                              |
| MinimalDiscriminatorMember | ✓   |                                            |
| MinimalUnionMember         | ✓   |                                            |
| CommonUnionMember          | ✗   | FINAL                                      |
| MinimalEnumeratedType      | ✗   | Flat                                       |
| MinimalEnumeratedHeader    | ✓   |                                            |
| MinimalEnumeratedLiteral   | ✓   |                                            |
| CommonEnumeratedLiteral    | ✓   | **Spec says FINAL, Cyclone DDS uses DLC**  |
| TypeIdentifier             | ✗   | FINAL union                                |
| StringSTypeDefn            | ✗   | FINAL                                      |
| PlainCollectionHeader      | ✗   | FINAL                                      |
| PlainArraySElemDefn        | ✗   | FINAL                                      |
| PlainSequenceSElemDefn     | ✗   | FINAL                                      |

**Sequence DHEADER rule:** Sequences of struct elements that have DLC get their
own DHEADER wrapping (around the uint32 length + all elements).

### 11.7 TypeFlag and MemberFlag Constants

Bitmask values from `ddsi_xt_typeinfo.h`:

```
StructTypeFlag / EnumTypeFlag / UnionTypeFlag (uint16):
  IS_FINAL        = 0x0001  @position(0)
  IS_APPENDABLE   = 0x0002  @position(1)
  IS_MUTABLE      = 0x0004  @position(2)
  IS_NESTED       = 0x0008  @position(3)
  IS_AUTOID_HASH  = 0x0010  @position(4)

StructMemberFlag / UnionMemberFlag / EnumLiteralFlag (uint16):
  TRY_CONSTRUCT1     = 0x0001  @position(0)
  TRY_CONSTRUCT2     = 0x0002  @position(1)
  IS_EXTERNAL        = 0x0004  @position(2)
  IS_OPTIONAL        = 0x0008  @position(3)
  IS_MUST_UNDERSTAND = 0x0010  @position(4)
  IS_KEY             = 0x0020  @position(5)
  IS_DEFAULT         = 0x0040  @position(6)
```

**Cyclone DDS `@try_construct` mapping:**

| Annotation Value | Bits Set           | Flag Value |
|------------------|--------------------|------------|
| DISCARD (default)| TRY_CONSTRUCT1     | 0x0001     |
| USE_DEFAULT      | TRY_CONSTRUCT2     | 0x0002     |
| TRIM             | TRY_CONSTRUCT1 + 2 | 0x0003     |

**Note:** This mapping differs from the XTypes spec text which associates
DISCARD with both bits set. Cyclone DDS follows its own convention above.

### 11.8 Cyclone DDS Default Behaviors

The following defaults are used by Cyclone DDS 11.0.1 idlc compiler when
no explicit annotations are present:

| Parameter      | Default Value              | Notes                        |
|----------------|----------------------------|------------------------------|
| Extensibility  | FINAL                      | struct_flags = IS_FINAL (0x0001) |
| Member IDs     | Sequential (0, 1, 2, ...)  | No IS_AUTOID_HASH flag       |
| Member flags   | TRY_CONSTRUCT_DISCARD      | = TRY_CONSTRUCT1 = 0x0001    |
| Enum flags     | IS_FINAL                   | 0x0001                       |
| Enum literals  | First literal: IS_DEFAULT  | flags = 0x0040 for value=0   |
| Enum bit_bound | 32                         | Default for unspecified       |

### 11.9 TypeIdentifier Discriminator Values

```
Primitive TypeKind (no payload):
  TK_NONE=0x00  TK_BOOLEAN=0x01  TK_BYTE=0x02    TK_INT16=0x03
  TK_INT32=0x04 TK_INT64=0x05    TK_UINT16=0x06   TK_UINT32=0x07
  TK_UINT64=0x08 TK_FLOAT32=0x09 TK_FLOAT64=0x0A  TK_FLOAT128=0x0B
  TK_INT8=0x0C  TK_UINT8=0x0D   TK_CHAR8=0x10     TK_CHAR16=0x11

Constructed TypeKind:
  TK_STRING8=0x20  TK_ENUM=0x40  TK_STRUCTURE=0x51  TK_UNION=0x52

Parameterized (with payload):
  TI_STRING8_SMALL=0x70   TI_STRING8_LARGE=0x71
  TI_PLAIN_SEQUENCE_SMALL=0x80  TI_PLAIN_SEQUENCE_LARGE=0x81
  TI_PLAIN_ARRAY_SMALL=0x90     TI_PLAIN_ARRAY_LARGE=0x91

Equivalence:
  EK_MINIMAL=0xF1  EK_COMPLETE=0xF2  EK_BOTH=0xF3
```

### 11.10 PlainCollectionHeader equiv_kind

For array and sequence TypeIdentifiers, `PlainCollectionHeader.equiv_kind`
depends on the element type:

| Element Type                        | equiv_kind |
|-------------------------------------|------------|
| Primitive (TK_BOOLEAN, TK_INT32...) | EK_BOTH (0xF3) |
| String (TI_STRING8_SMALL/LARGE)     | EK_BOTH (0xF3) |
| Hashed type (struct, union, enum)   | EK_MINIMAL (0xF1) for minimal, EK_COMPLETE (0xF2) for complete |

**Rule:** Fully descriptive element types (same TypeIdentifier in both minimal
and complete representations) use EK_BOTH. Hashed types use the kind matching
the current representation context.

### 11.11 Complete TypeObject

The Complete representation mirrors the Minimal but includes full type names
and member names instead of hashes:

| Minimal                | Complete                    |
|------------------------|-----------------------------|
| NameHash (4 bytes)     | MemberName (string, ≤256)   |
| MinimalTypeDetail (∅)  | CompleteTypeDetail          |
| —                      | QualifiedTypeName (string)  |

CompleteTypeDetail (FINAL):
```
bool   ann_builtin_present  = false
bool   ann_custom_present   = false
string type_name            = "module::TypeName"
```

CompleteMemberDetail (FINAL):
```
string name                 = "field_name"
bool   ann_builtin_present  = false
bool   ann_custom_present   = false
```

The QualifiedTypeName uses `::` separator (e.g., `"test::SimpleType"`).

### 11.12 Worked Example: SimpleType

```idl
module test {
    @final struct SimpleType {
        long   id;
        double value;
        string name;
    };
};
```

Serialized MinimalTypeObject (72 bytes, hex):

```
44000000              DHEADER = 68 (TypeObject content size)
f1                    TypeObject._d = EK_MINIMAL
51                    MinimalTypeObject._d = TK_STRUCTURE
0100                  struct_flags = 0x0001 (IS_FINAL)
01000000              DHEADER = 1 (MinimalStructHeader)
00                    base_type = TK_NONE
000000                padding
34000000              DHEADER = 52 (member_seq)
03000000              seq_length = 3
0b000000              DHEADER = 11 (member[0])
  00000000            member_id = 0
  0100                member_flags = 0x0001 (TRY_CONSTRUCT1)
  04                  TypeIdentifier = TK_INT32
  b80bb774            NameHash("id")
00                    padding
0b000000              DHEADER = 11 (member[1])
  01000000            member_id = 1
  0100                member_flags = 0x0001
  0a                  TypeIdentifier = TK_FLOAT64
  2063c160            NameHash("value")
00                    padding
0c000000              DHEADER = 12 (member[2])
  02000000            member_id = 2
  0100                member_flags = 0x0001
  70                  TypeIdentifier = TI_STRING8_SMALL
  00                  bound = 0 (unbounded)
  b068931c            NameHash("name")
```

MD5(above 72 bytes) = `8ea5a80251229848dcf89114089a...`
EquivalenceHash = first 14 bytes = `8ea5a80251229848dcf89114089a`

---

## Common Pitfalls

1. **Encapsulation kind LE/BE convention:** Even = BE, Odd = LE. This is the
   opposite of what one might intuitively expect.

2. **No sentinel in XCDR2:** PL_CDR2 uses DHEADER size to determine end of
   member list. The sentinel concept is XCDR1-only.

3. **DHEADER is required for MUTABLE:** Both APPENDABLE and MUTABLE types use
   DHEADER. For MUTABLE, it wraps the entire set of EMHEADER+field pairs.

4. **Alignment origin after encapsulation header:** The CDR payload alignment
   starts at offset 0 of the payload (which is byte 4 of the total data).
   Since the header is exactly 4 bytes, the first field is naturally 4-byte
   aligned.

5. **TypeObject hash includes DHEADER:** The EquivalenceHash is computed over
   the entire serialized TypeObject, **including** the 4-byte DHEADER. Not just
   the MinimalTypeObject body. This is the single most impactful discovery for
   getting hash values to match Cyclone DDS.

6. **Cyclone DDS DLC vs XTypes spec extensibility:** Cyclone DDS does not
   always follow the spec's `@extensibility` annotation for TypeObject
   serialization. For example, `CommonEnumeratedLiteral` is `@extensibility
   (FINAL)` in the spec but Cyclone DDS serializes it with a DHEADER (DLC).
   Conversely, `MinimalStructType` is `@extensibility(APPENDABLE)` in the spec
   but Cyclone DDS serializes it without DHEADER. The ops arrays in
   `ddsi_xt_typemap.c` are the authoritative source for Cyclone DDS behavior.

7. **Sequence DHEADER for DLC element types:** When a sequence contains
   elements whose type has DLC (DHEADER), the entire sequence (including its
   uint32 length prefix) is wrapped in its own DHEADER. This is separate from
   the per-element DHEADERs.

8. **TypeFlag bitmask positions:** IS_FINAL is bit 0 (0x0001), NOT zero.
   An earlier draft used 0x0000 for FINAL, 0x0004 for APPENDABLE, 0x0006 for
   MUTABLE — those values are WRONG. The correct values are IS_FINAL=0x0001,
   IS_APPENDABLE=0x0002, IS_MUTABLE=0x0004, matching bitmask @position(N).

9. **TRY_CONSTRUCT mapping differs from spec:** The XTypes spec text maps
   DISCARD to TRY_CONSTRUCT1|TRY_CONSTRUCT2, but Cyclone DDS maps DISCARD to
   TRY_CONSTRUCT1 alone (0x0001). Follow Cyclone DDS for interoperability.

10. **Enum literal IS_DEFAULT flag:** Cyclone DDS sets IS_DEFAULT (0x0040) on
    the first enum literal with value 0, even without explicit
    `@default_literal` annotation. Other literals have flags = 0x0000.

11. **Collection equiv_kind:** For PlainCollectionHeader in array/sequence
    TypeIdentifiers, primitive and string elements use EK_BOTH (0xF3), not
    EK_MINIMAL. Only hashed element types (struct, union, enum) use
    EK_MINIMAL or EK_COMPLETE.
