# go-dds-idlgen

A code generator that reads OMG IDL 4.2 files and produces Go structs with
XCDR2-compliant CDR serializers. Designed for DDS (Data Distribution Service)
pub/sub data models.

## Features

- **Zero external dependencies** -- only the Go standard library
- **Hand-written recursive descent parser** for a practical IDL subset
- **XCDR2 CDR encoding** with proper alignment (max 4 bytes)
- Three extensibility kinds: **FINAL**, **APPENDABLE**, **MUTABLE**
- `@optional` fields mapped to Go pointer types
- `@key`, `@id`, `@extensibility` annotations
- `sequence<T>` to `[]T`, fixed-length arrays to `[N]T`
- `enum`, `typedef`, `const` support
- `#include` resolution with cycle detection
- Unknown constructs (`interface`, `union`, ...) are skipped with warnings

## Installation

```sh
go install github.com/shirou/go-dds-idlgen/cmd/go-dds-idlgen@latest
```

## Quick Start

Given an IDL file `sensor.idl`:

```idl
module sensor {

    @final
    struct SensorData {
        @key
        long sensor_id;
        double temperature;
        float humidity;
        string location;
        @optional
        boolean active;
    };

};
```

Run the generator:

```sh
go-dds-idlgen -o gen/ sensor.idl
```

This produces `gen/sensor/sensor.go`:

```go
package sensor

import "github.com/shirou/go-dds-idlgen/cdr"

type SensorData struct {
    SensorId    int32
    Temperature float64
    Humidity    float32
    Location    string
    Active      *bool
}

func (s *SensorData) MarshalCDR(enc *cdr.Encoder) error { /* ... */ }
func (s *SensorData) UnmarshalCDR(dec *cdr.Decoder) error { /* ... */ }
```

Use the generated code:

```go
package main

import (
    "fmt"

    "your/project/gen/sensor"
)

func main() {
    data := &sensor.SensorData{
        SensorId:    42,
        Temperature: 23.5,
        Humidity:    65.2,
        Location:    "Room 101",
    }

    // Serialize (encapsulation header is written automatically)
    data, _ := data.MarshalCDR()

    // Deserialize (byte order is read from the encapsulation header)
    var result sensor.SensorData
    _ = result.UnmarshalCDR(data)

    fmt.Printf("%+v\n", result)
}
```

## CLI Usage

```
go-dds-idlgen [flags] [file.idl ...]
```

| Flag | Description |
|------|-------------|
| `-o`, `-out` | Output directory (default `.`) |
| `-I`, `-include` | Include search path (repeatable) |
| `-package-prefix` | Go package prefix for generated code |
| `-module-path` | Go module path |
| `-verbose` | Enable verbose logging |

If no IDL files are given, all `.idl` files under the include paths are
processed recursively.

## Supported IDL Subset

### Types

| IDL Type | Go Type |
|----------|---------|
| `boolean` | `bool` |
| `octet`, `uint8` | `uint8` |
| `char`, `int8` | `int8` |
| `short`, `int16` | `int16` |
| `unsigned short`, `uint16` | `uint16` |
| `long`, `int32` | `int32` |
| `unsigned long`, `uint32` | `uint32` |
| `long long`, `int64` | `int64` |
| `unsigned long long`, `uint64` | `uint64` |
| `float` | `float32` |
| `double` | `float64` |
| `string`, `string<N>` | `string` |
| `sequence<T>` | `[]T` |
| `T field[N]` | `[N]T` |

### Declarations

- `module` -- mapped to Go packages
- `struct` -- with optional inheritance via `:`
- `enum` -- mapped to `int32` with named constants
- `typedef` -- mapped to Go type aliases
- `const` -- mapped to Go constants

### Annotations

| Annotation | Effect |
|------------|--------|
| `@final` | XCDR2 FINAL encoding (no header) |
| `@appendable` | XCDR2 APPENDABLE encoding (DHEADER) |
| `@mutable` | XCDR2 MUTABLE encoding (PL_CDR2 with EMHEADER per field) |
| `@key` | Marks DDS key fields (metadata only) |
| `@optional` | Maps field to Go pointer type; presence-flagged in FINAL/APPENDABLE |
| `@id(N)` | Explicit member ID for MUTABLE encoding |

### Skipped Constructs

`interface`, `union`, `valuetype`, `bitset`, `bitmask` declarations are
skipped with a warning. They may be supported in a future release.

## CDR Runtime

The `cdr/` package is a public Go package that generated code imports. You can
also use it directly:

```go
import "github.com/shirou/go-dds-idlgen/cdr"

enc := cdr.NewEncoder(cdr.CDR2_LE)
enc.WriteUint32(42)
enc.WriteString("hello")

dec, _ := cdr.NewDecoder(enc.Bytes())
v, _ := dec.ReadUint32()   // 42
s, _ := dec.ReadString()   // "hello"
```

## Project Structure

```
cmd/go-dds-idlgen/    CLI entry point
cdr/                   CDR encoder/decoder (public, imported by generated code)
internal/
  ast/                 AST node definitions
  parser/              Lexer and recursive descent parser
  resolver/            #include resolution and type scope resolution
  generator/           Template-based Go code generation
    templates/         Go text/templates for code output
testdata/              IDL samples and golden/round-trip tests
```

## Development

```sh
# Run all tests
go test ./... ./testdata/

# Build the CLI
go build ./cmd/go-dds-idlgen/

# Generate from a sample IDL
go run ./cmd/go-dds-idlgen/ -o /tmp/out -verbose testdata/basic_struct/input.idl

# Regenerate cross-language interop test fixtures (requires Docker)
cd testdata/interop
docker build -t go-dds-idlgen-interop .
docker run --rm -v "$PWD:/out" go-dds-idlgen-interop
```

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
