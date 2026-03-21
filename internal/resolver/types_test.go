package resolver

import (
	"strings"
	"testing"

	"github.com/shirou/go-dds-idlgen/internal/ast"
)

func TestScope_Child(t *testing.T) {
	root := NewScope()
	child := root.Child("mymod")
	if child.Name != "mymod" {
		t.Fatalf("expected child name 'mymod', got %q", child.Name)
	}
	if child.Parent != root {
		t.Fatal("expected child parent to be root")
	}

	// Getting the same child again should return the same scope
	child2 := root.Child("mymod")
	if child != child2 {
		t.Fatal("expected same child scope on repeated call")
	}
}

func TestScope_Lookup(t *testing.T) {
	root := NewScope()
	s := &ast.Struct{Name: "Point"}
	root.Types["Point"] = s

	def, ok := root.Lookup("Point")
	if !ok {
		t.Fatal("expected to find Point")
	}
	if def != s {
		t.Fatal("expected same struct pointer")
	}

	_, ok = root.Lookup("Missing")
	if ok {
		t.Fatal("expected Missing to not be found")
	}
}

func TestScope_LookupParent(t *testing.T) {
	root := NewScope()
	s := &ast.Struct{Name: "Point"}
	root.Types["Point"] = s

	child := root.Child("mymod")

	// Lookup from child should find type in parent
	def, ok := child.Lookup("Point")
	if !ok {
		t.Fatal("expected to find Point from child scope")
	}
	if def != s {
		t.Fatal("expected same struct pointer")
	}
}

func TestScope_LookupScoped(t *testing.T) {
	root := NewScope()
	child := root.Child("geometry")
	s := &ast.Struct{Name: "Point"}
	child.Types["Point"] = s

	// Lookup from root using scoped name
	def, ok := root.LookupScoped("geometry::Point")
	if !ok {
		t.Fatal("expected to find geometry::Point")
	}
	if def != s {
		t.Fatal("expected same struct pointer")
	}

	// Lookup from a different child using scoped name
	otherChild := root.Child("other")
	def, ok = otherChild.LookupScoped("geometry::Point")
	if !ok {
		t.Fatal("expected to find geometry::Point from other scope")
	}
	if def != s {
		t.Fatal("expected same struct pointer")
	}

	// Non-existent scoped name
	_, ok = root.LookupScoped("nonexistent::Type")
	if ok {
		t.Fatal("expected nonexistent::Type to not be found")
	}
}

func TestTypeResolver_BuildScope(t *testing.T) {
	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Module{
				Name: "geometry",
				Definitions: []ast.Definition{
					&ast.Struct{Name: "Point", Fields: []ast.Field{
						{Name: "x", Type: &ast.BasicType{Name: "int32"}},
						{Name: "y", Type: &ast.BasicType{Name: "int32"}},
					}},
					&ast.Enum{Name: "Color", Values: []ast.EnumValue{
						{Name: "RED"},
						{Name: "GREEN"},
						{Name: "BLUE"},
					}},
				},
			},
			&ast.Typedef{Name: "MyInt", Type: &ast.BasicType{Name: "int32"}},
		},
	}

	r := NewTypeResolver()
	r.BuildScope(file)

	root := r.Root()

	// Check root-level typedef
	_, ok := root.Types["MyInt"]
	if !ok {
		t.Fatal("expected MyInt in root scope")
	}

	// Check module scope
	geom, ok := root.Children["geometry"]
	if !ok {
		t.Fatal("expected geometry child scope")
	}

	_, ok = geom.Types["Point"]
	if !ok {
		t.Fatal("expected Point in geometry scope")
	}

	_, ok = geom.Types["Color"]
	if !ok {
		t.Fatal("expected Color in geometry scope")
	}
}

func TestTypeResolver_Resolve(t *testing.T) {
	pointStruct := &ast.Struct{Name: "Point", Fields: []ast.Field{
		{Name: "x", Type: &ast.BasicType{Name: "int32"}},
		{Name: "y", Type: &ast.BasicType{Name: "int32"}},
	}}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			pointStruct,
			&ast.Struct{Name: "Line", Fields: []ast.Field{
				{Name: "start", Type: &ast.NamedType{Name: "Point"}},
				{Name: "end", Type: &ast.NamedType{Name: "Point"}},
			}},
		},
	}

	r := NewTypeResolver()
	r.BuildScope(file)
	err := r.Resolve(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that NamedType references were resolved
	lineStruct := file.Definitions[1].(*ast.Struct)
	startType := lineStruct.Fields[0].Type.(*ast.NamedType)
	if startType.Resolved != pointStruct {
		t.Fatal("expected start field type to be resolved to Point struct")
	}
	endType := lineStruct.Fields[1].Type.(*ast.NamedType)
	if endType.Resolved != pointStruct {
		t.Fatal("expected end field type to be resolved to Point struct")
	}
}

