package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/shirou/go-dds-idlgen/internal/ast"
	"github.com/shirou/go-dds-idlgen/internal/generator"
	"github.com/shirou/go-dds-idlgen/internal/resolver"
)

func main() {
	// Define flags
	var (
		includePaths  stringSlice
		outDir        string
		packagePrefix string
		modulePath    string
		verbose       bool
	)

	flag.Var(&includePaths, "I", "include path (can be specified multiple times)")
	flag.Var(&includePaths, "include", "include path (can be specified multiple times)")
	flag.StringVar(&outDir, "o", ".", "output directory")
	flag.StringVar(&outDir, "out", ".", "output directory")
	flag.StringVar(&packagePrefix, "package-prefix", "", "Go package prefix for generated code")
	flag.StringVar(&modulePath, "module-path", "", "Go module path")
	flag.BoolVar(&verbose, "verbose", false, "enable verbose logging")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go-dds-idlgen [flags] [file.idl ...]\n\n")
		fmt.Fprintf(os.Stderr, "Generate Go structs and CDR serializers from OMG IDL files.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nIf no IDL files are specified, all .idl files in include paths are processed.\n")
	}

	flag.Parse()

	// Collect input files
	idlFiles := flag.Args()
	if len(idlFiles) == 0 {
		// Find all .idl files in include paths recursively.
		for _, dir := range includePaths {
			_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if !d.IsDir() && filepath.Ext(path) == ".idl" {
					idlFiles = append(idlFiles, path)
				}
				return nil
			})
		}
	}

	if len(idlFiles) == 0 {
		fmt.Fprintln(os.Stderr, "error: no IDL files specified or found")
		flag.Usage()
		os.Exit(1)
	}

	// Create resolver and generator
	incResolver := resolver.NewIncludeResolver(includePaths)
	gen, err := generator.New(generator.Config{
		PackagePrefix: packagePrefix,
		ModulePath:    modulePath,
		OutputDir:     outDir,
	})
	if err != nil {
		log.Fatalf("initialize generator: %v", err)
	}

	// Track which files have been processed (by absolute path)
	processed := make(map[string]bool)

	// Process each explicitly specified IDL file
	for _, idlFile := range idlFiles {
		absPath, err := filepath.Abs(idlFile)
		if err != nil {
			log.Fatalf("resolve path %s: %v", idlFile, err)
		}
		processed[absPath] = true

		if verbose {
			log.Printf("processing %s", idlFile)
		}

		file, err := incResolver.ResolveFile(idlFile)
		if err != nil {
			log.Fatalf("resolve %s: %v", idlFile, err)
		}

		if err := processFile(file, gen); err != nil {
			log.Fatalf("%s: %v", idlFile, err)
		}

		if verbose {
			log.Printf("generated code for %s", idlFile)
		}
	}

	// Also generate code for non-system included files
	for _, inc := range incResolver.IncludedFiles(processed) {
		if verbose {
			log.Printf("processing included file %s", inc.Path)
		}

		if err := processFile(inc.File, gen); err != nil {
			log.Fatalf("included %s: %v", inc.Path, err)
		}

		if verbose {
			log.Printf("generated code for included %s", inc.Path)
		}
	}

	if verbose {
		log.Printf("done, output written to %s", outDir)
	}
}

// processFile resolves types and generates code for an IDL file.
func processFile(file *ast.File, gen *generator.Generator) error {
	if err := resolver.ResolveTypes(file); err != nil {
		return fmt.Errorf("resolve types: %w", err)
	}
	if err := gen.Generate(file); err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	return nil
}

// stringSlice implements flag.Value for collecting multiple string flags.
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}
