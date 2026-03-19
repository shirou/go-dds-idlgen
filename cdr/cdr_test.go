package cdr

import (
	"encoding/binary"
	"math"
	"testing"
)

// ---------- helpers ----------

func encoderLE() *Encoder { return NewRawEncoder(binary.LittleEndian) }
func encoderBE() *Encoder { return NewRawEncoder(binary.BigEndian) }

func roundTrip[T any](
	t *testing.T,
	write func(e *Encoder, v T) error,
	read func(d *Decoder) (T, error),
	values []T,
) {
	t.Helper()
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for _, v := range values {
			enc := NewRawEncoder(order)
			if err := write(enc, v); err != nil {
				t.Fatalf("write(%v): %v", v, err)
			}
			dec := NewRawDecoder(enc.Bytes(), order)
			got, err := read(dec)
			if err != nil {
				t.Fatalf("read back %v: %v", v, err)
			}
			if any(got) != any(v) {
				t.Errorf("order=%v: wrote %v, got %v", order, v, got)
			}
		}
	}
}

// ---------- primitive round-trip tests ----------

func TestBoolRoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v bool) error { return e.WriteBool(v) },
		func(d *Decoder) (bool, error) { return d.ReadBool() },
		[]bool{true, false},
	)
}

func TestByteRoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v byte) error { return e.WriteByte(v) },
		func(d *Decoder) (byte, error) { return d.ReadByte() },
		[]byte{0, 1, 127, 255},
	)
}

func TestInt8RoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v int8) error { return e.WriteInt8(v) },
		func(d *Decoder) (int8, error) { return d.ReadInt8() },
		[]int8{0, 1, -1, math.MinInt8, math.MaxInt8},
	)
}

func TestUint8RoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v uint8) error { return e.WriteUint8(v) },
		func(d *Decoder) (uint8, error) { return d.ReadUint8() },
		[]uint8{0, 1, 255},
	)
}

func TestInt16RoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v int16) error { return e.WriteInt16(v) },
		func(d *Decoder) (int16, error) { return d.ReadInt16() },
		[]int16{0, 1, -1, math.MinInt16, math.MaxInt16},
	)
}

func TestUint16RoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v uint16) error { return e.WriteUint16(v) },
		func(d *Decoder) (uint16, error) { return d.ReadUint16() },
		[]uint16{0, 1, math.MaxUint16},
	)
}

func TestInt32RoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v int32) error { return e.WriteInt32(v) },
		func(d *Decoder) (int32, error) { return d.ReadInt32() },
		[]int32{0, 1, -1, math.MinInt32, math.MaxInt32},
	)
}

func TestUint32RoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v uint32) error { return e.WriteUint32(v) },
		func(d *Decoder) (uint32, error) { return d.ReadUint32() },
		[]uint32{0, 1, math.MaxUint32},
	)
}

func TestInt64RoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v int64) error { return e.WriteInt64(v) },
		func(d *Decoder) (int64, error) { return d.ReadInt64() },
		[]int64{0, 1, -1, math.MinInt64, math.MaxInt64},
	)
}

func TestUint64RoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v uint64) error { return e.WriteUint64(v) },
		func(d *Decoder) (uint64, error) { return d.ReadUint64() },
		[]uint64{0, 1, math.MaxUint64},
	)
}

func TestFloat32RoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v float32) error { return e.WriteFloat32(v) },
		func(d *Decoder) (float32, error) { return d.ReadFloat32() },
		[]float32{0, 1.5, -1.5, math.SmallestNonzeroFloat32, math.MaxFloat32},
	)
}

func TestFloat64RoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v float64) error { return e.WriteFloat64(v) },
		func(d *Decoder) (float64, error) { return d.ReadFloat64() },
		[]float64{0, 1.5, -1.5, math.SmallestNonzeroFloat64, math.MaxFloat64},
	)
}

func TestStringRoundTrip(t *testing.T) {
	roundTrip(t,
		func(e *Encoder, v string) error { return e.WriteString(v) },
		func(d *Decoder) (string, error) { return d.ReadString() },
		[]string{"", "hello", "Hello, World!", "a"},
	)
}

// ---------- alignment ----------

