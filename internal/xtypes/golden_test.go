package xtypes

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/shirou/go-dds-idlgen/internal/ast"
)

// goldenEntry holds a parsed line from golden.txt.
type goldenEntry struct {
	Name  string
	Size  int
	Bytes []byte
}

func parseGoldenFile(t *testing.T, path string) []goldenEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open golden file: %v", err)
	}
	defer f.Close() //nolint:errcheck

	var entries []goldenEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var name string
		var size int
		var hexStr string
		n, _ := fmt.Sscanf(line, "%s size=%d bytes=%s", &name, &size, &hexStr)
		if n != 3 {
			continue
		}
		data, err := hex.DecodeString(hexStr)
		if err != nil {
			t.Fatalf("decode hex for %s: %v", name, err)
		}
		entries = append(entries, goldenEntry{Name: name, Size: size, Bytes: data})
	}
	return entries
}

// buildTestTypes constructs AST types matching testdata/typeinfo/test_types.idl.
func buildTestTypes() map[string]func(ctx *ComputeContext) ([]byte, error) {
	scope := []string{"test"}

	val0 := int64(0)
	val1 := int64(1)
	val2 := int64(2)

	colorEnum := &ast.Enum{
		Name: "Color",
		Values: []ast.EnumValue{
			{Name: "RED", Value: &val0},
			{Name: "GREEN", Value: &val1},
			{Name: "BLUE", Value: &val2},
		},
	}

	innerStruct := &ast.Struct{
		Name:        "InnerType",
		Annotations: []ast.Annotation{{Name: "final"}},
		Fields: []ast.Field{
			{Name: "x", Type: &ast.BasicType{Name: "long"}},
			{Name: "y", Type: &ast.BasicType{Name: "long"}},
		},
	}

	return map[string]func(ctx *ComputeContext) ([]byte, error){
		"SimpleType": func(ctx *ComputeContext) ([]byte, error) {
			s := &ast.Struct{
				Name:        "SimpleType",
				Annotations: []ast.Annotation{{Name: "final"}},
				Fields: []ast.Field{
					{Name: "id", Type: &ast.BasicType{Name: "long"}},
					{Name: "value", Type: &ast.BasicType{Name: "double"}},
					{Name: "name", Type: &ast.StringType{Bound: 0}},
				},
			}
			return ctx.BuildTypeInformation(s, scope)
		},
		"SensorData": func(ctx *ComputeContext) ([]byte, error) {
			s := &ast.Struct{
				Name:        "SensorData",
				Annotations: []ast.Annotation{{Name: "final"}},
				Fields: []ast.Field{
					{Name: "sensor_id", Type: &ast.BasicType{Name: "long"},
						Annotations: []ast.Annotation{{Name: "key"}}},
					{Name: "temperature", Type: &ast.BasicType{Name: "double"}},
					{Name: "humidity", Type: &ast.BasicType{Name: "float"}},
					{Name: "location", Type: &ast.StringType{Bound: 0}},
					{Name: "active", Type: &ast.BasicType{Name: "boolean"}},
				},
			}
			return ctx.BuildTypeInformation(s, scope)
		},
		"ArrayType": func(ctx *ComputeContext) ([]byte, error) {
			s := &ast.Struct{
				Name:        "ArrayType",
				Annotations: []ast.Annotation{{Name: "final"}},
				Fields: []ast.Field{
					{Name: "uuid", Type: &ast.ArrayType{
						ElemType: &ast.BasicType{Name: "octet"},
						Size:     16,
					}},
					{Name: "count", Type: &ast.BasicType{Name: "long"}},
				},
			}
			return ctx.BuildTypeInformation(s, scope)
		},
		"SequenceType": func(ctx *ComputeContext) ([]byte, error) {
			s := &ast.Struct{
				Name:        "SequenceType",
				Annotations: []ast.Annotation{{Name: "final"}},
				Fields: []ast.Field{
					{Name: "values", Type: &ast.SequenceType{
						ElemType: &ast.BasicType{Name: "double"},
						Bound:    0,
					}},
					{Name: "length", Type: &ast.BasicType{Name: "long"}},
				},
			}
			return ctx.BuildTypeInformation(s, scope)
		},
		"EnumStruct": func(ctx *ComputeContext) ([]byte, error) {
			// Pre-compute Color enum
			_, _ = ctx.ComputeTypeIdentifier(
				&ast.NamedType{Name: "Color", Resolved: colorEnum}, scope)
			_, _ = ctx.ComputeCompleteTypeIdentifier(
				&ast.NamedType{Name: "Color", Resolved: colorEnum}, scope)

			s := &ast.Struct{
				Name:        "EnumStruct",
				Annotations: []ast.Annotation{{Name: "final"}},
				Fields: []ast.Field{
					{Name: "color", Type: &ast.NamedType{Name: "Color", Resolved: colorEnum}},
					{Name: "intensity", Type: &ast.BasicType{Name: "long"}},
				},
			}
			return ctx.BuildTypeInformation(s, scope)
		},
		"OuterType": func(ctx *ComputeContext) ([]byte, error) {
			// Pre-compute InnerType
			_, _ = ctx.ComputeTypeIdentifier(
				&ast.NamedType{Name: "InnerType", Resolved: innerStruct}, scope)
			_, _ = ctx.ComputeCompleteTypeIdentifier(
				&ast.NamedType{Name: "InnerType", Resolved: innerStruct}, scope)

			s := &ast.Struct{
				Name:        "OuterType",
				Annotations: []ast.Annotation{{Name: "final"}},
				Fields: []ast.Field{
					{Name: "position", Type: &ast.NamedType{Name: "InnerType", Resolved: innerStruct}},
					{Name: "timestamp", Type: &ast.BasicType{Name: "double"}},
				},
			}
			return ctx.BuildTypeInformation(s, scope)
		},
		"AppendableType": func(ctx *ComputeContext) ([]byte, error) {
			s := &ast.Struct{
				Name:        "AppendableType",
				Annotations: []ast.Annotation{{Name: "appendable"}},
				Fields: []ast.Field{
					{Name: "id", Type: &ast.BasicType{Name: "long"}},
					{Name: "label", Type: &ast.StringType{Bound: 0}},
				},
			}
			return ctx.BuildTypeInformation(s, scope)
		},
		"MutableType": func(ctx *ComputeContext) ([]byte, error) {
			s := &ast.Struct{
				Name:        "MutableType",
				Annotations: []ast.Annotation{{Name: "mutable"}},
				Fields: []ast.Field{
					{Name: "id", Type: &ast.BasicType{Name: "long"}},
					{Name: "description", Type: &ast.StringType{Bound: 0},
						Annotations: []ast.Annotation{{Name: "optional"}}},
				},
			}
			return ctx.BuildTypeInformation(s, scope)
		},
		"BoundedStringType": func(ctx *ComputeContext) ([]byte, error) {
			s := &ast.Struct{
				Name:        "BoundedStringType",
				Annotations: []ast.Annotation{{Name: "final"}},
				Fields: []ast.Field{
					{Name: "short_name", Type: &ast.StringType{Bound: 64}},
					{Name: "long_name", Type: &ast.StringType{Bound: 1024}},
				},
			}
			return ctx.BuildTypeInformation(s, scope)
		},
	}
}

