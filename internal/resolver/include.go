package resolver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shirou/go-dds-idlgen/internal/ast"
	"github.com/shirou/go-dds-idlgen/internal/parser"
)

// IncludeResolver resolves #include directives in IDL files.
type IncludeResolver struct {
	// IncludePaths is the list of directories to search for included files.
	IncludePaths []string
	// cache stores already-parsed files to avoid re-parsing.
	cache map[string]*ast.File
	// inProgress tracks files currently being parsed to detect cycles.
	inProgress map[string]bool
}

// NewIncludeResolver creates a new IncludeResolver with the given include paths.
func NewIncludeResolver(includePaths []string) *IncludeResolver {
	return &IncludeResolver{
		IncludePaths: includePaths,
		cache:        make(map[string]*ast.File),
		inProgress:   make(map[string]bool),
	}
}

// ResolveFile parses an IDL file and recursively resolves all #include directives.
// It returns the parsed AST with all included definitions merged.
func (r *IncludeResolver) ResolveFile(path string) (*ast.File, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path %s: %w", path, err)
	}

	// Check cache
	if f, ok := r.cache[absPath]; ok {
		return f, nil
	}

	// Check for circular includes
	if r.inProgress[absPath] {
		return nil, fmt.Errorf("circular include detected: %s", absPath)
	}
	r.inProgress[absPath] = true
	defer delete(r.inProgress, absPath)

	// Read and parse the file
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", absPath, err)
	}

	file, err := parser.Parse(filepath.Base(absPath), data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", absPath, err)
	}

	// Resolve includes: collect all included definitions first, then prepend once.
	baseDir := filepath.Dir(absPath)
	var includedDefs []ast.Definition
	for _, inc := range file.Includes {
		incPath, err := r.findInclude(inc, baseDir)
		if err != nil {
			return nil, fmt.Errorf("resolve include %q in %s: %w", inc, absPath, err)
		}

		incFile, err := r.ResolveFile(incPath)
		if err != nil {
			return nil, err
		}

		includedDefs = append(includedDefs, incFile.Definitions...)
	}
	if len(includedDefs) > 0 {
		file.Definitions = append(includedDefs, file.Definitions...)
	}

	r.cache[absPath] = file
	return file, nil
}

// findInclude searches for an include file in the include paths.
func (r *IncludeResolver) findInclude(name string, baseDir string) (string, error) {
	// First try relative to the current file's directory
	candidate := filepath.Join(baseDir, name)
	if _, err := os.Stat(candidate); err == nil {
		return filepath.Abs(candidate)
	}

	// Then try each include path
	for _, dir := range r.IncludePaths {
		candidate = filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Abs(candidate)
		}
	}

	return "", fmt.Errorf("include file %q not found", name)
}
