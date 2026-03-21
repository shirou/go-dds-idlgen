package ast

import "fmt"

// File represents a parsed IDL file.
type File struct {
	Name        string       // source file name
	Includes    []string     // #include paths
	Definitions []Definition // top-level definitions (included + own)
	OwnStart    int          // index where the file's own definitions begin
}

// Definition is the interface for all top-level declarations.
type Definition interface {
	definitionNode()
}

// Module represents an IDL module (namespace).
type Module struct {
	Name        string
	Definitions []Definition
}

// Struct represents an IDL struct.
type Struct struct {
	Name        string
	Fields      []Field
	Annotations []Annotation
	Inherits    string // base type name, empty if none
}

// Field represents a struct field.
type Field struct {
	Name        string
	Type        TypeRef
	Annotations []Annotation
	ID          *uint32 // @id value if specified
}

// Enum represents an IDL enum.
type Enum struct {
	Name        string
	Values      []EnumValue
	Annotations []Annotation
}

// EnumValue represents a single enum constant.
type EnumValue struct {
	Name  string
	Value *int64 // explicit value if specified
}

// Typedef represents an IDL typedef.
type Typedef struct {
	Name string
	Type TypeRef
}

// Const represents an IDL const declaration.
type Const struct {
	Name  string
	Type  TypeRef
	Value string // string representation of the value
}

// Union represents an IDL discriminated union.
type Union struct {
	Name          string
	Discriminator TypeRef      // switch type: NamedType (enum) or BasicType (integer)
	Cases         []UnionCase
	DefaultCase   *UnionCase   // nil if no default
	Annotations   []Annotation
}

// UnionCase represents a single case in a union.
type UnionCase struct {
	Labels []string // case label(s): enum value names or integer literals
	Type   TypeRef
	Name   string // field name
}

// SkippedDecl represents a declaration that was skipped (interface, etc.).
type SkippedDecl struct {
	Kind    string // "interface", "valuetype", etc.
	Name    string
	Warning string
}

// Annotation represents an IDL annotation like @key, @optional, @extensibility.
type Annotation struct {
	Name   string
	Params map[string]string // annotation parameters
}

// All Definition implementations.
func (*Module) definitionNode()      {}
func (*Struct) definitionNode()      {}
func (*Enum) definitionNode()        {}
func (*Union) definitionNode()       {}
func (*Typedef) definitionNode()     {}
func (*Const) definitionNode()       {}
func (*SkippedDecl) definitionNode() {}

// TypeRef is the interface for type references.
type TypeRef interface {
	typeRefNode()
	String() string
}

// BasicType represents a primitive IDL type.
type BasicType struct {
	Name string // "boolean", "octet", "char", "int8", "uint8", "int16", "uint16",
	// "int32", "uint32", "int64", "uint64", "float", "double"
}

// StringType represents a string type with optional bound.
type StringType struct {
	Bound int // 0 means unbounded
}

// SequenceType represents sequence<T> with optional bound.
type SequenceType struct {
	ElemType TypeRef
	Bound    int // 0 means unbounded
}

// ArrayType represents a fixed-size array T[N].
type ArrayType struct {
	ElemType TypeRef
	Size     int
}

// NamedType represents a reference to a user-defined type.
type NamedType struct {
	Name     string     // may be scoped, e.g. "module::TypeName"
	Resolved Definition // filled in by resolver, nil initially
}

func (*BasicType) typeRefNode()    {}
func (*StringType) typeRefNode()   {}
func (*SequenceType) typeRefNode() {}
func (*ArrayType) typeRefNode()    {}
func (*NamedType) typeRefNode()    {}

func (t *BasicType) String() string { return t.Name }

func (t *StringType) String() string {
	if t.Bound > 0 {
		return fmt.Sprintf("string<%d>", t.Bound)
	}
	return "string"
}

func (t *SequenceType) String() string {
	if t.Bound > 0 {
		return fmt.Sprintf("sequence<%s, %d>", t.ElemType, t.Bound)
	}
	return fmt.Sprintf("sequence<%s>", t.ElemType)
}

func (t *ArrayType) String() string { return fmt.Sprintf("%s[%d]", t.ElemType, t.Size) }
func (t *NamedType) String() string { return t.Name }
