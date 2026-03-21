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
	ModulePath  []string    // flattened IDL module path, e.g., ["Org", "Common"]
	FileName    string      // output file name without extension (e.g., "GlobalHoveringHoverType")
	Imports     []pkgImport // cross-package imports needed
	Structs     []*ast.Struct
	Enums       []*ast.Enum
	Unions      []*ast.Union
	Typedefs    []*ast.Typedef
	Consts      []*ast.Const
	Skipped     []*ast.SkippedDecl // skipped declarations to emit as placeholders
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
	// emitted tracks type/const names already generated per package path
	// to avoid redeclaration when sibling sub-modules define the same names.
	emitted map[string]map[string]bool // relPath -> set of emitted names
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
		"enumComputedValue": enumComputedValue,
		"hasExplicitValue":  hasExplicitValue,
		"fieldMemberID":     fieldMemberID,
		"cdrSerializedSize": cdrSerializedSize,
		"unionDiscriminatorIsEnum":          unionDiscriminatorIsEnum,
		"unionDiscriminatorEnum":           unionDiscriminatorEnum,
		"unionDiscriminatorGoType":         unionDiscriminatorGoType,
		"unionCaseGoConstant":              unionCaseGoConstant,
		"unionCaseWrapperName":             unionCaseWrapperName,
		"unionInterfaceName":               unionInterfaceName,
		"unionDiscriminatorWriteFunc":      unionDiscriminatorWriteFunc,
		"unionDiscriminatorReadFunc":       unionDiscriminatorReadFunc,
		"unionDiscriminatorCastToWire":     unionDiscriminatorCastToWire,
		"unionSwitchExpr":                  unionSwitchExpr,
		"unionDefaultDiscriminatorGoType":  unionDefaultDiscriminatorGoType,
		"unionHasDefaultCase":              unionHasDefaultCase,
		"isNestedStruct":    isNestedStruct,
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
		emitted:   make(map[string]map[string]bool),
	}
	g.packagePrefix = g.resolvePackagePrefix()
	return g, nil
}

// flattenModPath drops the last segment of module paths with depth >= 3,
// merging leaf-level categorization modules into their parent package.
func flattenModPath(modPath []string) []string {
	if len(modPath) >= 3 {
		return modPath[:len(modPath)-1]
	}
	return modPath
}

// collectUnits flattens an AST file's definitions into per-package genUnits.
func collectUnits(file *ast.File) map[string]*genUnit {
	units := make(map[string]*genUnit)

	// Derive output file name from IDL source file name.
	fileName := strings.TrimSuffix(filepath.Base(file.Name), filepath.Ext(file.Name))

	var collect func(defs []ast.Definition, modPath []string)
	collect = func(defs []ast.Definition, modPath []string) {
		flatPath := flattenModPath(modPath)
		pkg := "main"
		relPath := "."
		if len(flatPath) > 0 {
			pkg = strings.ToLower(flatPath[len(flatPath)-1])
			relPath = strings.ToLower(strings.Join(flatPath, "/"))
		}

		for _, def := range defs {
			switch d := def.(type) {
			case *ast.Module:
				collect(d.Definitions, append(modPath[:len(modPath):len(modPath)], d.Name))
			case *ast.Struct:
				u := getOrCreateUnit(units, relPath, pkg, flatPath, fileName)
				u.Structs = append(u.Structs, cloneStruct(d))
			case *ast.Enum:
				u := getOrCreateUnit(units, relPath, pkg, flatPath, fileName)
				u.Enums = append(u.Enums, d)
			case *ast.Union:
				u := getOrCreateUnit(units, relPath, pkg, flatPath, fileName)
				u.Unions = append(u.Unions, cloneUnion(d))
			case *ast.Typedef:
				u := getOrCreateUnit(units, relPath, pkg, flatPath, fileName)
				u.Typedefs = append(u.Typedefs, cloneTypedef(d))
			case *ast.Const:
				u := getOrCreateUnit(units, relPath, pkg, flatPath, fileName)
				u.Consts = append(u.Consts, d)
			case *ast.SkippedDecl:
				if d.Name != "" {
					u := getOrCreateUnit(units, relPath, pkg, flatPath, fileName)
					u.Skipped = append(u.Skipped, d)
				}
			}
		}
	}
	// Only process the file's own definitions, not included ones.
	// Included definitions are in Definitions[:OwnStart] and are only
	// needed for type resolution, not code generation.
	ownDefs := file.Definitions[file.OwnStart:]
	collect(ownDefs, nil)
	return units
}

