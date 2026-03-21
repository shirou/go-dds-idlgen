package testdata_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shirou/go-dds-idlgen/internal/generator"
	"github.com/shirou/go-dds-idlgen/internal/parser"
	"github.com/shirou/go-dds-idlgen/internal/resolver"
)

// TestGoldenFiles parses each testdata IDL file and verifies the generated
// Go code contains expected patterns.
func TestGoldenFiles(t *testing.T) {
	tests := []struct {
		name     string
		dir      string
		expected map[string][]string // package path -> expected substrings
	}{
		{
			name: "basic_struct",
			dir:  "basic_struct",
			expected: map[string][]string{
				"sensor": {
					"package sensor",
					"type SensorData struct",
					"SensorID",
					"Temperature",
					"Humidity",
					"Location",
					"Active",
					"float64",
					"float32",
					"string",
					"bool",
					"int32",
					"MarshalCDR",
					"UnmarshalCDR",
					"EncodeCDR",
					"DecodeCDR",
					"CDRExtensibility",
				},
			},
		},
		{
			name: "enum_typedef",
			dir:  "enum_typedef",
			expected: map[string][]string{
				"types": {
					"package types",
					"type Color int32",
					"ColorRED",
					"ColorGREEN",
					"ColorBLUE",
					"type Priority int32",
					"PriorityLOW",
					"PriorityMEDIUM",
					"PriorityHIGH",
					"type Timestamp =",
					"MaxItems",
					"type Item struct",
				},
			},
		},
		{
			name: "mutable_struct",
			dir:  "mutable_struct",
			expected: map[string][]string{
				"msg": {
					"package msg",
					"type FlexMessage struct",
					"MsgID",
					"Content",
					"Priority",
					"*float32", // optional
					"MarshalCDR",
					"UnmarshalCDR",
					"EncodeCDR",
					"DecodeCDR",
					"CDRExtensibility",
					"WriteEMHeader",
				},
			},
		},
		{
			name: "optional_fields",
			dir:  "optional_fields",
			expected: map[string][]string{
				"data": {
					"type Record struct",
					"*string",  // optional description
					"*float64", // optional score
				},
			},
		},
		{
			name: "sequence_array",
			dir:  "sequence_array",
			expected: map[string][]string{
				"collection": {
					"type Matrix struct",
					"[]float64",   // sequence<double>
					"[3]float32",  // float values[3]
					"[]string",    // sequence<string>
				},
			},
		},
		{
			name: "union_type",
			dir:  "union_type",
			expected: map[string][]string{
				"variant": {
					"package variant",
					"type ShapeKind int32",
					"ShapeKindCIRCLE_D",
					"ShapeKindRECTANGLE_D",
					"ShapeKindTRIANGLE_D",
					"isShapeUnionValue",
					"type ShapeUnion struct",
					"ShapeUnion_CircleVariant",
					"ShapeUnion_RectangleVariant",
					"ShapeUnion_TriangleVariant",
					"EncodeCDR",
					"DecodeCDR",
					"WriteUint32",
					"ReadUint32",
					"type ShapeMessage struct",
				},
			},
		},
		{
			name: "appendable_struct",
			dir:  "appendable_struct",
			expected: map[string][]string{
				"event": {
					"type Event struct",
					"EventID",
					"Topic",
					"Timestamp",
					"Payload",
					"[64]uint8",
					"MarshalCDR",
					"UnmarshalCDR",
					"EncodeCDR",
					"DecodeCDR",
					"CDRExtensibility",
					"BeginDHeader",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idlPath := filepath.Join(tt.dir, "input.idl")
			data, err := os.ReadFile(idlPath)
			if err != nil {
				t.Fatalf("read IDL file: %v", err)
			}

			file, err := parser.Parse(filepath.Base(idlPath), data)
			if err != nil {
				t.Fatalf("parse IDL: %v", err)
			}

			// Resolve types so that NamedType references are linked.
			typeResolver := resolver.NewTypeResolver()
			typeResolver.BuildScope(file)
			if err := typeResolver.Resolve(file); err != nil {
				t.Fatalf("resolve types: %v", err)
			}

			gen, err := generator.New(generator.Config{
				OutputDir: t.TempDir(),
			})
			if err != nil {
				t.Fatalf("create generator: %v", err)
			}

			result, err := gen.GenerateToBuffer(file)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}

			for pkgPath, expectedStrs := range tt.expected {
				src, ok := result[pkgPath]
				if !ok {
					keys := make([]string, 0, len(result))
					for k := range result {
						keys = append(keys, k)
					}
					t.Fatalf("expected package path %q in result, got: %v", pkgPath, keys)
				}

				srcStr := string(src)
				for _, exp := range expectedStrs {
					if !strings.Contains(srcStr, exp) {
						t.Errorf("expected %q in generated output for %s:\n%s", exp, pkgPath, srcStr)
					}
				}
			}
		})
	}
}
