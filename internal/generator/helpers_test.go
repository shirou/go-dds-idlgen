package generator

import (
	"testing"

	"github.com/shirou/go-dds-idlgen/internal/ast"
)

func TestGoType_BasicTypes(t *testing.T) {
	tests := []struct {
		input ast.TypeRef
		want  string
	}{
		{&ast.BasicType{Name: "boolean"}, "bool"},
		{&ast.BasicType{Name: "octet"}, "uint8"},
		{&ast.BasicType{Name: "uint8"}, "uint8"},
		{&ast.BasicType{Name: "char"}, "int8"},
		{&ast.BasicType{Name: "int8"}, "int8"},
		{&ast.BasicType{Name: "int16"}, "int16"},
		{&ast.BasicType{Name: "short"}, "int16"},
		{&ast.BasicType{Name: "uint16"}, "uint16"},
		{&ast.BasicType{Name: "int32"}, "int32"},
		{&ast.BasicType{Name: "long"}, "int32"},
		{&ast.BasicType{Name: "uint32"}, "uint32"},
		{&ast.BasicType{Name: "int64"}, "int64"},
		{&ast.BasicType{Name: "uint64"}, "uint64"},
		{&ast.BasicType{Name: "float"}, "float32"},
		{&ast.BasicType{Name: "float32"}, "float32"},
		{&ast.BasicType{Name: "double"}, "float64"},
		{&ast.BasicType{Name: "float64"}, "float64"},
	}
	for _, tt := range tests {
		got := goType(tt.input)
		if got != tt.want {
			t.Errorf("goType(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGoType_StringType(t *testing.T) {
	got := goType(&ast.StringType{})
	if got != "string" {
		t.Errorf("goType(StringType) = %q, want %q", got, "string")
	}
}

func TestGoType_SequenceType(t *testing.T) {
	got := goType(&ast.SequenceType{ElemType: &ast.BasicType{Name: "int32"}})
	if got != "[]int32" {
		t.Errorf("goType(SequenceType<int32>) = %q, want %q", got, "[]int32")
	}
}

func TestGoType_ArrayType(t *testing.T) {
	got := goType(&ast.ArrayType{ElemType: &ast.BasicType{Name: "double"}, Size: 3})
	if got != "[3]float64" {
		t.Errorf("goType(ArrayType<double,3>) = %q, want %q", got, "[3]float64")
	}
}

func TestGoType_NamedType(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"MyType", "MyType"},
		{"sensor_data", "SensorData"},
		{"module::MyType", "module.MyType"},
	}
	for _, tt := range tests {
		got := goType(&ast.NamedType{Name: tt.name})
		if got != tt.want {
			t.Errorf("goType(NamedType{%q}) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestGoType_Unknown(t *testing.T) {
	// Unknown TypeRef implementations should return "any" from the default case.
	// We cannot easily construct a nil interface, so this test documents the expectation.
}

func TestPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello_world", "HelloWorld"},
		{"my_struct", "MyStruct"},
		{"single", "Single"},
		{"", ""},
		{"already_Upper", "AlreadyUpper"},
	}
	for _, tt := range tests {
		got := pascalCase(tt.input)
		if got != tt.want {
			t.Errorf("pascalCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello_world", "helloWorld"},
		{"MyStruct", "myStruct"},
		{"", ""},
	}
	for _, tt := range tests {
		got := camelCase(tt.input)
		if got != tt.want {
			t.Errorf("camelCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"HelloWorld", "hello_world"},
		{"MyStruct", "my_struct"},
		{"simple", "simple"},
		{"", ""},
		{"HTTPServer", "http_server"},
		{"getHTTPResponse", "get_http_response"},
		{"sensor_data", "sensor_data"},
	}
	for _, tt := range tests {
		got := snakeCase(tt.input)
		if got != tt.want {
			t.Errorf("snakeCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHasAnnotation(t *testing.T) {
	annots := []ast.Annotation{
		{Name: "key", Params: nil},
		{Name: "optional", Params: nil},
	}
	if !hasAnnotation(annots, "key") {
		t.Error("expected to find @key annotation")
	}
	if !hasAnnotation(annots, "optional") {
		t.Error("expected to find @optional annotation")
	}
	if hasAnnotation(annots, "nonexistent") {
		t.Error("did not expect to find @nonexistent annotation")
	}
	if hasAnnotation(nil, "key") {
		t.Error("nil annotations should not contain any annotation")
	}
}

func TestAnnotationValue(t *testing.T) {
	annots := []ast.Annotation{
		{Name: "extensibility", Params: map[string]string{"value": "MUTABLE"}},
		{Name: "id", Params: map[string]string{"": "42"}},
		{Name: "range", Params: map[string]string{"min": "0", "max": "100"}},
	}

	if v := annotationValue(annots, "extensibility", ""); v != "MUTABLE" {
		t.Errorf("annotationValue(extensibility, '') = %q, want %q", v, "MUTABLE")
	}
	if v := annotationValue(annots, "id", ""); v != "42" {
		t.Errorf("annotationValue(id, '') = %q, want %q", v, "42")
	}
	if v := annotationValue(annots, "range", "min"); v != "0" {
		t.Errorf("annotationValue(range, 'min') = %q, want %q", v, "0")
	}
	if v := annotationValue(annots, "range", "max"); v != "100" {
		t.Errorf("annotationValue(range, 'max') = %q, want %q", v, "100")
	}
	if v := annotationValue(annots, "missing", ""); v != "" {
		t.Errorf("annotationValue(missing, '') = %q, want empty", v)
	}
}

func TestExtensibility(t *testing.T) {
	tests := []struct {
		name string
		s    *ast.Struct
		want string
	}{
		{
			"default is FINAL",
			&ast.Struct{Name: "Foo"},
			"FINAL",
		},
		{
			"explicit extensibility annotation",
			&ast.Struct{Name: "Bar", Annotations: []ast.Annotation{
				{Name: "extensibility", Params: map[string]string{"value": "appendable"}},
			}},
			"APPENDABLE",
		},
		{
			"shorthand final",
			&ast.Struct{Name: "Baz", Annotations: []ast.Annotation{
				{Name: "final"},
			}},
			"FINAL",
		},
		{
			"shorthand appendable",
			&ast.Struct{Name: "Qux", Annotations: []ast.Annotation{
				{Name: "appendable"},
			}},
			"APPENDABLE",
		},
		{
			"shorthand mutable",
			&ast.Struct{Name: "Quux", Annotations: []ast.Annotation{
				{Name: "mutable"},
			}},
			"MUTABLE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extensibility(tt.s)
			if got != tt.want {
				t.Errorf("extensibility(%q) = %q, want %q", tt.s.Name, got, tt.want)
			}
		})
	}
}

func TestIsOptionalAndIsKey(t *testing.T) {
	f := ast.Field{
		Name: "x",
		Type: &ast.BasicType{Name: "int32"},
		Annotations: []ast.Annotation{
			{Name: "optional"},
			{Name: "key"},
		},
	}
	if !isOptional(f) {
		t.Error("expected field to be optional")
	}
	if !isKey(f) {
		t.Error("expected field to be key")
	}

	f2 := ast.Field{Name: "y", Type: &ast.BasicType{Name: "int32"}}
	if isOptional(f2) {
		t.Error("expected field not to be optional")
	}
	if isKey(f2) {
		t.Error("expected field not to be key")
	}
}

func TestGoFieldType(t *testing.T) {
	// Non-optional field
	f := ast.Field{Name: "x", Type: &ast.BasicType{Name: "int32"}}
	if got := goFieldType(f); got != "int32" {
		t.Errorf("goFieldType(non-optional) = %q, want %q", got, "int32")
	}

	// Optional field
	f2 := ast.Field{
		Name:        "y",
		Type:        &ast.BasicType{Name: "int32"},
		Annotations: []ast.Annotation{{Name: "optional"}},
	}
	if got := goFieldType(f2); got != "*int32" {
		t.Errorf("goFieldType(optional) = %q, want %q", got, "*int32")
	}
}

func TestCdrAlignment(t *testing.T) {
	tests := []struct {
		typ  ast.TypeRef
		want int
	}{
		{&ast.BasicType{Name: "boolean"}, 1},
		{&ast.BasicType{Name: "octet"}, 1},
		{&ast.BasicType{Name: "int16"}, 2},
		{&ast.BasicType{Name: "int32"}, 4},
		{&ast.BasicType{Name: "int64"}, 4}, // XCDR2 max is 4
		{&ast.BasicType{Name: "double"}, 4},
		{&ast.StringType{}, 4},
		{&ast.SequenceType{ElemType: &ast.BasicType{Name: "int32"}}, 4},
		{&ast.ArrayType{ElemType: &ast.BasicType{Name: "int16"}, Size: 5}, 2},
		{&ast.NamedType{Name: "SomeType"}, 4},
	}
	for _, tt := range tests {
		got := cdrAlignment(tt.typ)
		if got != tt.want {
			t.Errorf("cdrAlignment(%v) = %d, want %d", tt.typ, got, tt.want)
		}
	}
}

func TestCdrWriteFunc(t *testing.T) {
	tests := []struct {
		typ  ast.TypeRef
		want string
	}{
		{&ast.BasicType{Name: "boolean"}, "WriteBool"},
		{&ast.BasicType{Name: "octet"}, "WriteUint8"},
		{&ast.BasicType{Name: "int16"}, "WriteInt16"},
		{&ast.BasicType{Name: "int32"}, "WriteInt32"},
		{&ast.BasicType{Name: "int64"}, "WriteInt64"},
		{&ast.BasicType{Name: "uint64"}, "WriteUint64"},
		{&ast.BasicType{Name: "float"}, "WriteFloat32"},
		{&ast.BasicType{Name: "double"}, "WriteFloat64"},
		{&ast.StringType{}, "WriteString"},
		{&ast.SequenceType{ElemType: &ast.BasicType{Name: "int32"}}, ""},
	}
	for _, tt := range tests {
		got := cdrWriteFunc(tt.typ)
		if got != tt.want {
			t.Errorf("cdrWriteFunc(%v) = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

func TestCdrReadFunc(t *testing.T) {
	tests := []struct {
		typ  ast.TypeRef
		want string
	}{
		{&ast.BasicType{Name: "boolean"}, "ReadBool"},
		{&ast.BasicType{Name: "octet"}, "ReadUint8"},
		{&ast.BasicType{Name: "int16"}, "ReadInt16"},
		{&ast.BasicType{Name: "int32"}, "ReadInt32"},
		{&ast.BasicType{Name: "int64"}, "ReadInt64"},
		{&ast.BasicType{Name: "uint64"}, "ReadUint64"},
		{&ast.BasicType{Name: "float"}, "ReadFloat32"},
		{&ast.BasicType{Name: "double"}, "ReadFloat64"},
		{&ast.StringType{}, "ReadString"},
		{&ast.NamedType{Name: "SomeType"}, ""},
	}
	for _, tt := range tests {
		got := cdrReadFunc(tt.typ)
		if got != tt.want {
			t.Errorf("cdrReadFunc(%v) = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

func TestIsPrimitive(t *testing.T) {
	if !isPrimitive(&ast.BasicType{Name: "int32"}) {
		t.Error("BasicType should be primitive")
	}
	if !isPrimitive(&ast.StringType{}) {
		t.Error("StringType should be primitive")
	}
	if isPrimitive(&ast.SequenceType{ElemType: &ast.BasicType{Name: "int32"}}) {
		t.Error("SequenceType should not be primitive")
	}
	if isPrimitive(&ast.ArrayType{ElemType: &ast.BasicType{Name: "int32"}, Size: 3}) {
		t.Error("ArrayType should not be primitive")
	}
	if isPrimitive(&ast.NamedType{Name: "Foo"}) {
		t.Error("NamedType should not be primitive")
	}
}

func TestIsSequenceAndIsArray(t *testing.T) {
	seq := &ast.SequenceType{ElemType: &ast.BasicType{Name: "int32"}}
	arr := &ast.ArrayType{ElemType: &ast.BasicType{Name: "int32"}, Size: 5}
	basic := &ast.BasicType{Name: "int32"}

	if !isSequence(seq) {
		t.Error("expected SequenceType to be a sequence")
	}
	if isSequence(basic) {
		t.Error("expected BasicType not to be a sequence")
	}
	if !isArray(arr) {
		t.Error("expected ArrayType to be an array")
	}
	if isArray(basic) {
		t.Error("expected BasicType not to be an array")
	}
}

func TestSequenceElemType(t *testing.T) {
	seq := &ast.SequenceType{ElemType: &ast.BasicType{Name: "int32"}}
	if got := sequenceElemType(seq); got != "int32" {
		t.Errorf("sequenceElemType = %q, want %q", got, "int32")
	}
	if got := sequenceElemType(&ast.BasicType{Name: "int32"}); got != "" {
		t.Errorf("sequenceElemType(non-seq) = %q, want empty", got)
	}
}

func TestArrayElemTypeAndSize(t *testing.T) {
	arr := &ast.ArrayType{ElemType: &ast.BasicType{Name: "double"}, Size: 10}
	if got := arrayElemType(arr); got != "float64" {
		t.Errorf("arrayElemType = %q, want %q", got, "float64")
	}
	if got := arraySize(arr); got != 10 {
		t.Errorf("arraySize = %d, want %d", got, 10)
	}
	if got := arrayElemType(&ast.BasicType{Name: "int32"}); got != "" {
		t.Errorf("arrayElemType(non-arr) = %q, want empty", got)
	}
	if got := arraySize(&ast.BasicType{Name: "int32"}); got != 0 {
		t.Errorf("arraySize(non-arr) = %d, want 0", got)
	}
}

func TestEnumComputedValue(t *testing.T) {
	val0 := int64(0)
	val5 := int64(5)
	values := []ast.EnumValue{
		{Name: "RED", Value: &val0},
		{Name: "GREEN"},          // should be 1
		{Name: "BLUE", Value: &val5},
		{Name: "YELLOW"},         // should be 6
		{Name: "PURPLE"},         // should be 7
	}

	want := []int64{0, 1, 5, 6, 7}
	for i, w := range want {
		got := enumComputedValue(values, i)
		if got != w {
			t.Errorf("enumComputedValue(values, %d) = %d, want %d", i, got, w)
		}
	}

	// All implicit values starting from 0
	implicitValues := []ast.EnumValue{
		{Name: "A"},
		{Name: "B"},
		{Name: "C"},
	}
	for i, w := range []int64{0, 1, 2} {
		got := enumComputedValue(implicitValues, i)
		if got != w {
			t.Errorf("enumComputedValue(implicit, %d) = %d, want %d", i, got, w)
		}
	}
}

func TestCdrSerializedSizeRec(t *testing.T) {
	// Basic types
	if got := cdrSerializedSizeRec(&ast.BasicType{Name: "int32"}, nil); got != 4 {
		t.Errorf("int32 size = %d, want 4", got)
	}
	if got := cdrSerializedSizeRec(&ast.BasicType{Name: "octet"}, nil); got != 1 {
		t.Errorf("octet size = %d, want 1", got)
	}

	// String is variable
	if got := cdrSerializedSizeRec(&ast.StringType{}, nil); got != 0 {
		t.Errorf("string size = %d, want 0", got)
	}

	// Fixed array
	if got := cdrSerializedSizeRec(&ast.ArrayType{ElemType: &ast.BasicType{Name: "octet"}, Size: 16}, nil); got != 16 {
		t.Errorf("octet[16] size = %d, want 16", got)
	}

	// Enum via NamedType
	enumDef := &ast.Enum{Name: "Color", Values: []ast.EnumValue{{Name: "RED"}}}
	nt := &ast.NamedType{Name: "Color", Resolved: enumDef}
	if got := cdrSerializedSizeRec(nt, nil); got != 4 {
		t.Errorf("enum size = %d, want 4", got)
	}

	// Fixed-size struct via NamedType
	innerStruct := &ast.Struct{
		Name: "Point",
		Fields: []ast.Field{
			{Name: "x", Type: &ast.BasicType{Name: "float"}},
			{Name: "y", Type: &ast.BasicType{Name: "float"}},
		},
	}
	ntStruct := &ast.NamedType{Name: "Point", Resolved: innerStruct}
	if got := cdrSerializedSizeRec(ntStruct, nil); got != 8 {
		t.Errorf("Point struct size = %d, want 8", got)
	}

	// Struct with optional field -> variable
	optStruct := &ast.Struct{
		Name: "Opt",
		Fields: []ast.Field{
			{Name: "x", Type: &ast.BasicType{Name: "int32"}, Annotations: []ast.Annotation{{Name: "optional"}}},
		},
	}
	ntOpt := &ast.NamedType{Name: "Opt", Resolved: optStruct}
	if got := cdrSerializedSizeRec(ntOpt, nil); got != 0 {
		t.Errorf("optional struct size = %d, want 0", got)
	}

	// Struct with string field -> variable
	strStruct := &ast.Struct{
		Name: "Msg",
		Fields: []ast.Field{
			{Name: "text", Type: &ast.StringType{}},
		},
	}
	ntStr := &ast.NamedType{Name: "Msg", Resolved: strStruct}
	if got := cdrSerializedSizeRec(ntStr, nil); got != 0 {
		t.Errorf("string-field struct size = %d, want 0", got)
	}

	// Struct with inheritance -> variable (can't resolve base)
	inheritStruct := &ast.Struct{
		Name:     "Derived",
		Inherits: "Base",
		Fields: []ast.Field{
			{Name: "z", Type: &ast.BasicType{Name: "int32"}},
		},
	}
	ntInherit := &ast.NamedType{Name: "Derived", Resolved: inheritStruct}
	if got := cdrSerializedSizeRec(ntInherit, nil); got != 0 {
		t.Errorf("inherited struct size = %d, want 0", got)
	}
}

func TestCanStaticOffset(t *testing.T) {
	fields := []ast.Field{
		{Name: "a", Type: &ast.BasicType{Name: "int32"}},
		{Name: "b", Type: &ast.BasicType{Name: "octet"}},
		{Name: "c", Type: &ast.BasicType{Name: "int16"}},
	}

	// All fixed fields before index 2
	if !canStaticOffset(fields, 2) {
		t.Error("expected canStaticOffset(fields, 2) = true")
	}
	if !canStaticOffset(fields, 0) {
		t.Error("expected canStaticOffset(fields, 0) = true for first field")
	}

	// With optional field
	fieldsOpt := []ast.Field{
		{Name: "a", Type: &ast.BasicType{Name: "int32"}},
		{Name: "b", Type: &ast.BasicType{Name: "int32"}, Annotations: []ast.Annotation{{Name: "optional"}}},
		{Name: "c", Type: &ast.BasicType{Name: "int32"}},
	}
	if canStaticOffset(fieldsOpt, 2) {
		t.Error("expected canStaticOffset = false with optional before target")
	}
	if !canStaticOffset(fieldsOpt, 1) {
		t.Error("expected canStaticOffset(fieldsOpt, 1) = true (optional is at index 1)")
	}

	// With string field (variable size)
	fieldsStr := []ast.Field{
		{Name: "name", Type: &ast.StringType{}},
		{Name: "id", Type: &ast.BasicType{Name: "int32"}},
	}
	if canStaticOffset(fieldsStr, 1) {
		t.Error("expected canStaticOffset = false with string before target")
	}
}

func TestStaticOffset(t *testing.T) {
	tests := []struct {
		name   string
		fields []ast.Field
		upTo   int
		want   int
	}{
		{
			"first field",
			[]ast.Field{
				{Name: "a", Type: &ast.BasicType{Name: "int32"}},
			},
			0,
			0,
		},
		{
			"second field after int32",
			[]ast.Field{
				{Name: "a", Type: &ast.BasicType{Name: "int32"}},
				{Name: "b", Type: &ast.BasicType{Name: "int32"}},
			},
			1,
			4,
		},
		{
			"alignment: octet then int32",
			[]ast.Field{
				{Name: "a", Type: &ast.BasicType{Name: "octet"}},
				{Name: "b", Type: &ast.BasicType{Name: "int32"}},
			},
			1,
			4, // 1 byte + 3 padding
		},
		{
			"alignment: int32 then octet then int16",
			[]ast.Field{
				{Name: "a", Type: &ast.BasicType{Name: "int32"}},
				{Name: "b", Type: &ast.BasicType{Name: "octet"}},
				{Name: "c", Type: &ast.BasicType{Name: "int16"}},
			},
			2,
			6, // 4 + 1 + 1(pad) = 6
		},
		{
			"array of octets",
			[]ast.Field{
				{Name: "data", Type: &ast.ArrayType{ElemType: &ast.BasicType{Name: "octet"}, Size: 32}},
				{Name: "id", Type: &ast.BasicType{Name: "int32"}},
			},
			1,
			32, // 32 bytes, already 4-aligned
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := staticOffset(tt.fields, tt.upTo)
			if got != tt.want {
				t.Errorf("staticOffset = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestKeyTypeHint(t *testing.T) {
	tests := []struct {
		typ  ast.TypeRef
		want string
	}{
		{&ast.BasicType{Name: "int32"}, "KeyInt32"},
		{&ast.BasicType{Name: "long"}, "KeyInt32"},
		{&ast.BasicType{Name: "uint32"}, "KeyInt32"},
		{&ast.BasicType{Name: "int64"}, "KeyInt64"},
		{&ast.BasicType{Name: "uint64"}, "KeyInt64"},
		{&ast.StringType{}, "KeyString"},
		{&ast.ArrayType{ElemType: &ast.BasicType{Name: "octet"}, Size: 16}, "KeyUUID"},
		{&ast.ArrayType{ElemType: &ast.BasicType{Name: "uint8"}, Size: 16}, "KeyUUID"},
		{&ast.ArrayType{ElemType: &ast.BasicType{Name: "octet"}, Size: 32}, "KeyOpaque"},
		{&ast.BasicType{Name: "float"}, "KeyOpaque"},
		{&ast.BasicType{Name: "boolean"}, "KeyOpaque"},
	}
	for _, tt := range tests {
		got := keyTypeHint(tt.typ)
		if got != tt.want {
			t.Errorf("keyTypeHint(%v) = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

func TestComputeKeyFields_Static(t *testing.T) {
	s := &ast.Struct{
		Name: "Sensor",
		Fields: []ast.Field{
			{Name: "id", Type: &ast.BasicType{Name: "int32"}, Annotations: []ast.Annotation{{Name: "key"}}},
			{Name: "value", Type: &ast.BasicType{Name: "float"}},
		},
	}

	kfs := computeKeyFields(s)
	if len(kfs) != 1 {
		t.Fatalf("expected 1 key field, got %d", len(kfs))
	}
	kf := kfs[0]
	if kf.FieldName != "id" {
		t.Errorf("FieldName = %q, want %q", kf.FieldName, "id")
	}
	if kf.StaticOffset != 0 {
		t.Errorf("StaticOffset = %d, want 0", kf.StaticOffset)
	}
	if kf.Size != 4 {
		t.Errorf("Size = %d, want 4", kf.Size)
	}
	if kf.TypeHint != "KeyInt32" {
		t.Errorf("TypeHint = %q, want %q", kf.TypeHint, "KeyInt32")
	}
}

func TestComputeKeyFields_StaticLater(t *testing.T) {
	// @key is not the first field but all preceding fields are fixed
	s := &ast.Struct{
		Name: "Msg",
		Fields: []ast.Field{
			{Name: "header", Type: &ast.BasicType{Name: "int32"}},
			{Name: "source", Type: &ast.ArrayType{ElemType: &ast.BasicType{Name: "octet"}, Size: 16},
				Annotations: []ast.Annotation{{Name: "key"}}},
		},
	}

	kfs := computeKeyFields(s)
	if len(kfs) != 1 {
		t.Fatalf("expected 1 key field, got %d", len(kfs))
	}
	if kfs[0].StaticOffset != 4 {
		t.Errorf("StaticOffset = %d, want 4", kfs[0].StaticOffset)
	}
	if kfs[0].TypeHint != "KeyUUID" {
		t.Errorf("TypeHint = %q, want %q", kfs[0].TypeHint, "KeyUUID")
	}
}

func TestComputeKeyFields_RuntimeOptional(t *testing.T) {
	// @optional before @key forces runtime
	s := &ast.Struct{
		Name: "Msg",
		Fields: []ast.Field{
			{Name: "name", Type: &ast.StringType{}, Annotations: []ast.Annotation{{Name: "optional"}}},
			{Name: "source", Type: &ast.ArrayType{ElemType: &ast.BasicType{Name: "octet"}, Size: 32},
				Annotations: []ast.Annotation{{Name: "key"}}},
		},
	}

	kfs := computeKeyFields(s)
	if len(kfs) != 1 {
		t.Fatalf("expected 1 key field, got %d", len(kfs))
	}
	if kfs[0].StaticOffset != -1 {
		t.Errorf("StaticOffset = %d, want -1 (runtime)", kfs[0].StaticOffset)
	}
}

func TestComputeKeyFields_Mutable(t *testing.T) {
	// MUTABLE always requires runtime
	s := &ast.Struct{
		Name: "Msg",
		Annotations: []ast.Annotation{{Name: "mutable"}},
		Fields: []ast.Field{
			{Name: "id", Type: &ast.BasicType{Name: "int32"}, Annotations: []ast.Annotation{{Name: "key"}}},
		},
	}

	kfs := computeKeyFields(s)
	if len(kfs) != 1 {
		t.Fatalf("expected 1 key field, got %d", len(kfs))
	}
	if kfs[0].StaticOffset != -1 {
		t.Errorf("StaticOffset = %d, want -1 (runtime)", kfs[0].StaticOffset)
	}
}

func TestComputeKeyFields_Appendable(t *testing.T) {
	// APPENDABLE adds 4 bytes for DHEADER
	s := &ast.Struct{
		Name:        "Msg",
		Annotations: []ast.Annotation{{Name: "appendable"}},
		Fields: []ast.Field{
			{Name: "id", Type: &ast.BasicType{Name: "int32"}, Annotations: []ast.Annotation{{Name: "key"}}},
		},
	}

	kfs := computeKeyFields(s)
	if len(kfs) != 1 {
		t.Fatalf("expected 1 key field, got %d", len(kfs))
	}
	if kfs[0].StaticOffset != 4 {
		t.Errorf("StaticOffset = %d, want 4 (DHEADER)", kfs[0].StaticOffset)
	}
}

func TestComputeKeyFields_NoKey(t *testing.T) {
	s := &ast.Struct{
		Name: "Simple",
		Fields: []ast.Field{
			{Name: "x", Type: &ast.BasicType{Name: "int32"}},
		},
	}
	kfs := computeKeyFields(s)
	if len(kfs) != 0 {
		t.Errorf("expected 0 key fields, got %d", len(kfs))
	}
}

func TestAllFields_NoInheritance(t *testing.T) {
	s := &ast.Struct{
		Name: "Simple",
		Fields: []ast.Field{
			{Name: "a", Type: &ast.BasicType{Name: "int32"}},
		},
	}
	fields, resolved := allFields(s)
	if !resolved {
		t.Error("expected resolved=true for non-inherited struct")
	}
	if len(fields) != 1 {
		t.Errorf("expected 1 field, got %d", len(fields))
	}
}

func TestAllFields_WithInheritance(t *testing.T) {
	s := &ast.Struct{
		Name:     "Derived",
		Inherits: "Base",
		Fields: []ast.Field{
			{Name: "z", Type: &ast.BasicType{Name: "int32"}},
		},
	}
	fields, resolved := allFields(s)
	if resolved {
		t.Error("expected resolved=false for inherited struct (base not resolved)")
	}
	if len(fields) != 1 {
		t.Errorf("expected 1 field (own only), got %d", len(fields))
	}
}

func TestEmFieldSize(t *testing.T) {
	tests := []struct {
		lc      uint8
		nextInt uint32
		want    uint32
	}{
		{0, 0, 1},
		{1, 0, 2},
		{2, 0, 4},
		{3, 0, 8},
		{4, 100, 100},
		{5, 200, 200},
		{6, 300, 300},
		{7, 400, 400},
	}
	for _, tt := range tests {
		got := emFieldSize(tt.lc, tt.nextInt)
		if got != tt.want {
			t.Errorf("emFieldSize(%d, %d) = %d, want %d", tt.lc, tt.nextInt, got, tt.want)
		}
	}
}

func TestNeedsRuntimeKeyExtract(t *testing.T) {
	static := []keyFieldInfo{{StaticOffset: 0}, {StaticOffset: 4}}
	if needsRuntimeKeyExtract(static) {
		t.Error("expected false for all-static key fields")
	}

	runtime := []keyFieldInfo{{StaticOffset: 0}, {StaticOffset: -1}}
	if !needsRuntimeKeyExtract(runtime) {
		t.Error("expected true when any key field needs runtime")
	}

	if needsRuntimeKeyExtract(nil) {
		t.Error("expected false for nil key fields")
	}
}