// Generate processes an AST file and writes Go source files.
func (g *Generator) Generate(file *ast.File) error {
	for relPath, unit := range collectUnits(file) {
		g.dedup(relPath, unit)
		if err := g.generateUnit(relPath, unit); err != nil {
			return err
		}
	}
	return nil
}

// dedup removes types/consts from a unit that have already been emitted
// in the same package. It handles both in-process dedup (multiple Generate
// calls) and cross-process dedup (scanning existing files on disk).
func (g *Generator) dedup(relPath string, unit *genUnit) {
	seen := g.emitted[relPath]
	if seen == nil {
		seen = make(map[string]bool)
		// Scan existing generated files in the output directory for declared names.
		g.scanExistingDecls(relPath, seen)
		g.emitted[relPath] = seen
	}

	unit.Structs = deduplicateSlice(unit.Structs, seen, func(s *ast.Struct) string { return pascalCase(s.Name) })
	unit.Enums = deduplicateSlice(unit.Enums, seen, func(e *ast.Enum) string { return pascalCase(e.Name) })
	unit.Unions = deduplicateSlice(unit.Unions, seen, func(u *ast.Union) string { return pascalCase(u.Name) })
	unit.Typedefs = deduplicateSlice(unit.Typedefs, seen, func(t *ast.Typedef) string { return pascalCase(t.Name) })
	unit.Consts = deduplicateSlice(unit.Consts, seen, func(c *ast.Const) string { return pascalCase(c.Name) })
	unit.Skipped = deduplicateSlice(unit.Skipped, seen, func(s *ast.SkippedDecl) string { return pascalCase(s.Name) })
}

// scanExistingDecls reads existing generated .go files in the output directory
// and extracts declared type/const names into the seen set. This enables dedup
// across separate process invocations.
func (g *Generator) scanExistingDecls(relPath string, seen map[string]bool) {
	dir := filepath.Join(g.config.OutputDir, relPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // directory doesn't exist yet
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".go" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			line = strings.TrimSpace(line)
			// Match "type Name ..." or "const Name ..."
			for _, prefix := range []string{"type ", "const "} {
				if !strings.HasPrefix(line, prefix) {
					continue
				}
				rest := line[len(prefix):]
				// Extract the identifier (first word)
				if idx := strings.IndexAny(rest, " =\t"); idx > 0 {
					seen[rest[:idx]] = true
				}
			}
		}
	}
}

