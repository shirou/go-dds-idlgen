package parser

import (
	"testing"

	"github.com/shirou/go-dds-idlgen/internal/ast"
)

func TestParseSimpleStruct(t *testing.T) {
	src := `
struct Point {
    long x;
    long y;
    double z;
};
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(file.Definitions))
	}
	s, ok := file.Definitions[0].(*ast.Struct)
	if !ok {
		t.Fatalf("expected *ast.Struct, got %T", file.Definitions[0])
	}
	if s.Name != "Point" {
		t.Fatalf("expected name 'Point', got %q", s.Name)
	}
	if len(s.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(s.Fields))
	}
	if s.Fields[0].Name != "x" || s.Fields[0].Type.String() != "int32" {
		t.Fatalf("field 0: got %q %s", s.Fields[0].Name, s.Fields[0].Type)
	}
	if s.Fields[2].Name != "z" || s.Fields[2].Type.String() != "double" {
		t.Fatalf("field 2: got %q %s", s.Fields[2].Name, s.Fields[2].Type)
	}
}

func TestParseModuleWithStruct(t *testing.T) {
	src := `
module geometry {
    struct Point {
        double x;
        double y;
    };
};
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(file.Definitions))
	}
	mod, ok := file.Definitions[0].(*ast.Module)
	if !ok {
		t.Fatalf("expected *ast.Module, got %T", file.Definitions[0])
	}
	if mod.Name != "geometry" {
		t.Fatalf("expected module name 'geometry', got %q", mod.Name)
	}
	if len(mod.Definitions) != 1 {
		t.Fatalf("expected 1 nested definition, got %d", len(mod.Definitions))
	}
	s, ok := mod.Definitions[0].(*ast.Struct)
	if !ok {
		t.Fatalf("expected nested *ast.Struct")
	}
	if s.Name != "Point" || len(s.Fields) != 2 {
		t.Fatalf("unexpected struct: %q with %d fields", s.Name, len(s.Fields))
	}
}

func TestParseAnnotations(t *testing.T) {
	src := `
@extensibility(MUTABLE)
struct Message {
    @key long id;
    @optional string payload;
};
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	s := file.Definitions[0].(*ast.Struct)
	if len(s.Annotations) != 1 || s.Annotations[0].Name != "extensibility" {
		t.Fatalf("expected extensibility annotation, got %+v", s.Annotations)
	}
	if s.Annotations[0].Params["value"] != "MUTABLE" {
		t.Fatalf("expected MUTABLE param, got %v", s.Annotations[0].Params)
	}
	if len(s.Fields[0].Annotations) != 1 || s.Fields[0].Annotations[0].Name != "key" {
		t.Fatalf("expected @key on first field, got %+v", s.Fields[0].Annotations)
	}
	if len(s.Fields[1].Annotations) != 1 || s.Fields[1].Annotations[0].Name != "optional" {
		t.Fatalf("expected @optional on second field, got %+v", s.Fields[1].Annotations)
	}
}

func TestParseEnum(t *testing.T) {
	src := `
enum Color {
    RED,
    GREEN = 5,
    BLUE
};
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	e := file.Definitions[0].(*ast.Enum)
	if e.Name != "Color" {
		t.Fatalf("expected 'Color', got %q", e.Name)
	}
	if len(e.Values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(e.Values))
	}
	if e.Values[0].Name != "RED" || e.Values[0].Value != nil {
		t.Fatalf("unexpected RED: %+v", e.Values[0])
	}
	if e.Values[1].Name != "GREEN" || e.Values[1].Value == nil || *e.Values[1].Value != 5 {
		t.Fatalf("unexpected GREEN: %+v", e.Values[1])
	}
	if e.Values[2].Name != "BLUE" {
		t.Fatalf("unexpected BLUE: %+v", e.Values[2])
	}
}

func TestParseTypedef(t *testing.T) {
	src := `typedef long MyInt;`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	td := file.Definitions[0].(*ast.Typedef)
	if td.Name != "MyInt" || td.Type.String() != "int32" {
		t.Fatalf("unexpected typedef: %q -> %s", td.Name, td.Type)
	}
}

func TestParseConst(t *testing.T) {
	src := `const long MAX_SIZE = 100;`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	c := file.Definitions[0].(*ast.Const)
	if c.Name != "MAX_SIZE" || c.Type.String() != "int32" || c.Value != "100" {
		t.Fatalf("unexpected const: %q %s = %q", c.Name, c.Type, c.Value)
	}
}

func TestParseInclude(t *testing.T) {
	src := `
#include "base.idl"
#include <dds/core.idl>
struct Foo { long x; };
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Includes) != 2 {
		t.Fatalf("expected 2 includes, got %d", len(file.Includes))
	}
	if file.Includes[0] != "base.idl" {
		t.Fatalf("include 0: expected 'base.idl', got %q", file.Includes[0])
	}
	// Angle bracket includes may join tokens; check it contains the path.
	if file.Includes[1] == "" {
		t.Fatal("include 1 is empty")
	}
}

func TestParseSequenceAndArray(t *testing.T) {
	src := `
struct Data {
    sequence<long> values;
    sequence<double, 10> bounded;
    octet buffer[256];
};
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	s := file.Definitions[0].(*ast.Struct)
	if len(s.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(s.Fields))
	}

	seq, ok := s.Fields[0].Type.(*ast.SequenceType)
	if !ok {
		t.Fatalf("field 0: expected SequenceType, got %T", s.Fields[0].Type)
	}
	if seq.ElemType.String() != "int32" || seq.Bound != 0 {
		t.Fatalf("field 0: unexpected sequence: %s bound=%d", seq.ElemType, seq.Bound)
	}

	bseq, ok := s.Fields[1].Type.(*ast.SequenceType)
	if !ok {
		t.Fatalf("field 1: expected SequenceType, got %T", s.Fields[1].Type)
	}
	if bseq.Bound != 10 {
		t.Fatalf("field 1: expected bound 10, got %d", bseq.Bound)
	}

	arr, ok := s.Fields[2].Type.(*ast.ArrayType)
	if !ok {
		t.Fatalf("field 2: expected ArrayType, got %T", s.Fields[2].Type)
	}
	if arr.Size != 256 || arr.ElemType.String() != "octet" {
		t.Fatalf("field 2: unexpected array: %s size=%d", arr.ElemType, arr.Size)
	}
}

