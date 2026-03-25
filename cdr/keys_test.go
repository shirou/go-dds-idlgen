package cdr

import (
	"bytes"
	"testing"
)

// mockExtractor implements KeyFieldExtractor for testing.
type mockExtractor struct {
	fields []DDSKeyField
	err    error
}

func (m *mockExtractor) ExtractKeyFields(_ []byte) ([]DDSKeyField, error) {
	return m.fields, m.err
}

func TestSerializedKeyExtractorBasic(t *testing.T) {
	// Simulate a CDR payload where key field is at offset 4, size 4
	data := []byte{0x00, 0x01, 0x00, 0x00, 0xCA, 0xFE, 0xBA, 0xBE, 0xFF, 0xFF}
	ext := &mockExtractor{
		fields: []DDSKeyField{
			{Offset: 4, Size: 4, TypeHint: KeyOpaque},
		},
	}
	fn := SerializedKeyExtractor(ext)
	got, err := fn(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0xCA, 0xFE, 0xBA, 0xBE}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestSerializedKeyExtractorMultipleFields(t *testing.T) {
	data := make([]byte, 40)
	data[4] = 0xAA // key1 at offset 4, size 2
	data[5] = 0xBB
	data[10] = 0xCC // key2 at offset 10, size 3
	data[11] = 0xDD
	data[12] = 0xEE
	ext := &mockExtractor{
		fields: []DDSKeyField{
			{Offset: 4, Size: 2, TypeHint: KeyOpaque},
			{Offset: 10, Size: 3, TypeHint: KeyOpaque},
		},
	}
	fn := SerializedKeyExtractor(ext)
	got, err := fn(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestSerializedKeyExtractorNoFields(t *testing.T) {
	ext := &mockExtractor{fields: nil}
	fn := SerializedKeyExtractor(ext)
	got, err := fn([]byte{0x00, 0x01, 0x00, 0x00})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %x", got)
	}
}

func TestSerializedKeyExtractorOutOfBounds(t *testing.T) {
	data := []byte{0x00, 0x01, 0x00, 0x00} // only 4 bytes
	ext := &mockExtractor{
		fields: []DDSKeyField{
			{Offset: 2, Size: 8, TypeHint: KeyOpaque}, // extends past end
		},
	}
	fn := SerializedKeyExtractor(ext)
	_, err := fn(data)
	if err == nil {
		t.Fatal("expected error for out-of-bounds key field")
	}
}