func deduplicateSlice[T any](items []T, seen map[string]bool, name func(T) string) []T {
	result := make([]T, 0, len(items))
	for _, item := range items {
		n := name(item)
		if seen[n] {
			continue
		}
		seen[n] = true
		result = append(result, item)
	}
	return result
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

func getOrCreateUnit(units map[string]*genUnit, relPath, pkg string, modPath []string, fileName string) *genUnit {
	u, ok := units[relPath]
	if !ok {
		mp := make([]string, len(modPath))
		copy(mp, modPath)
		u = &genUnit{
			PackageName: pkg,
			PackagePath: relPath,
			ModulePath:  mp,
			FileName:    fileName,
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

	outFileName := "types_gen.go"
	if unit.FileName != "" {
		outFileName = unit.FileName + ".go"
	}
	outFile := filepath.Join(outDir, outFileName)
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
	for line := range strings.SplitSeq(string(data), "\n") {
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

	resolveNamed := func(ref *ast.TypeRef) {
		nt := (*ref).(*ast.NamedType)
		parts := strings.Split(nt.Name, "::")
		if len(parts) <= 1 {
			return // simple name, no module prefix
		}

		typeName := parts[len(parts)-1]
		typeModPath := parts[:len(parts)-1]

		// Flatten the referenced type's module path to match package grouping.
		flatTypeModPath := flattenModPath(typeModPath)

		// Check if same package after flattening.
		// Create a new NamedType to avoid mutating shared/cached AST nodes.
		if sliceEqual(flatTypeModPath, unit.ModulePath) {
			*ref = &ast.NamedType{Name: typeName, Resolved: nt.Resolved}
			return
		}

		// Different package - use last segment of flattened path as package qualifier
		pkgAlias := strings.ToLower(flatTypeModPath[len(flatTypeModPath)-1])
		*ref = &ast.NamedType{Name: pkgAlias + "::" + typeName, Resolved: nt.Resolved}

		// Compute import path
		if g.packagePrefix != "" {
			pkgPath := strings.ToLower(strings.Join(flatTypeModPath, "/"))
			imports[pkgAlias] = g.packagePrefix + "/" + pkgPath
		}
	}

	// Walk all type references in structs
	for _, s := range unit.Structs {
		for i := range s.Fields {
			walkTypeRef(&s.Fields[i].Type, resolveNamed)
		}
	}
	// Walk unions
	for _, u := range unit.Unions {
		walkTypeRef(&u.Discriminator, resolveNamed)
		for i := range u.Cases {
			walkTypeRef(&u.Cases[i].Type, resolveNamed)
		}
		if u.DefaultCase != nil {
			walkTypeRef(&u.DefaultCase.Type, resolveNamed)
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

func walkTypeRef(ref *ast.TypeRef, fn func(*ast.TypeRef)) {
	switch t := (*ref).(type) {
	case *ast.NamedType:
		fn(ref)
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

	// Render unions
	if len(unit.Unions) > 0 {
		if err := g.templates.ExecuteTemplate(&buf, "union.go.tmpl", unit); err != nil {
			return nil, fmt.Errorf("execute union template: %w", err)
		}
	}

	// Render placeholder types for skipped declarations
	for _, sk := range unit.Skipped {
		name := pascalCase(sk.Name)
		fmt.Fprintf(&buf, "\n// %s is a placeholder for IDL %s (not fully supported).\ntype %s struct{}\n", name, sk.Kind, name)
		fmt.Fprintf(&buf, "\nfunc (s *%s) EncodeCDR(enc *cdr.Encoder) error { return nil }\n", name)
		fmt.Fprintf(&buf, "func (s *%s) DecodeCDR(dec *cdr.Decoder) error { return nil }\n", name)
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

// cloneStruct deep-clones a struct's fields so that preprocessUnit can
// replace TypeRef values without mutating the cached AST.
func cloneStruct(s *ast.Struct) *ast.Struct {
	c := *s
	c.Fields = make([]ast.Field, len(s.Fields))
	for i, f := range s.Fields {
		c.Fields[i] = f
		c.Fields[i].Type = cloneTypeRef(f.Type)
	}
	return &c
}

// cloneTypedef clones a typedef so its Type can be replaced safely.
func cloneTypedef(t *ast.Typedef) *ast.Typedef {
	c := *t
	c.Type = cloneTypeRef(t.Type)
	return &c
}

// cloneUnion deep-clones a union so that preprocessUnit can replace TypeRef values safely.
func cloneUnion(u *ast.Union) *ast.Union {
	c := *u
	c.Discriminator = cloneTypeRef(u.Discriminator)
	c.Cases = make([]ast.UnionCase, len(u.Cases))
	for i, uc := range u.Cases {
		c.Cases[i] = uc
		c.Cases[i].Labels = append([]string(nil), uc.Labels...)
		c.Cases[i].Type = cloneTypeRef(uc.Type)
	}
	if u.DefaultCase != nil {
		dc := *u.DefaultCase
		dc.Type = cloneTypeRef(dc.Type)
		c.DefaultCase = &dc
	}
	return &c
}

// cloneTypeRef deep-clones a TypeRef, creating new SequenceType/ArrayType
// wrappers so that modifying inner NamedTypes doesn't affect the originals.
func cloneTypeRef(t ast.TypeRef) ast.TypeRef {
	switch v := t.(type) {
	case *ast.SequenceType:
		return &ast.SequenceType{ElemType: cloneTypeRef(v.ElemType), Bound: v.Bound}
	case *ast.ArrayType:
		return &ast.ArrayType{ElemType: cloneTypeRef(v.ElemType), Size: v.Size}
	default:
		return t
	}
}
