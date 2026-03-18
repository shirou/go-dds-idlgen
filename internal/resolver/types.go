package resolver

import (
	"fmt"
	"strings"

	"github.com/shirou/go-dds-idlgen/internal/ast"
)

// Scope represents a namespace scope for type resolution.
type Scope struct {
	Name     string
	Parent   *Scope
	Children map[string]*Scope
	Types    map[string]ast.Definition // types defined in this scope
}

// NewScope creates a new root scope.
func NewScope() *Scope {
	return &Scope{
		Children: make(map[string]*Scope),
		Types:    make(map[string]ast.Definition),
	}
}

// Child returns or creates a child scope with the given name.
func (s *Scope) Child(name string) *Scope {
	if child, ok := s.Children[name]; ok {
		return child
	}
	child := &Scope{
		Name:     name,
		Parent:   s,
		Children: make(map[string]*Scope),
		Types:    make(map[string]ast.Definition),
	}
	s.Children[name] = child
	return child
}

// Lookup looks up a type name in this scope and parent scopes.
func (s *Scope) Lookup(name string) (ast.Definition, bool) {
	// Check current scope
	if def, ok := s.Types[name]; ok {
		return def, true
	}
	// Check parent
	if s.Parent != nil {
		return s.Parent.Lookup(name)
	}
	return nil, false
}

// LookupScoped looks up a scoped name like "module::Type".
func (s *Scope) LookupScoped(name string) (ast.Definition, bool) {
	parts := strings.Split(name, "::")
	if len(parts) == 1 {
		return s.Lookup(name)
	}

	// Navigate to the target scope from root
	root := s
	for root.Parent != nil {
		root = root.Parent
	}

	current := root
	for _, part := range parts[:len(parts)-1] {
		child, ok := current.Children[part]
		if !ok {
			return nil, false
		}
		current = child
	}

	typeName := parts[len(parts)-1]
	if def, ok := current.Types[typeName]; ok {
		return def, true
	}
	return nil, false
}

// TypeResolver resolves NamedType references in an AST.
type TypeResolver struct {
	root *Scope
}

// NewTypeResolver creates a new TypeResolver.
func NewTypeResolver() *TypeResolver {
	return &TypeResolver{
		root: NewScope(),
	}
}

// BuildScope builds the scope tree from a file's definitions.
func (r *TypeResolver) BuildScope(file *ast.File) {
	r.buildScopeDefs(r.root, file.Definitions)
}

func (r *TypeResolver) buildScopeDefs(scope *Scope, defs []ast.Definition) {
	for _, def := range defs {
		switch d := def.(type) {
		case *ast.Module:
			child := scope.Child(d.Name)
			r.buildScopeDefs(child, d.Definitions)
		case *ast.Struct:
			scope.Types[d.Name] = d
		case *ast.Enum:
			scope.Types[d.Name] = d
		case *ast.Typedef:
			scope.Types[d.Name] = d
		case *ast.SkippedDecl:
			if d.Name != "" {
				scope.Types[d.Name] = d
			}
		}
	}
}

// Resolve resolves all NamedType references in the file's definitions.
func (r *TypeResolver) Resolve(file *ast.File) error {
	return r.resolveDefs(r.root, file.Definitions)
}

func (r *TypeResolver) resolveDefs(scope *Scope, defs []ast.Definition) error {
	for _, def := range defs {
		switch d := def.(type) {
		case *ast.Module:
			child, ok := scope.Children[d.Name]
			if !ok {
				return fmt.Errorf("internal error: module %s not in scope tree", d.Name)
			}
			if err := r.resolveDefs(child, d.Definitions); err != nil {
				return err
			}
		case *ast.Struct:
			for i := range d.Fields {
				if err := r.resolveTypeRef(scope, &d.Fields[i].Type); err != nil {
					return fmt.Errorf("field %s.%s: %w", d.Name, d.Fields[i].Name, err)
				}
			}
		case *ast.Typedef:
			if err := r.resolveTypeRef(scope, &d.Type); err != nil {
				return fmt.Errorf("typedef %s: %w", d.Name, err)
			}
		}
	}
	return nil
}

func (r *TypeResolver) resolveTypeRef(scope *Scope, ref *ast.TypeRef) error {
	switch t := (*ref).(type) {
	case *ast.NamedType:
		def, ok := scope.LookupScoped(t.Name)
		if !ok {
			return fmt.Errorf("unresolved type: %s", t.Name)
		}
		t.Resolved = def
	case *ast.SequenceType:
		return r.resolveTypeRef(scope, &t.ElemType)
	case *ast.ArrayType:
		return r.resolveTypeRef(scope, &t.ElemType)
	}
	return nil
}

// Root returns the root scope for inspection.
func (r *TypeResolver) Root() *Scope {
	return r.root
}
