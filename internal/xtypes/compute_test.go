package xtypes

import (
	"testing"

	"github.com/shirou/go-dds-idlgen/internal/ast"
)

func TestPrimitiveTypeIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		idlType  string
		wantKind byte
	}{
		{"boolean", "boolean", TK_BOOLEAN},
		{"octet", "octet", TK_BYTE},
		{"uint8", "uint8", TK_UINT8},
		{"int8", "int8", TK_INT8},
		{"int16", "int16", TK_INT16},
		{"short", "short", TK_INT16},
		{"int32", "int32", TK_INT32},
		{"long", "long", TK_INT32},
		{"int64", "int64", TK_INT64},
		{"uint16", "uint16", TK_UINT16},
		{"uint32", "uint32", TK_UINT32},
		{"uint64", "uint64", TK_UINT64},
		{"float", "float", TK_FLOAT32},
		{"float32", "float32", TK_FLOAT32},
		{"double", "double", TK_FLOAT64},
		{"float64", "float64", TK_FLOAT64},
		{"char", "char", TK_CHAR8},
	}

	ctx := NewComputeContext()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tid, err := ctx.ComputeTypeIdentifier(&ast.BasicType{Name: tt.idlType}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tid.Discriminator != tt.wantKind {
				t.Errorf("got discriminator 0x%02x, want 0x%02x", tid.Discriminator, tt.wantKind)
			}
		})
	}
}

