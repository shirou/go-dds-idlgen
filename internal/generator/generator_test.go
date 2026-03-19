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
	// Check marshal methods
	for _, method := range []string{"MarshalCDR", "UnmarshalCDR", "EncodeCDR", "DecodeCDR", "CDRExtensibility"} {
		if !strings.Contains(src, method) {
			t.Errorf("expected %s method in output:\n%s", method, src)
		}
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

func TestGenerate_EnumMixedValues(t *testing.T) {
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
				Name: "Status",
				Values: []ast.EnumValue{
					{Name: "OK", Value: &val0},
					{Name: "WARN"},
					{Name: "ERROR", Value: &val5},
					{Name: "FATAL"},
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
	// Verify that values after an explicit value are computed correctly
	// (not using iota which would give wrong values).
	// go/format may add alignment spaces, so check key fragments.
	for _, want := range []struct {
		name string
		val  string
	}{
		{"StatusOK", "= 0"},
		{"StatusWARN", "= 1"},
		{"StatusERROR", "= 5"},
		{"StatusFATAL", "= 6"},
	} {
		if !strings.Contains(src, want.name) || !strings.Contains(src, want.val) {
			t.Errorf("expected '%s ... %s' in output:\n%s", want.name, want.val, src)
		}
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
	if !strings.Contains(src, "FinishDHeader") {
		t.Errorf("expected FinishDHeader in MUTABLE output:\n%s", src)
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

func TestGenerate_CrossPackageTypeRef(t *testing.T) {
	g, err := New(Config{
		OutputDir:     "/tmp/test",
		PackagePrefix: "example.com/project/gen",
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Types in Org::Common::Measurement (depth 3) and Org::Other::Sensors (depth 3)
	// should be in different flattened packages: org/common and org/other.
	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Module{
				Name: "Org",
				Definitions: []ast.Definition{
					&ast.Module{
						Name: "Common",
						Definitions: []ast.Definition{
							&ast.Module{
								Name: "Measurement",
								Definitions: []ast.Definition{
									&ast.Struct{
										Name: "NumericGUID",
										Fields: []ast.Field{
											{Name: "value", Type: &ast.BasicType{Name: "uint64"}},
										},
									},
								},
							},
						},
					},
					&ast.Module{
						Name: "Other",
						Definitions: []ast.Definition{
							&ast.Module{
								Name: "Sensors",
								Definitions: []ast.Definition{
									&ast.Struct{
										Name: "IdentifierType",
										Fields: []ast.Field{
											{Name: "id", Type: &ast.NamedType{Name: "Org::Common::Measurement::NumericGUID"}},
										},
									},
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

	// NumericGUID (Org::Common::Measurement) flattens to org/common
	_, ok := result["org/common"]
	if !ok {
		keys := make([]string, 0, len(result))
		for k := range result {
			keys = append(keys, k)
		}
		t.Fatalf("expected output for 'org/common', got keys: %v", keys)
	}

	// IdentifierType (Org::Other::Sensors) flattens to org/other
	data, ok := result["org/other"]
	if !ok {
		keys := make([]string, 0, len(result))
		for k := range result {
			keys = append(keys, k)
		}
		t.Fatalf("expected output for 'org/other', got keys: %v", keys)
	}

	src := string(data)

	// Should use package-qualified type reference to the common package
	if !strings.Contains(src, "common.NumericGUID") {
		t.Errorf("expected 'common.NumericGUID' in output:\n%s", src)
	}

	// Should have import for the common package
	if !strings.Contains(src, `"example.com/project/gen/org/common"`) {
		t.Errorf("expected common import in output:\n%s", src)
	}
}

func TestGenerate_FlattenSamePackage(t *testing.T) {
	g, err := New(Config{OutputDir: "/tmp/test"})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Types in sibling sub-modules (Org::Common::Measurement and Org::Common::Enumeration)
	// should flatten into the same package (org/common).
	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Module{
				Name: "Org",
				Definitions: []ast.Definition{
					&ast.Module{
						Name: "Common",
						Definitions: []ast.Definition{
							&ast.Module{
								Name: "Measurement",
								Definitions: []ast.Definition{
									&ast.Struct{
										Name: "NumericGUID",
										Fields: []ast.Field{
											{Name: "value", Type: &ast.BasicType{Name: "uint64"}},
										},
									},
								},
							},
							&ast.Module{
								Name: "Enumeration",
								Definitions: []ast.Definition{
									&ast.Struct{
										Name: "SensorReading",
										Fields: []ast.Field{
											{Name: "id", Type: &ast.NamedType{Name: "Org::Common::Measurement::NumericGUID"}},
										},
									},
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

	// Both should be in the same package org/common
	if len(result) != 1 {
		keys := make([]string, 0, len(result))
		for k := range result {
			keys = append(keys, k)
		}
		t.Fatalf("expected 1 package, got %d: %v", len(result), keys)
	}

	data, ok := result["org/common"]
	if !ok {
		t.Fatal("expected output for 'org/common'")
	}

	src := string(data)

	// Same-package reference should NOT have package qualifier
	if strings.Contains(src, "common.NumericGUID") {
		t.Errorf("same-package type should not have package qualifier:\n%s", src)
	}
	// Should reference NumericGUID without qualifier
	if !strings.Contains(src, "ID NumericGUID") {
		t.Errorf("expected 'ID NumericGUID' field in output:\n%s", src)
	}
}

func TestGenerate_SamePackageTypeRef(t *testing.T) {
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
						Name: "Config",
						Fields: []ast.Field{
							{Name: "value", Type: &ast.BasicType{Name: "uint32"}},
						},
					},
					&ast.Struct{
						Name: "Reading",
						Fields: []ast.Field{
							{Name: "cfg", Type: &ast.NamedType{Name: "sensor::Config"}},
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

	data, ok := result["sensor"]
	if !ok {
		t.Fatal("expected output for 'sensor'")
	}

	src := string(data)

	// Same-package reference should NOT have package qualifier
	if strings.Contains(src, "sensor.Config") {
		t.Errorf("same-package type should not have package qualifier:\n%s", src)
	}
	// Should just be Config (PascalCase)
	if !strings.Contains(src, "Cfg Config") {
		t.Errorf("expected 'Cfg Config' field in output:\n%s", src)
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
