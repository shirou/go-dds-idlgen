package resolver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveFile_Basic(t *testing.T) {
	dir := t.TempDir()

	// Create a simple IDL file
	mainIDL := `
struct Point {
	long x;
	long y;
};
`
	writeFile(t, dir, "main.idl", mainIDL)

	r := NewIncludeResolver(nil)
	f, err := r.ResolveFile(filepath.Join(dir, "main.idl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(f.Definitions))
	}
}

func TestResolveFile_WithInclude(t *testing.T) {
	dir := t.TempDir()

	// Create included file
	typesIDL := `
struct Color {
	uint8 r;
	uint8 g;
	uint8 b;
};
`
	writeFile(t, dir, "types.idl", typesIDL)

	// Create main file that includes types.idl
	mainIDL := `
#include "types.idl"

struct Pixel {
	long x;
	long y;
};
`
	writeFile(t, dir, "main.idl", mainIDL)

	r := NewIncludeResolver(nil)
	f, err := r.ResolveFile(filepath.Join(dir, "main.idl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have both Color (from include) and Pixel
	if len(f.Definitions) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(f.Definitions))
	}
}

func TestResolveFile_CircularInclude(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "a.idl", `#include "b.idl"
struct A { long x; };
`)
	writeFile(t, dir, "b.idl", `#include "a.idl"
struct B { long y; };
`)

	r := NewIncludeResolver(nil)
	_, err := r.ResolveFile(filepath.Join(dir, "a.idl"))
	if err == nil {
		t.Fatal("expected circular include error")
	}
	if !strings.Contains(err.Error(), "circular include") {
		t.Fatalf("expected circular include error, got: %v", err)
	}
}

func TestResolveFile_IncludePathSearch(t *testing.T) {
	dir := t.TempDir()
	libDir := filepath.Join(dir, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Put the included file in a separate directory
	writeFile(t, libDir, "types.idl", `
struct Color {
	uint8 r;
	uint8 g;
	uint8 b;
};
`)

	// Main file includes types.idl, which is not in the same directory
	writeFile(t, dir, "main.idl", `
#include "types.idl"

struct Pixel {
	long x;
};
`)

	// Without include paths, should fail
	r := NewIncludeResolver(nil)
	_, err := r.ResolveFile(filepath.Join(dir, "main.idl"))
	if err == nil {
		t.Fatal("expected error without include paths")
	}

	// With include paths, should succeed
	r = NewIncludeResolver([]string{libDir})
	f, err := r.ResolveFile(filepath.Join(dir, "main.idl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Definitions) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(f.Definitions))
	}
}

func TestResolveFile_NotFound(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "main.idl", `
#include "missing.idl"

struct A { long x; };
`)

	r := NewIncludeResolver(nil)
	_, err := r.ResolveFile(filepath.Join(dir, "main.idl"))
	if err == nil {
		t.Fatal("expected error for missing include")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestResolveFile_Caching(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "common.idl", `
struct Common { long x; };
`)
	writeFile(t, dir, "a.idl", `
#include "common.idl"
struct A { long y; };
`)

	r := NewIncludeResolver(nil)

	// Resolve common.idl first
	f1, err := r.ResolveFile(filepath.Join(dir, "common.idl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolve a.idl which includes common.idl - should use cache
	_, err = r.ResolveFile(filepath.Join(dir, "a.idl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolve common.idl again - should return cached value
	f2, err := r.ResolveFile(filepath.Join(dir, "common.idl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if f1 != f2 {
		t.Fatal("expected cached file to be the same pointer")
	}
}

func TestResolveFile_WithIncludeGuards(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "types.idl", `
#ifndef TYPES_IDL
#define TYPES_IDL

struct Color {
	uint8 r;
	uint8 g;
	uint8 b;
};

#endif
`)

	writeFile(t, dir, "main.idl", `
#ifndef MAIN_IDL
#define MAIN_IDL

#include "types.idl"

struct Pixel {
	long x;
	long y;
};

#endif
`)

	r := NewIncludeResolver(nil)
	f, err := r.ResolveFile(filepath.Join(dir, "main.idl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Definitions) != 2 {
		t.Fatalf("expected 2 definitions, got %d", len(f.Definitions))
	}
}

func TestIncludedFiles_ExcludesInputAndSystem(t *testing.T) {
	dir := t.TempDir()

	// User include
	writeFile(t, dir, "types.idl", `
struct Color {
	uint8 r;
	uint8 g;
	uint8 b;
};
`)
	// System include (simulated)
	writeFile(t, dir, "system.idl", `
struct SysType { long x; };
`)
	// Main file with both user and system includes
	writeFile(t, dir, "main.idl", `
#include "types.idl"
#include <system.idl>

struct Pixel {
	long x;
	long y;
};
`)

	r := NewIncludeResolver([]string{dir})
	mainPath := filepath.Join(dir, "main.idl")
	_, err := r.ResolveFile(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	absMain, _ := filepath.Abs(mainPath)
	inputPaths := map[string]bool{absMain: true}
	included := r.IncludedFiles(inputPaths)

	// Should only contain types.idl (not main.idl, not system.idl)
	if len(included) != 1 {
		t.Fatalf("expected 1 included file, got %d: %v", len(included), included)
	}
	if filepath.Base(included[0].Path) != "types.idl" {
		t.Fatalf("expected types.idl, got %s", included[0].Path)
	}
}

func TestIncludedFiles_Sorted(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "c.idl", `struct C { long x; };`)
	writeFile(t, dir, "a.idl", `struct A { long x; };`)
	writeFile(t, dir, "b.idl", `struct B { long x; };`)
	writeFile(t, dir, "main.idl", `
#include "c.idl"
#include "a.idl"
#include "b.idl"
struct Main { long x; };
`)

	r := NewIncludeResolver(nil)
	mainPath := filepath.Join(dir, "main.idl")
	_, err := r.ResolveFile(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	absMain, _ := filepath.Abs(mainPath)
	included := r.IncludedFiles(map[string]bool{absMain: true})

	if len(included) != 3 {
		t.Fatalf("expected 3 included files, got %d", len(included))
	}

	// Verify sorted order
	for i := 1; i < len(included); i++ {
		if included[i-1].Path >= included[i].Path {
			t.Fatalf("included files not sorted: %s >= %s", included[i-1].Path, included[i].Path)
		}
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	if err != nil {
		t.Fatal(err)
	}
}