func TestStringTypeIdentifier(t *testing.T) {
	ctx := NewComputeContext()

	// Unbounded string
	tid, err := ctx.ComputeTypeIdentifier(&ast.StringType{Bound: 0}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tid.Discriminator != TI_STRING8_SMALL {
		t.Errorf("unbounded string: got discriminator 0x%02x, want 0x%02x", tid.Discriminator, TI_STRING8_SMALL)
	}
	if tid.StringSBound != 0 {
		t.Errorf("unbounded string: got bound %d, want 0", tid.StringSBound)
	}

	// Small bounded string
	tid, err = ctx.ComputeTypeIdentifier(&ast.StringType{Bound: 100}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tid.Discriminator != TI_STRING8_SMALL {
		t.Errorf("small string: got discriminator 0x%02x, want 0x%02x", tid.Discriminator, TI_STRING8_SMALL)
	}
	if tid.StringSBound != 100 {
		t.Errorf("small string: got bound %d, want 100", tid.StringSBound)
	}

	// Large bounded string
	tid, err = ctx.ComputeTypeIdentifier(&ast.StringType{Bound: 1000}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tid.Discriminator != TI_STRING8_LARGE {
		t.Errorf("large string: got discriminator 0x%02x, want 0x%02x", tid.Discriminator, TI_STRING8_LARGE)
	}
	if tid.StringLBound != 1000 {
		t.Errorf("large string: got bound %d, want 1000", tid.StringLBound)
	}
}

func TestSimpleStructTypeIdentifier(t *testing.T) {
	ctx := NewComputeContext()

	// struct SimpleType { long id; double value; string name; };
	s := &ast.Struct{
		Name: "SimpleType",
		Fields: []ast.Field{
			{Name: "id", Type: &ast.BasicType{Name: "long"}},
			{Name: "value", Type: &ast.BasicType{Name: "double"}},
			{Name: "name", Type: &ast.StringType{Bound: 0}},
		},
	}

	tid, err := ctx.ComputeTypeIdentifier(
		&ast.NamedType{Name: "SimpleType", Resolved: s},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Should be EK_MINIMAL with a non-zero hash
	if tid.Discriminator != EK_MINIMAL {
		t.Errorf("got discriminator 0x%02x, want EK_MINIMAL (0x%02x)", tid.Discriminator, EK_MINIMAL)
	}

	zeroHash := EquivalenceHash{}
	if tid.Hash == zeroHash {
		t.Error("expected non-zero equivalence hash")
	}

	// Computing again should give the same result (cache hit)
	tid2, err := ctx.ComputeTypeIdentifier(
		&ast.NamedType{Name: "SimpleType", Resolved: s},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if tid.Hash != tid2.Hash {
		t.Error("expected same hash on second computation")
	}
}

func TestStructWithAnnotations(t *testing.T) {
	ctx := NewComputeContext()

	// @appendable struct
	s := &ast.Struct{
		Name: "AppendableType",
		Annotations: []ast.Annotation{
			{Name: "appendable"},
		},
		Fields: []ast.Field{
			{Name: "x", Type: &ast.BasicType{Name: "int32"}},
		},
	}

	tid, err := ctx.ComputeTypeIdentifier(
		&ast.NamedType{Name: "AppendableType", Resolved: s},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if tid.Discriminator != EK_MINIMAL {
		t.Errorf("got discriminator 0x%02x, want EK_MINIMAL", tid.Discriminator)
	}

	// Different extensibility should produce different hash
	sFinal := &ast.Struct{
		Name: "FinalType",
		Fields: []ast.Field{
			{Name: "x", Type: &ast.BasicType{Name: "int32"}},
		},
	}

	tidFinal, err := ctx.ComputeTypeIdentifier(
		&ast.NamedType{Name: "FinalType", Resolved: sFinal},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if tid.Hash == tidFinal.Hash {
		t.Error("expected different hashes for different extensibility kinds")
	}
}

func TestArrayTypeIdentifier(t *testing.T) {
	ctx := NewComputeContext()

	// uint8[16]
	tid, err := ctx.ComputeTypeIdentifier(
		&ast.ArrayType{
			ElemType: &ast.BasicType{Name: "uint8"},
			Size:     16,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if tid.Discriminator != TI_PLAIN_ARRAY_SMALL {
		t.Errorf("got discriminator 0x%02x, want TI_PLAIN_ARRAY_SMALL (0x%02x)",
			tid.Discriminator, TI_PLAIN_ARRAY_SMALL)
	}
	if len(tid.ArrSBounds) != 1 || tid.ArrSBounds[0] != 16 {
		t.Errorf("got bounds %v, want [16]", tid.ArrSBounds)
	}
}

func TestSequenceTypeIdentifier(t *testing.T) {
	ctx := NewComputeContext()

	// sequence<int32>
	tid, err := ctx.ComputeTypeIdentifier(
		&ast.SequenceType{
			ElemType: &ast.BasicType{Name: "int32"},
			Bound:    0,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if tid.Discriminator != TI_PLAIN_SEQUENCE_SMALL {
		t.Errorf("got discriminator 0x%02x, want TI_PLAIN_SEQUENCE_SMALL (0x%02x)",
			tid.Discriminator, TI_PLAIN_SEQUENCE_SMALL)
	}
	if tid.SeqSBound != 0 {
		t.Errorf("got bound %d, want 0", tid.SeqSBound)
	}
}

func TestBuildTypeInformation(t *testing.T) {
	ctx := NewComputeContext()

	s := &ast.Struct{
		Name: "SimpleType",
		Fields: []ast.Field{
			{Name: "id", Type: &ast.BasicType{Name: "long"}},
			{Name: "value", Type: &ast.BasicType{Name: "double"}},
			{Name: "name", Type: &ast.StringType{Bound: 0}},
		},
	}

	data, err := ctx.BuildTypeInformation(s, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty TypeInformation bytes")
	}

	// No encapsulation header — should start with DHEADER (MUTABLE outer)
	if len(data) < 4 {
		t.Fatal("TypeInformation too short")
	}

	t.Logf("TypeInformation (%d bytes): %x", len(data), data)
}

func TestNameHash(t *testing.T) {
	h := ComputeNameHash("id")
	if h == (NameHash{}) {
		t.Error("expected non-zero name hash")
	}

	// Same name should produce same hash
	h2 := ComputeNameHash("id")
	if h != h2 {
		t.Error("expected same hash for same name")
	}

	// Different names should (very likely) produce different hashes
	h3 := ComputeNameHash("value")
	if h == h3 {
		t.Error("expected different hashes for different names")
	}
}

func TestFormatByteLiteral(t *testing.T) {
	data := []byte{0x00, 0x07, 0x00, 0x00, 0xF1}
	result := FormatByteLiteral(data)
	if result == "" {
		t.Error("expected non-empty result")
	}
	if result == "[]byte{}" {
		t.Error("expected non-empty byte literal")
	}
}

func TestEnumTypeIdentifier(t *testing.T) {
	ctx := NewComputeContext()

	val0 := int64(0)
	val1 := int64(1)
	e := &ast.Enum{
		Name: "Color",
		Values: []ast.EnumValue{
			{Name: "RED", Value: &val0},
			{Name: "GREEN", Value: &val1},
		},
	}

	tid, err := ctx.ComputeTypeIdentifier(
		&ast.NamedType{Name: "Color", Resolved: e},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	if tid.Discriminator != EK_MINIMAL {
		t.Errorf("got discriminator 0x%02x, want EK_MINIMAL", tid.Discriminator)
	}

	zeroHash := EquivalenceHash{}
	if tid.Hash == zeroHash {
		t.Error("expected non-zero equivalence hash for enum")
	}
}

func TestDeterministicOutput(t *testing.T) {
	// Building TypeInformation twice for the same type should produce identical bytes
	s := &ast.Struct{
		Name: "DetTest",
		Fields: []ast.Field{
			{Name: "a", Type: &ast.BasicType{Name: "int32"}},
			{Name: "b", Type: &ast.BasicType{Name: "double"}},
		},
	}

	ctx1 := NewComputeContext()
	data1, err := ctx1.BuildTypeInformation(s, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx2 := NewComputeContext()
	data2, err := ctx2.BuildTypeInformation(s, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(data1) != len(data2) {
		t.Fatalf("different lengths: %d vs %d", len(data1), len(data2))
	}
	for i := range data1 {
		if data1[i] != data2[i] {
			t.Fatalf("byte %d differs: 0x%02x vs 0x%02x", i, data1[i], data2[i])
		}
	}
}

func TestNestedStructDependencies(t *testing.T) {
	ctx := NewComputeContext()

	inner := &ast.Struct{
		Name: "Inner",
		Fields: []ast.Field{
			{Name: "x", Type: &ast.BasicType{Name: "int32"}},
		},
	}

	outer := &ast.Struct{
		Name: "Outer",
		Fields: []ast.Field{
			{Name: "inner", Type: &ast.NamedType{Name: "Inner", Resolved: inner}},
			{Name: "count", Type: &ast.BasicType{Name: "uint32"}},
		},
	}

	data, err := ctx.BuildTypeInformation(outer, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty TypeInformation")
	}

	t.Logf("Nested struct TypeInformation (%d bytes): %x", len(data), data)
}
