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
