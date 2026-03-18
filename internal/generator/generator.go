package generator

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/shirou/go-dds-idlgen/internal/ast"
)

//go:embed templates/*.go.tmpl
var templateFS embed.FS

// Config holds code generation configuration.
type Config struct {
	PackagePrefix string // Go package prefix (e.g., "github.com/user/project")
	ModulePath    string // Go module path
	OutputDir     string // output directory
}

// genUnit represents a single Go package to generate.
type genUnit struct {
	PackageName string
	PackagePath string
	ModulePath  []string    // IDL module path, e.g., ["Org", "Common"]
	Imports     []pkgImport // cross-package imports needed
	Structs     []*ast.Struct
	Enums       []*ast.Enum
	Typedefs    []*ast.Typedef
	Consts      []*ast.Const
}

// pkgImport represents a Go import for a cross-package type reference.
type pkgImport struct {
	Path string // full import path
}

// Generator generates Go code from IDL AST.
type Generator struct {
	config        Config
	templates     *template.Template
	packagePrefix string // resolved Go import prefix for generated packages
}

// New creates a new Generator.
func New(cfg Config) (*Generator, error) {
	funcMap := template.FuncMap{
		"goType":            goType,
		"goBasicType":       goBasicType,
		"goFieldType":       goFieldType,
		"pascalCase":        pascalCase,
		"camelCase":         camelCase,
		"snakeCase":         snakeCase,
		"hasAnnotation":     func(annots []ast.Annotation, name string) bool { return hasAnnotation(annots, name) },
		"annotationValue":   func(annots []ast.Annotation, name, param string) string { return annotationValue(annots, name, param) },
		"extensibility":     extensibility,
		"isOptional":        isOptional,
		"isKey":             isKey,
		"cdrAlignment":      cdrAlignment,
		"cdrWriteFunc":      cdrWriteFunc,
		"cdrReadFunc":       cdrReadFunc,
		"isPrimitive":       isPrimitive,
		"isString":          isString,
		"isFixedPrimitive":  isFixedPrimitive,
		"isSequence":        isSequence,
		"isArray":           isArray,
		"sequenceElemType":  sequenceElemType,
		"arrayElemType":     arrayElemType,
		"arraySize":         arraySize,
		"seqElemTypeRef":    seqElemTypeRef,
		"arrElemTypeRef":    arrElemTypeRef,
		"enumValueInt":      enumValueInt,
		"hasExplicitValue":  hasExplicitValue,
		"fieldMemberID":     fieldMemberID,
		"cdrSerializedSize": cdrSerializedSize,
		"lower":             strings.ToLower,
		"upper":             strings.ToUpper,
		"add":               func(a, b int) int { return a + b },
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.go.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	g := &Generator{
		config:    cfg,
		templates: tmpl,
	}
	g.packagePrefix = g.resolvePackagePrefix()
	return g, nil
}

// collectUnits flattens an AST file's definitions into per-package genUnits.
func collectUnits(file *ast.File) map[string]*genUnit {
	units := make(map[string]*genUnit)

	var collect func(defs []ast.Definition, modPath []string)
	collect = func(defs []ast.Definition, modPath []string) {
		pkg := "main"
		relPath := "."
		if len(modPath) > 0 {
			pkg = strings.ToLower(modPath[len(modPath)-1])
			relPath = strings.ToLower(strings.Join(modPath, "/"))
		}

		for _, def := range defs {
			switch d := def.(type) {
			case *ast.Module:
				collect(d.Definitions, append(modPath[:len(modPath):len(modPath)], d.Name))
			case *ast.Struct:
				u := getOrCreateUnit(units, relPath, pkg, modPath)
				u.Structs = append(u.Structs, d)
			case *ast.Enum:
				u := getOrCreateUnit(units, relPath, pkg, modPath)
				u.Enums = append(u.Enums, d)
			case *ast.Typedef:
				u := getOrCreateUnit(units, relPath, pkg, modPath)
				u.Typedefs = append(u.Typedefs, d)
			case *ast.Const:
				u := getOrCreateUnit(units, relPath, pkg, modPath)
				u.Consts = append(u.Consts, d)
			}
		}
	}
	collect(file.Definitions, nil)
	return units
}

// Generate processes an AST file and writes Go source files.
func (g *Generator) Generate(file *ast.File) error {
	for relPath, unit := range collectUnits(file) {
		if err := g.generateUnit(relPath, unit); err != nil {
			return err
		}
	}
	return nil
}

// GenerateToBuffer processes an AST file and returns the generated Go source
// as a byte slice. This is useful for testing without writing to disk.
func (g *Generator) GenerateToBuffer(file *ast.File) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for relPath, unit := range collectUnits(file) {
		data, err := g.renderUnit(unit)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", relPath, err)
		}
		result[relPath] = data
	}
	return result, nil
}

func getOrCreateUnit(units map[string]*genUnit, relPath, pkg string, modPath []string) *genUnit {
	u, ok := units[relPath]
	if !ok {
		mp := make([]string, len(modPath))
		copy(mp, modPath)
		u = &genUnit{
			PackageName: pkg,
			PackagePath: relPath,
			ModulePath:  mp,
		}
		units[relPath] = u
	}
	return u
}