func TestAlignmentInt8ThenInt32(t *testing.T) {
	enc := encoderLE()
	_ = enc.WriteInt8(1)   // pos 1
	_ = enc.WriteInt32(42) // should pad 3 bytes, then write at pos 4

	data := enc.Bytes()
	// Total: 1 (int8) + 3 (pad) + 4 (int32) = 8
	if len(data) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(data))
	}
	// Padding bytes must be zero.
	for i := 1; i < 4; i++ {
		if data[i] != 0 {
			t.Errorf("padding byte %d is %d, want 0", i, data[i])
		}
	}

	dec := NewRawDecoder(data, binary.LittleEndian)
	v8, _ := dec.ReadInt8()
	v32, _ := dec.ReadInt32()
	if v8 != 1 || v32 != 42 {
		t.Errorf("got (%d, %d), want (1, 42)", v8, v32)
	}
}

func TestXCDR2MaxAlignment(t *testing.T) {
	// XCDR2 caps alignment at 4 even for 8-byte types.
	enc := encoderLE()
	_ = enc.WriteInt8(1)   // pos 1
	_ = enc.WriteInt64(99) // should align to 4, not 8

	data := enc.Bytes()
	// 1 (int8) + 3 (pad to 4) + 8 (int64) = 12
	if len(data) != 12 {
		t.Fatalf("expected 12 bytes (XCDR2 max align 4), got %d", len(data))
	}

	dec := NewRawDecoder(data, binary.LittleEndian)
	v8, _ := dec.ReadInt8()
	v64, _ := dec.ReadInt64()
	if v8 != 1 || v64 != 99 {
		t.Errorf("got (%d, %d), want (1, 99)", v8, v64)
	}
}

// ---------- DHEADER ----------

func TestDHeaderRoundTrip(t *testing.T) {
	enc := encoderLE()

	start := enc.BeginDHeader()
	_ = enc.WriteUint16(0x1234)
	_ = enc.WriteUint32(0xDEADBEEF)
	enc.FinishDHeader(start)

	data := enc.Bytes()
	dec := NewRawDecoder(data, binary.LittleEndian)

	dlen, err := dec.ReadDHeader()
	if err != nil {
		t.Fatal(err)
	}
	// Body: 2 (uint16) + 2 (pad) + 4 (uint32) = 8
	if dlen != 8 {
		t.Fatalf("DHEADER length = %d, want 8", dlen)
	}

	v16, _ := dec.ReadUint16()
	v32, _ := dec.ReadUint32()
	if v16 != 0x1234 || v32 != 0xDEADBEEF {
		t.Errorf("body mismatch: got (%#x, %#x)", v16, v32)
	}
}

// ---------- EMHEADER ----------

func TestEMHeaderFixedSizes(t *testing.T) {
	tests := []struct {
		dataSize int
		wantLC   uint8
	}{
		{1, 0},
		{2, 1},
		{4, 2},
		{8, 3},
	}

	for _, tt := range tests {
		enc := encoderLE()
		_ = enc.WriteEMHeader(false, 5, tt.dataSize)

		dec := NewRawDecoder(enc.Bytes(), binary.LittleEndian)
		mu, lc, mid, ni, err := dec.ReadEMHeader()
		if err != nil {
			t.Fatalf("dataSize=%d: %v", tt.dataSize, err)
		}
		if mu {
			t.Errorf("dataSize=%d: must_understand should be false", tt.dataSize)
		}
		if lc != tt.wantLC {
			t.Errorf("dataSize=%d: LC=%d, want %d", tt.dataSize, lc, tt.wantLC)
		}
		if mid != 5 {
			t.Errorf("dataSize=%d: memberID=%d, want 5", tt.dataSize, mid)
		}
		if ni != 0 {
			t.Errorf("dataSize=%d: nextInt=%d, want 0", tt.dataSize, ni)
		}
	}
}

func TestEMHeaderNextInt(t *testing.T) {
	enc := encoderLE()
	_ = enc.WriteEMHeader(true, 42, 100)

	dec := NewRawDecoder(enc.Bytes(), binary.LittleEndian)
	mu, lc, mid, ni, err := dec.ReadEMHeader()
	if err != nil {
		t.Fatal(err)
	}
	if !mu {
		t.Error("must_understand should be true")
	}
	if lc != 4 {
		t.Errorf("LC=%d, want 4", lc)
	}
	if mid != 42 {
		t.Errorf("memberID=%d, want 42", mid)
	}
	if ni != 100 {
		t.Errorf("nextInt=%d, want 100", ni)
	}
}

