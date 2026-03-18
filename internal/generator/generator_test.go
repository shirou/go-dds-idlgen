package generator

import (
	"go/format"
	"strings"
	"testing"

	"github.com/shirou/go-dds-idlgen/internal/ast"
)

func TestNew(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if g == nil {
		t.Fatal("New() returned nil generator")
	}
	if g.templates == nil {
		t.Fatal("New() generator has nil templates")
	}
}

func TestGenerate_SimpleStruct(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Struct{
				Name: "sensor_data",
				Fields: []ast.Field{
					{Name: "sensor_id", Type: &ast.BasicType{Name: "uint32"}},
					{Name: "temperature", Type: &ast.BasicType{Name: "double"}},
					{Name: "label", Type: &ast.StringType{}},
				},
			},
		},
	}

	result, err := g.GenerateToBuffer(file)
	if err != nil {
		t.Fatalf("GenerateToBuffer() error: %v", err)
	}

	data, ok := result["."]
	if !ok {
		t.Fatal("expected output for '.' package path")
	}

	src := string(data)

	// Verify it is valid Go source (go/format would have failed in renderUnit)
	if _, err := format.Source(data); err != nil {
		t.Fatalf("generated source is not valid Go: %v\nsource:\n%s", err, src)
	}

	// Check the struct is defined
	if !strings.Contains(src, "type SensorData struct") {
		t.Errorf("expected struct definition 'type SensorData struct' in output:\n%s", src)
	}
	// Check fields
	if !strings.Contains(src, "SensorID") {
		t.Errorf("expected field SensorID in output:\n%s", src)
	}
	if !strings.Contains(src, "Temperature") {
		t.Errorf("expected field Temperature in output:\n%s", src)
	}
	if !strings.Contains(src, "Label") {
		t.Errorf("expected field Label in output:\n%s", src)
	}
	// Check marshal method
	if !strings.Contains(src, "MarshalCDR") {
		t.Errorf("expected MarshalCDR method in output:\n%s", src)
	}
	if !strings.Contains(src, "UnmarshalCDR") {
		t.Errorf("expected UnmarshalCDR method in output:\n%s", src)
	}
}

func TestGenerate_ModuleFlattening(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Module{
				Name: "sensor",
				Definitions: []ast.Definition{
					&ast.Struct{
						Name: "Reading",
						Fields: []ast.Field{
							{Name: "value", Type: &ast.BasicType{Name: "float"}},
						},
					},
				},
			},
		},
	}

	result, err := g.GenerateToBuffer(file)
	if err != nil {
		t.Fatalf("GenerateToBuffer() error: %v", err)
	}

	// Should produce output for "sensor" package path
	data, ok := result["sensor"]
	if !ok {
		keys := make([]string, 0, len(result))
		for k := range result {
			keys = append(keys, k)
		}
		t.Fatalf("expected output for 'sensor' package path, got keys: %v", keys)
	}

	src := string(data)
	if !strings.Contains(src, "package sensor") {
		t.Errorf("expected 'package sensor' in output:\n%s", src)
	}
	if !strings.Contains(src, "type Reading struct") {
		t.Errorf("expected 'type Reading struct' in output:\n%s", src)
	}
}

func TestGenerate_Enum(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	val0 := int64(0)
	val5 := int64(5)
	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Enum{
				Name: "Color",
				Values: []ast.EnumValue{
					{Name: "RED", Value: &val0},
					{Name: "GREEN"},
					{Name: "BLUE", Value: &val5},
				},
			},
		},
	}

	result, err := g.GenerateToBuffer(file)
	if err != nil {
		t.Fatalf("GenerateToBuffer() error: %v", err)
	}

	data, ok := result["."]
	if !ok {
		t.Fatal("expected output for '.' package path")
	}

	src := string(data)
	if !strings.Contains(src, "type Color int32") {
		t.Errorf("expected 'type Color int32' in output:\n%s", src)
	}
	if !strings.Contains(src, "ColorRED") {
		t.Errorf("expected 'ColorRED' in output:\n%s", src)
	}
}

func TestGenerate_Typedef(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Typedef{
				Name: "timestamp",
				Type: &ast.BasicType{Name: "uint64"},
			},
		},
	}

	result, err := g.GenerateToBuffer(file)
	if err != nil {
		t.Fatalf("GenerateToBuffer() error: %v", err)
	}

	data, ok := result["."]
	if !ok {
		t.Fatal("expected output for '.' package path")
	}

	src := string(data)
	if !strings.Contains(src, "type Timestamp = uint64") {
		t.Errorf("expected 'type Timestamp = uint64' in output:\n%s", src)
	}
}

func TestGenerate_Const(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Const{
				Name:  "MAX_SIZE",
				Type:  &ast.BasicType{Name: "uint32"},
				Value: "256",
			},
		},
	}

	result, err := g.GenerateToBuffer(file)
	if err != nil {
		t.Fatalf("GenerateToBuffer() error: %v", err)
	}

	data, ok := result["."]
	if !ok {
		t.Fatal("expected output for '.' package path")
	}

	src := string(data)
	if !strings.Contains(src, "MaxSize") {
		t.Errorf("expected 'MaxSize' const in output:\n%s", src)
	}
	if !strings.Contains(src, "256") {
		t.Errorf("expected value '256' in output:\n%s", src)
	}
}