// generateUnit renders a genUnit to a Go source file and writes it to disk.
func (g *Generator) generateUnit(relPath string, unit *genUnit) error {
	data, err := g.renderUnit(unit)
	if err != nil {
		return err
	}

	outDir := filepath.Join(g.config.OutputDir, relPath)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", outDir, err)
	}

	outFile := filepath.Join(outDir, "types_gen.go")
	if err := os.WriteFile(outFile, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outFile, err)
	}
	return nil
}

// resolvePackagePrefix returns the Go import path prefix for generated packages.
// If Config.PackagePrefix is set, it is used directly. Otherwise, it is
// auto-detected from go.mod and the output directory.
func (g *Generator) resolvePackagePrefix() string {
	if g.config.PackagePrefix != "" {
		return g.config.PackagePrefix
	}

	absOut, err := filepath.Abs(g.config.OutputDir)
	if err != nil {
		return ""
	}

	// Walk up to find go.mod
	dir := absOut
	for {
		goModPath := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(goModPath)
		if err == nil {
			modPath := parseModulePath(data)
			if modPath != "" {
				rel, err := filepath.Rel(dir, absOut)
				if err == nil {
					return modPath + "/" + filepath.ToSlash(rel)
				}
			}
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// parseModulePath extracts the module path from go.mod content.
func parseModulePath(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module"))
		}
	}
	return ""
}

// preprocessUnit resolves cross-package NamedType references and collects imports.
func (g *Generator) preprocessUnit(unit *genUnit) {
	imports := make(map[string]string) // pkgAlias -> import path

	resolveNamed := func(nt *ast.NamedType) {
		parts := strings.Split(nt.Name, "::")
		if len(parts) <= 1 {
			return // simple name, no module prefix
		}

		typeName := parts[len(parts)-1]
		typeModPath := parts[:len(parts)-1]

		// Check if same module
		if sliceEqual(typeModPath, unit.ModulePath) {
			nt.Name = typeName
			return
		}

		// Different module - use last module segment as package qualifier
		pkgAlias := strings.ToLower(typeModPath[len(typeModPath)-1])
		nt.Name = pkgAlias + "::" + typeName

		// Compute import path
		if g.packagePrefix != "" {
			pkgPath := strings.ToLower(strings.Join(typeModPath, "/"))
			imports[pkgAlias] = g.packagePrefix + "/" + pkgPath
		}
	}

	// Walk all type references in structs
	for _, s := range unit.Structs {
		for i := range s.Fields {
			walkTypeRef(&s.Fields[i].Type, resolveNamed)
		}
	}
	// Walk typedefs
	for _, td := range unit.Typedefs {
		walkTypeRef(&td.Type, resolveNamed)
	}

	// Collect imports
	for _, path := range imports {
		unit.Imports = append(unit.Imports, pkgImport{Path: path})
	}
}

func walkTypeRef(ref *ast.TypeRef, fn func(*ast.NamedType)) {
	switch t := (*ref).(type) {
	case *ast.NamedType:
		fn(t)
	case *ast.SequenceType:
		walkTypeRef(&t.ElemType, fn)
	case *ast.ArrayType:
		walkTypeRef(&t.ElemType, fn)
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// renderUnit renders a genUnit to formatted Go source bytes.
func (g *Generator) renderUnit(unit *genUnit) ([]byte, error) {
	// Resolve cross-package type references before rendering.
	g.preprocessUnit(unit)

	var buf bytes.Buffer

	// Render file header
	if err := g.templates.ExecuteTemplate(&buf, "file.go.tmpl", unit); err != nil {
		return nil, fmt.Errorf("execute file template: %w", err)
	}

	// Render consts
	if len(unit.Consts) > 0 {
		if err := g.templates.ExecuteTemplate(&buf, "const.go.tmpl", unit); err != nil {
			return nil, fmt.Errorf("execute const template: %w", err)
		}
	}

	// Render typedefs
	if len(unit.Typedefs) > 0 {
		if err := g.templates.ExecuteTemplate(&buf, "typedef.go.tmpl", unit); err != nil {
			return nil, fmt.Errorf("execute typedef template: %w", err)
		}
	}

	// Render enums
	if len(unit.Enums) > 0 {
		if err := g.templates.ExecuteTemplate(&buf, "enum.go.tmpl", unit); err != nil {
			return nil, fmt.Errorf("execute enum template: %w", err)
		}
	}

	// Render structs
	if len(unit.Structs) > 0 {
		if err := g.templates.ExecuteTemplate(&buf, "struct.go.tmpl", unit); err != nil {
			return nil, fmt.Errorf("execute struct template: %w", err)
		}

		// Render marshal methods based on extensibility
		for _, s := range unit.Structs {
			ext := extensibility(s)
			data := struct {
				PackageName string
				Struct      *ast.Struct
			}{unit.PackageName, s}

			switch ext {
			case "FINAL":
				if err := g.templates.ExecuteTemplate(&buf, "marshal_final.go.tmpl", data); err != nil {
					return nil, fmt.Errorf("execute marshal_final template: %w", err)
				}
			case "APPENDABLE":
				if err := g.templates.ExecuteTemplate(&buf, "marshal_appendable.go.tmpl", data); err != nil {
					return nil, fmt.Errorf("execute marshal_appendable template: %w", err)
				}
			case "MUTABLE":
				if err := g.templates.ExecuteTemplate(&buf, "marshal_mutable.go.tmpl", data); err != nil {
					return nil, fmt.Errorf("execute marshal_mutable template: %w", err)
				}
			}
		}
	}

	// Format the generated source
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format source: %w\n\nraw source:\n%s", err, buf.String())
	}
	return formatted, nil
}