func TestEMHeaderMustUnderstand(t *testing.T) {
	enc := encoderLE()
	_ = enc.WriteEMHeader(true, 7, 4)

	dec := NewRawDecoder(enc.Bytes(), binary.LittleEndian)
	mu, lc, mid, _, err := dec.ReadEMHeader()
	if err != nil {
		t.Fatal(err)
	}
	if !mu {
		t.Error("must_understand should be true")
	}
	if lc != 2 {
		t.Errorf("LC=%d, want 2", lc)
	}
	if mid != 7 {
		t.Errorf("memberID=%d, want 7", mid)
	}
}

func TestPLCDRSentinelRoundTrip(t *testing.T) {
	enc := encoderLE()
	_ = enc.WritePLCDRSentinel()

	data := enc.Bytes()
	raw := binary.LittleEndian.Uint32(data)
	if raw != PLCDRSentinelHeader {
		t.Fatalf("sentinel raw = %#08x, want %#08x", raw, PLCDRSentinelHeader)
	}
	if !IsSentinel(raw) {
		t.Error("IsSentinel should return true for sentinel header")
	}
}

// ---------- edge cases ----------

func TestEmptyString(t *testing.T) {
	enc := encoderLE()
	_ = enc.WriteString("")

	dec := NewRawDecoder(enc.Bytes(), binary.LittleEndian)
	s, err := dec.ReadString()
	if err != nil {
		t.Fatal(err)
	}
	if s != "" {
		t.Errorf("got %q, want empty string", s)
	}
}

func TestZeroValues(t *testing.T) {
	enc := encoderLE()
	_ = enc.WriteBool(false)
	_ = enc.WriteInt8(0)
	_ = enc.WriteUint16(0)
	_ = enc.WriteInt32(0)
	_ = enc.WriteUint64(0)
	_ = enc.WriteFloat32(0)
	_ = enc.WriteFloat64(0)

	dec := NewRawDecoder(enc.Bytes(), binary.LittleEndian)

	b, _ := dec.ReadBool()
	i8, _ := dec.ReadInt8()
	u16, _ := dec.ReadUint16()
	i32, _ := dec.ReadInt32()
	u64, _ := dec.ReadUint64()
	f32, _ := dec.ReadFloat32()
	f64, _ := dec.ReadFloat64()

	if b || i8 != 0 || u16 != 0 || i32 != 0 || u64 != 0 || f32 != 0 || f64 != 0 {
		t.Error("one or more zero values did not round-trip correctly")
	}
}

func TestMaxValues(t *testing.T) {
	enc := encoderBE()
	_ = enc.WriteUint16(math.MaxUint16)
	_ = enc.WriteInt32(math.MaxInt32)
	_ = enc.WriteUint64(math.MaxUint64)

	dec := NewRawDecoder(enc.Bytes(), binary.BigEndian)
	u16, _ := dec.ReadUint16()
	i32, _ := dec.ReadInt32()
	u64, _ := dec.ReadUint64()

	if u16 != math.MaxUint16 {
		t.Errorf("uint16 max: got %d", u16)
	}
	if i32 != math.MaxInt32 {
		t.Errorf("int32 max: got %d", i32)
	}
	if u64 != math.MaxUint64 {
		t.Errorf("uint64 max: got %d", u64)
	}
}

func TestDecoderRemainingAndPos(t *testing.T) {
	enc := encoderLE()
	_ = enc.WriteUint32(1)
	_ = enc.WriteUint32(2)

	dec := NewRawDecoder(enc.Bytes(), binary.LittleEndian)
	if dec.Remaining() != 8 {
		t.Errorf("remaining = %d, want 8", dec.Remaining())
	}
	if dec.Pos() != 0 {
		t.Errorf("pos = %d, want 0", dec.Pos())
	}

	_, _ = dec.ReadUint32()
	if dec.Remaining() != 4 {
		t.Errorf("remaining = %d, want 4", dec.Remaining())
	}
	if dec.Pos() != 4 {
		t.Errorf("pos = %d, want 4", dec.Pos())
	}
}

func TestDecoderReadPastEnd(t *testing.T) {
	dec := NewRawDecoder([]byte{0x01}, binary.LittleEndian)
	_, err := dec.ReadUint32()
	if err == nil {
		t.Fatal("expected error reading uint32 from 1-byte buffer")
	}
}

