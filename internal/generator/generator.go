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
	Structs     []*ast.Struct
	Enums       []*ast.Enum
	Typedefs    []*ast.Typedef
	Consts      []*ast.Const
}

// Generator generates Go code from IDL AST.
type Generator struct {
	config    Config
	templates *template.Template
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

	return &Generator{
		config:    cfg,
		templates: tmpl,
	}, nil
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
				u := getOrCreateUnit(units, relPath, pkg)
				u.Structs = append(u.Structs, d)
			case *ast.Enum:
				u := getOrCreateUnit(units, relPath, pkg)
				u.Enums = append(u.Enums, d)
			case *ast.Typedef:
				u := getOrCreateUnit(units, relPath, pkg)
				u.Typedefs = append(u.Typedefs, d)
			case *ast.Const:
				u := getOrCreateUnit(units, relPath, pkg)
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

func getOrCreateUnit(units map[string]*genUnit, relPath, pkg string) *genUnit {
	u, ok := units[relPath]
	if !ok {
		u = &genUnit{
			PackageName: pkg,
			PackagePath: relPath,
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

// renderUnit renders a genUnit to formatted Go source bytes.
func (g *Generator) renderUnit(unit *genUnit) ([]byte, error) {
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