func TestParseStructInheritance(t *testing.T) {
	src := `
struct Base {
    long id;
};
struct Derived : Base {
    string name;
};
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Definitions) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(file.Definitions))
	}
	derived := file.Definitions[1].(*ast.Struct)
	if derived.Inherits != "Base" {
		t.Fatalf("expected inherits 'Base', got %q", derived.Inherits)
	}
}

func TestParseCompoundTypes(t *testing.T) {
	src := `
struct TypeTest {
    short a;
    unsigned short b;
    long c;
    unsigned long d;
    long long e;
    unsigned long long f;
    float g;
    double h;
};
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	s := file.Definitions[0].(*ast.Struct)
	expected := []string{
		"int16", "uint16", "int32", "uint32",
		"int64", "uint64", "float", "double",
	}
	if len(s.Fields) != len(expected) {
		t.Fatalf("expected %d fields, got %d", len(expected), len(s.Fields))
	}
	for i, want := range expected {
		got := s.Fields[i].Type.String()
		if got != want {
			t.Errorf("field %d (%s): expected type %q, got %q", i, s.Fields[i].Name, want, got)
		}
	}
}

func TestParseErrorMissingSemicolon(t *testing.T) {
	src := `
struct Bad {
    long x
};
`
	p := NewParser("test.idl", []byte(src))
	_, err := p.ParseFile()
	if err == nil {
		t.Fatal("expected error for missing semicolon")
	}
}

func TestParseErrorUnexpectedToken(t *testing.T) {
	src := `= bad;`
	p := NewParser("test.idl", []byte(src))
	_, err := p.ParseFile()
	if err == nil {
		t.Fatal("expected error for unexpected token")
	}
}

func TestParseSkippedDecl(t *testing.T) {
	src := `
interface Foo {
    void bar();
};
struct OK {
    long x;
};
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Definitions) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(file.Definitions))
	}
	skipped, ok := file.Definitions[0].(*ast.SkippedDecl)
	if !ok {
		t.Fatalf("expected SkippedDecl, got %T", file.Definitions[0])
	}
	if skipped.Kind != "interface" || skipped.Name != "Foo" {
		t.Fatalf("unexpected skipped: %+v", skipped)
	}
	_, ok = file.Definitions[1].(*ast.Struct)
	if !ok {
		t.Fatalf("expected Struct after skipped decl, got %T", file.Definitions[1])
	}
}

func TestParseStringBounded(t *testing.T) {
	src := `
struct S {
    string<128> name;
};
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	s := file.Definitions[0].(*ast.Struct)
	st, ok := s.Fields[0].Type.(*ast.StringType)
	if !ok {
		t.Fatalf("expected StringType, got %T", s.Fields[0].Type)
	}
	if st.Bound != 128 {
		t.Fatalf("expected bound 128, got %d", st.Bound)
	}
}

func TestParseScopedName(t *testing.T) {
	src := `
struct S {
    geometry::Point pos;
};
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	s := file.Definitions[0].(*ast.Struct)
	nt, ok := s.Fields[0].Type.(*ast.NamedType)
	if !ok {
		t.Fatalf("expected NamedType, got %T", s.Fields[0].Type)
	}
	if nt.Name != "geometry::Point" {
		t.Fatalf("expected 'geometry::Point', got %q", nt.Name)
	}
}

func TestParseAnnotationWithKeyValue(t *testing.T) {
	src := `
struct S {
    @id(5) long x;
};
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	s := file.Definitions[0].(*ast.Struct)
	f := s.Fields[0]
	if f.ID == nil || *f.ID != 5 {
		t.Fatalf("expected @id(5), got %v", f.ID)
	}
}

func TestParsePreprocessorDirectives(t *testing.T) {
	src := `
#ifndef MYFILE_IDL
#define MYFILE_IDL

#include "base.idl"

module UMAA {
    struct Foo {
        long x;
    };
};

#endif
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Includes) != 1 || file.Includes[0] != "base.idl" {
		t.Fatalf("expected 1 include 'base.idl', got %v", file.Includes)
	}
	if len(file.Definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(file.Definitions))
	}
}

func TestParseDirectivesOnly(t *testing.T) {
	src := `
#ifndef GUARD_H
#define GUARD_H
#endif
`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Definitions) != 0 {
		t.Fatalf("expected 0 definitions, got %d", len(file.Definitions))
	}
	if len(file.Includes) != 0 {
		t.Fatalf("expected 0 includes, got %d", len(file.Includes))
	}
}

func TestParseTypedefArray(t *testing.T) {
	src := `typedef octet UUID[16];`
	p := NewParser("test.idl", []byte(src))
	file, err := p.ParseFile()
	if err != nil {
		t.Fatal(err)
	}
	td := file.Definitions[0].(*ast.Typedef)
	if td.Name != "UUID" {
		t.Fatalf("expected name 'UUID', got %q", td.Name)
	}
	arr, ok := td.Type.(*ast.ArrayType)
	if !ok {
		t.Fatalf("expected ArrayType, got %T", td.Type)
	}
	if arr.Size != 16 {
		t.Fatalf("expected size 16, got %d", arr.Size)
	}
}
