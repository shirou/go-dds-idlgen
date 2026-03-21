package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
	// systemPaths tracks absolute paths of system includes (angle-bracket).
	systemPaths map[string]bool
}

// NewIncludeResolver creates a new IncludeResolver with the given include paths.
func NewIncludeResolver(includePaths []string) *IncludeResolver {
	return &IncludeResolver{
		IncludePaths: includePaths,
		cache:        make(map[string]*ast.File),
		inProgress:   make(map[string]bool),
		systemPaths:  make(map[string]bool),
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
		incPath, err := r.findInclude(inc.Path, baseDir)
		if err != nil {
			return nil, fmt.Errorf("resolve include %q in %s: %w", inc.Path, absPath, err)
		}

		if inc.System {
			r.systemPaths[incPath] = true
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
	file.OwnStart = len(includedDefs)

	r.cache[absPath] = file
	return file, nil
}

// IncludedFiles returns the cached files that were resolved as non-system
// includes, excluding the given input paths. The result is sorted by path
// for deterministic processing order.
func (r *IncludeResolver) IncludedFiles(inputPaths map[string]bool) []IncludedFile {
	var files []IncludedFile
	for absPath, file := range r.cache {
		if inputPaths[absPath] {
			continue
		}
		if r.systemPaths[absPath] {
			continue
		}
		files = append(files, IncludedFile{Path: absPath, File: file})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files
}

// IncludedFile pairs an absolute file path with its parsed AST.
type IncludedFile struct {
	Path string
	File *ast.File
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