func TestEncoderReset(t *testing.T) {
	enc := encoderLE()
	_ = enc.WriteUint32(42)
	enc.Reset()
	if len(enc.Bytes()) != 0 {
		t.Error("buffer should be empty after Reset")
	}
	// Verify encoder is usable after reset.
	_ = enc.WriteUint32(7)
	dec := NewRawDecoder(enc.Bytes(), binary.LittleEndian)
	v, _ := dec.ReadUint32()
	if v != 7 {
		t.Errorf("got %d after reset, want 7", v)
	}
}

// ---------- encapsulation header ----------

func TestNewEncoderWritesHeader(t *testing.T) {
	enc := NewEncoder(CDR2_LE)
	_ = enc.WriteUint32(0xDEADBEEF)

	data := enc.Bytes()
	// 4 (header) + 4 (uint32) = 8
	if len(data) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(data))
	}
	// Header: kind=0x0006 big-endian, options=0x0000
	if data[0] != 0x00 || data[1] != 0x06 || data[2] != 0x00 || data[3] != 0x00 {
		t.Errorf("header = %x, want [00 06 00 00]", data[:4])
	}
	// Payload in LE
	v := binary.LittleEndian.Uint32(data[4:8])
	if v != 0xDEADBEEF {
		t.Errorf("payload = %#x, want 0xDEADBEEF", v)
	}
}

func TestNewEncoderBEHeader(t *testing.T) {
	enc := NewEncoder(CDR2_BE)
	_ = enc.WriteUint32(0x12345678)

	data := enc.Bytes()
	if data[0] != 0x00 || data[1] != 0x07 {
		t.Errorf("header kind = %x %x, want 00 07", data[0], data[1])
	}
	v := binary.BigEndian.Uint32(data[4:8])
	if v != 0x12345678 {
		t.Errorf("payload = %#x, want 0x12345678", v)
	}
}

func TestNewDecoderReadsHeader(t *testing.T) {
	// Build data with CDR2_LE header + a uint32
	var buf [8]byte
	binary.BigEndian.PutUint16(buf[:2], uint16(CDR2_LE))
	binary.LittleEndian.PutUint32(buf[4:8], 42)

	dec, err := NewDecoder(buf[:])
	if err != nil {
		t.Fatal(err)
	}
	v, err := dec.ReadUint32()
	if err != nil {
		t.Fatal(err)
	}
	if v != 42 {
		t.Errorf("got %d, want 42", v)
	}
}

func TestNewDecoderTooShort(t *testing.T) {
	_, err := NewDecoder([]byte{0x00, 0x06})
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestNewDecoderUnknownKind(t *testing.T) {
	data := []byte{0xFF, 0xFF, 0x00, 0x00}
	_, err := NewDecoder(data)
	if err == nil {
		t.Fatal("expected error for unknown encapsulation kind")
	}
}

func TestRoundTripWithEncapsulationHeader(t *testing.T) {
	for _, kind := range []EncapsulationKind{CDR2_LE, CDR2_BE, DELIMITED_CDR2_LE, PL_CDR2_BE} {
		enc := NewEncoder(kind)
		_ = enc.WriteUint32(12345)
		_ = enc.WriteString("hello")

		dec, err := NewDecoder(enc.Bytes())
		if err != nil {
			t.Fatalf("kind=%#x: NewDecoder: %v", kind, err)
		}
		v, _ := dec.ReadUint32()
		s, _ := dec.ReadString()
		if v != 12345 || s != "hello" {
			t.Errorf("kind=%#x: got (%d, %q), want (12345, hello)", kind, v, s)
		}
	}
}

func TestGetEncapsulationKind(t *testing.T) {
	kind := GetEncapsulationKind(FINAL)
	// Just verify it returns a valid LE or BE kind
	if kind != CDR2_LE && kind != CDR2_BE {
		t.Errorf("FINAL: got %#x, want CDR2_LE or CDR2_BE", kind)
	}
	kind = GetEncapsulationKind(APPENDABLE)
	if kind != DELIMITED_CDR2_LE && kind != DELIMITED_CDR2_BE {
		t.Errorf("APPENDABLE: got %#x", kind)
	}
	kind = GetEncapsulationKind(MUTABLE)
	if kind != PL_CDR2_LE && kind != PL_CDR2_BE {
		t.Errorf("MUTABLE: got %#x", kind)
	}
}