func TestTypeResolver_ResolveScoped(t *testing.T) {
	pointStruct := &ast.Struct{Name: "Point", Fields: []ast.Field{
		{Name: "x", Type: &ast.BasicType{Name: "int32"}},
	}}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Module{
				Name: "geometry",
				Definitions: []ast.Definition{
					pointStruct,
				},
			},
			&ast.Struct{Name: "Drawing", Fields: []ast.Field{
				{Name: "origin", Type: &ast.NamedType{Name: "geometry::Point"}},
			}},
		},
	}

	r := NewTypeResolver()
	r.BuildScope(file)
	err := r.Resolve(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	drawingStruct := file.Definitions[1].(*ast.Struct)
	originType := drawingStruct.Fields[0].Type.(*ast.NamedType)
	if originType.Resolved != pointStruct {
		t.Fatal("expected origin field to be resolved to geometry::Point")
	}
}

func TestTypeResolver_ResolveSequence(t *testing.T) {
	pointStruct := &ast.Struct{Name: "Point", Fields: []ast.Field{
		{Name: "x", Type: &ast.BasicType{Name: "int32"}},
	}}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			pointStruct,
			&ast.Struct{Name: "Path", Fields: []ast.Field{
				{Name: "points", Type: &ast.SequenceType{
					ElemType: &ast.NamedType{Name: "Point"},
				}},
			}},
		},
	}

	r := NewTypeResolver()
	r.BuildScope(file)
	err := r.Resolve(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pathStruct := file.Definitions[1].(*ast.Struct)
	seqType := pathStruct.Fields[0].Type.(*ast.SequenceType)
	namedType := seqType.ElemType.(*ast.NamedType)
	if namedType.Resolved != pointStruct {
		t.Fatal("expected sequence element type to be resolved to Point")
	}
}

func TestTypeResolver_ResolveArray(t *testing.T) {
	pointStruct := &ast.Struct{Name: "Point", Fields: []ast.Field{
		{Name: "x", Type: &ast.BasicType{Name: "int32"}},
	}}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			pointStruct,
			&ast.Struct{Name: "Triangle", Fields: []ast.Field{
				{Name: "vertices", Type: &ast.ArrayType{
					ElemType: &ast.NamedType{Name: "Point"},
					Size:     3,
				}},
			}},
		},
	}

	r := NewTypeResolver()
	r.BuildScope(file)
	err := r.Resolve(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	triStruct := file.Definitions[1].(*ast.Struct)
	arrType := triStruct.Fields[0].Type.(*ast.ArrayType)
	namedType := arrType.ElemType.(*ast.NamedType)
	if namedType.Resolved != pointStruct {
		t.Fatal("expected array element type to be resolved to Point")
	}
}

func TestTypeResolver_ResolveTypedef(t *testing.T) {
	pointStruct := &ast.Struct{Name: "Point", Fields: []ast.Field{
		{Name: "x", Type: &ast.BasicType{Name: "int32"}},
	}}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			pointStruct,
			&ast.Typedef{Name: "Vertex", Type: &ast.NamedType{Name: "Point"}},
		},
	}

	r := NewTypeResolver()
	r.BuildScope(file)
	err := r.Resolve(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	td := file.Definitions[1].(*ast.Typedef)
	namedType := td.Type.(*ast.NamedType)
	if namedType.Resolved != pointStruct {
		t.Fatal("expected typedef type to be resolved to Point")
	}
}

func TestTypeResolver_ResolveUnion(t *testing.T) {
	colorEnum := &ast.Enum{Name: "Color", Values: []ast.EnumValue{
		{Name: "RED"},
		{Name: "GREEN"},
	}}
	pointStruct := &ast.Struct{Name: "Point", Fields: []ast.Field{
		{Name: "x", Type: &ast.BasicType{Name: "int32"}},
	}}

	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			colorEnum,
			pointStruct,
			&ast.Union{
				Name:          "MyUnion",
				Discriminator: &ast.NamedType{Name: "Color"},
				Cases: []ast.UnionCase{
					{
						Labels: []string{"RED"},
						Type:   &ast.NamedType{Name: "Point"},
						Name:   "pointVal",
					},
					{
						Labels: []string{"GREEN"},
						Type:   &ast.BasicType{Name: "int32"},
						Name:   "intVal",
					},
				},
			},
		},
	}

	r := NewTypeResolver()
	r.BuildScope(file)
	err := r.Resolve(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := file.Definitions[2].(*ast.Union)
	// discriminator resolved
	discType := u.Discriminator.(*ast.NamedType)
	if discType.Resolved != colorEnum {
		t.Fatal("expected discriminator to be resolved to Color enum")
	}
	// case type resolved
	caseType := u.Cases[0].Type.(*ast.NamedType)
	if caseType.Resolved != pointStruct {
		t.Fatal("expected case type to be resolved to Point struct")
	}

	// union itself is registered as a type
	_, ok := r.Root().Types["MyUnion"]
	if !ok {
		t.Fatal("expected MyUnion to be registered in scope")
	}
}

func TestTypeResolver_UnresolvedError(t *testing.T) {
	file := &ast.File{
		Name: "test.idl",
		Definitions: []ast.Definition{
			&ast.Struct{Name: "Line", Fields: []ast.Field{
				{Name: "start", Type: &ast.NamedType{Name: "Point"}},
			}},
		},
	}

	r := NewTypeResolver()
	r.BuildScope(file)
	err := r.Resolve(file)
	if err == nil {
		t.Fatal("expected unresolved type error")
	}
	if !strings.Contains(err.Error(), "unresolved type") {
		t.Fatalf("expected 'unresolved type' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Point") {
		t.Fatalf("expected error to mention 'Point', got: %v", err)
	}
}