func TestGoldenTypeInformation(t *testing.T) {
	goldenPath := "../../testdata/typeinfo/golden.txt"
	entries := parseGoldenFile(t, goldenPath)
	if len(entries) == 0 {
		t.Fatal("no entries parsed from golden file")
	}

	builders := buildTestTypes()

	for _, entry := range entries {
		t.Run(entry.Name, func(t *testing.T) {
			builder, ok := builders[entry.Name]
			if !ok {
				t.Skipf("no builder for %s", entry.Name)
				return
			}

			ctx := NewComputeContext()
			data, err := builder(ctx)
			if err != nil {
				t.Fatalf("BuildTypeInformation: %v", err)
			}

			if len(data) != entry.Size {
				t.Errorf("size mismatch: got %d, want %d", len(data), entry.Size)
			}

			// Compare bytes
			golden := entry.Bytes
			if len(data) != len(golden) {
				t.Errorf("length mismatch: got %d bytes, want %d bytes", len(data), len(golden))
				t.Logf("got:    %x", data)
				t.Logf("golden: %x", golden)
				return
			}

			mismatch := false
			for i := range data {
				if data[i] != golden[i] {
					if !mismatch {
						t.Errorf("first byte mismatch at offset %d: got 0x%02x, want 0x%02x",
							i, data[i], golden[i])
						mismatch = true
					}
				}
			}
			if mismatch {
				t.Logf("got:    %x", data)
				t.Logf("golden: %x", golden)
			}
		})
	}
}