func TestGenerate_AppendableStruct(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Struct{
				Name: "Message",
				Fields: []ast.Field{
					{Name: "id", Type: &ast.BasicType{Name: "uint32"}},
					{Name: "text", Type: &ast.StringType{}},
				},
				Annotations: []ast.Annotation{
					{Name: "appendable"},
				},
			},
		},
	}

	result, err := g.GenerateToBuffer(file)
	if err != nil {
		t.Fatalf("GenerateToBuffer() error: %v", err)
	}

	data, ok := result["."]
	if !ok {
		t.Fatal("expected output for '.' package path")
	}

	src := string(data)
	// APPENDABLE uses DHEADER
	if !strings.Contains(src, "BeginDHeader") {
		t.Errorf("expected BeginDHeader in APPENDABLE output:\n%s", src)
	}
	if !strings.Contains(src, "FinishDHeader") {
		t.Errorf("expected FinishDHeader in APPENDABLE output:\n%s", src)
	}
	if !strings.Contains(src, "ReadDHeader") {
		t.Errorf("expected ReadDHeader in APPENDABLE output:\n%s", src)
	}
}

func TestGenerate_MutableStruct(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Struct{
				Name: "Config",
				Fields: []ast.Field{
					{Name: "version", Type: &ast.BasicType{Name: "uint32"}},
					{Name: "name", Type: &ast.StringType{}},
				},
				Annotations: []ast.Annotation{
					{Name: "mutable"},
				},
			},
		},
	}

	result, err := g.GenerateToBuffer(file)
	if err != nil {
		t.Fatalf("GenerateToBuffer() error: %v", err)
	}

	data, ok := result["."]
	if !ok {
		t.Fatal("expected output for '.' package path")
	}

	src := string(data)
	// MUTABLE uses EMHEADER and sentinel
	if !strings.Contains(src, "WriteEMHeader") {
		t.Errorf("expected WriteEMHeader in MUTABLE output:\n%s", src)
	}
	if !strings.Contains(src, "WritePLCDRSentinel") {
		t.Errorf("expected WritePLCDRSentinel in MUTABLE output:\n%s", src)
	}
	if !strings.Contains(src, "ReadEMHeader") {
		t.Errorf("expected ReadEMHeader in MUTABLE output:\n%s", src)
	}
}

func TestGenerate_SequenceField(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Struct{
				Name: "DataSet",
				Fields: []ast.Field{
					{Name: "values", Type: &ast.SequenceType{ElemType: &ast.BasicType{Name: "float"}}},
				},
			},
		},
	}

	result, err := g.GenerateToBuffer(file)
	if err != nil {
		t.Fatalf("GenerateToBuffer() error: %v", err)
	}

	data, ok := result["."]
	if !ok {
		t.Fatal("expected output for '.' package path")
	}

	src := string(data)
	if !strings.Contains(src, "[]float32") {
		t.Errorf("expected '[]float32' field type in output:\n%s", src)
	}
}

func TestGenerate_ArrayField(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Struct{
				Name: "Position",
				Fields: []ast.Field{
					{Name: "coords", Type: &ast.ArrayType{ElemType: &ast.BasicType{Name: "double"}, Size: 3}},
				},
			},
		},
	}

	result, err := g.GenerateToBuffer(file)
	if err != nil {
		t.Fatalf("GenerateToBuffer() error: %v", err)
	}

	data, ok := result["."]
	if !ok {
		t.Fatal("expected output for '.' package path")
	}

	src := string(data)
	if !strings.Contains(src, "[3]float64") {
		t.Errorf("expected '[3]float64' field type in output:\n%s", src)
	}
}

func TestGenerate_NestedModules(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Module{
				Name: "org",
				Definitions: []ast.Definition{
					&ast.Module{
						Name: "example",
						Definitions: []ast.Definition{
							&ast.Struct{
								Name: "Point",
								Fields: []ast.Field{
									{Name: "x", Type: &ast.BasicType{Name: "float"}},
									{Name: "y", Type: &ast.BasicType{Name: "float"}},
								},
							},
						},
					},
				},
			},
		},
	}

	result, err := g.GenerateToBuffer(file)
	if err != nil {
		t.Fatalf("GenerateToBuffer() error: %v", err)
	}

	// Nested module should be at org/example
	data, ok := result["org/example"]
	if !ok {
		keys := make([]string, 0, len(result))
		for k := range result {
			keys = append(keys, k)
		}
		t.Fatalf("expected output for 'org/example' package path, got keys: %v", keys)
	}

	src := string(data)
	if !strings.Contains(src, "package example") {
		t.Errorf("expected 'package example' in output:\n%s", src)
	}
}

func TestGenerate_EmptyFile(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	file := &ast.File{
		Name:        "empty.idl",
		Definitions: nil,
	}

	result, err := g.GenerateToBuffer(file)
	if err != nil {
		t.Fatalf("GenerateToBuffer() error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty result for empty file, got %d entries", len(result))
	}
}
