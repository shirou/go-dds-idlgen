package generator

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/shirou/go-dds-idlgen/cdr"
	"github.com/shirou/go-dds-idlgen/internal/ast"
	"github.com/shirou/go-dds-idlgen/internal/xtypes"
)

// resolveUnderlying follows NamedType → Typedef chains for primitive typedefs.
// Compound typedefs (array, sequence) are generated as defined types with their
// own MarshalCDR methods, so we stop at those.
func resolveUnderlying(t ast.TypeRef) ast.TypeRef {
	for {
		nt, ok := t.(*ast.NamedType)
		if !ok {
			return t
		}
		td, ok := nt.Resolved.(*ast.Typedef)
		if !ok {
			return t // resolved to struct/enum/skipped, not a typedef
		}
		// Stop at compound typedefs (array, sequence) - they are defined types
		// with their own MarshalCDR/UnmarshalCDR methods.
		switch td.Type.(type) {
		case *ast.ArrayType, *ast.SequenceType:
			return t
		}
		t = td.Type
	}
}

// goType converts an IDL TypeRef to a Go type string.
func goType(t ast.TypeRef) string {
	switch v := t.(type) {
	case *ast.BasicType:
		return goBasicType(v.Name)
	case *ast.StringType:
		return "string"
	case *ast.SequenceType:
		return "[]" + goType(v.ElemType)
	case *ast.ArrayType:
		return fmt.Sprintf("[%d]%s", v.Size, goType(v.ElemType))
	case *ast.NamedType:
		// Convert scoped name (e.g., "module::Type") to Go (e.g., "module.Type")
		parts := strings.Split(v.Name, "::")
		if len(parts) > 1 {
			return strings.Join(parts[:len(parts)-1], ".") + "." + pascalCase(parts[len(parts)-1])
		}
		return pascalCase(v.Name)
	}
	return "any"
}

// goBasicType maps IDL primitive type names to Go types.
func goBasicType(name string) string {
	switch name {
	case "boolean":
		return "bool"
	case "octet", "uint8":
		return "uint8"
	case "char", "int8":
		return "int8"
	case "int16", "short":
		return "int16"
	case "uint16":
		return "uint16"
	case "int32", "long":
		return "int32"
	case "uint32":
		return "uint32"
	case "int64":
		return "int64"
	case "uint64":
		return "uint64"
	case "float", "float32":
		return "float32"
	case "double", "float64":
		return "float64"
	}
	return name
}

// commonInitialisms is the set of Go common initialisms that should remain
// fully uppercase. Based on golang/lint's commonInitialisms.
var commonInitialisms = map[string]struct{}{
	"ACL": {}, "API": {}, "ASCII": {}, "CPU": {}, "CSS": {},
	"DNS": {}, "EOF": {}, "GUID": {}, "HTML": {}, "HTTP": {},
	"HTTPS": {}, "ID": {}, "IP": {}, "JSON": {}, "LHS": {},
	"QPS": {}, "RAM": {}, "RHS": {}, "RPC": {}, "SLA": {},
	"SMTP": {}, "SQL": {}, "SSH": {}, "TCP": {}, "TLS": {},
	"TTL": {}, "UDP": {}, "UI": {}, "UID": {}, "UUID": {},
	"URI": {}, "URL": {}, "UTF8": {}, "VM": {}, "XML": {},
	"XMPP": {}, "XSRF": {}, "XSS": {},
}

// pascalCase converts snake_case or camelCase to PascalCase.
// Common initialisms (ID, URL, UUID, etc.) are kept fully uppercase.
// Other all-uppercase words are title-cased (e.g., "MAX" -> "Max").
// Mixed-case words are left as-is except for capitalizing the first letter.
func pascalCase(s string) string {
	words := splitWords(s)
	for i, w := range words {
		if len(w) == 0 {
			continue
		}
		upper := strings.ToUpper(w)
		if _, ok := commonInitialisms[upper]; ok {
			// Known initialism: keep fully uppercase
			words[i] = upper
		} else if upper == w {
			// All-uppercase word but not a known initialism: title-case it
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		} else {
			// Mixed-case word: just capitalize first letter
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, "")
}

// camelCase converts to camelCase.
func camelCase(s string) string {
	p := pascalCase(s)
	if len(p) == 0 {
		return p
	}
	return strings.ToLower(p[:1]) + p[1:]
}

// snakeCase converts PascalCase/camelCase to snake_case.
// Handles consecutive uppercase letters correctly (e.g., "HTTPServer" -> "http_server").
func snakeCase(s string) string {
	words := splitWords(s)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "_")
}

// splitWords splits a string into words by underscores and camelCase boundaries.
func splitWords(s string) []string {
	// First split by underscores.
	parts := strings.Split(s, "_")
	var words []string
	for _, part := range parts {
		if part == "" {
			continue
		}
		// Split camelCase boundaries within each underscore-delimited segment.
		words = append(words, splitCamelCase(part)...)
	}
	return words
}

// splitCamelCase splits a string on camelCase boundaries.
// e.g., "sensorId" -> ["sensor", "Id"], "HTTPServer" -> ["HTTP", "Server"]
func splitCamelCase(s string) []string {
	if len(s) == 0 {
		return nil
	}
	runes := []rune(s)
	var words []string
	start := 0
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) {
			// If previous is lowercase, split before this char.
			if unicode.IsLower(runes[i-1]) {
				words = append(words, string(runes[start:i]))
				start = i
			} else if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				// Uppercase followed by lowercase, and previous is uppercase:
				// e.g., "HTTPServer" -> split before 'S'
				words = append(words, string(runes[start:i]))
				start = i
			}
		}
	}
	words = append(words, string(runes[start:]))
	return words
}

// hasAnnotation checks if a list of annotations contains one with the given name.
func hasAnnotation(annotations []ast.Annotation, name string) bool {
	for _, a := range annotations {
		if a.Name == name {
			return true
		}
	}
	return false
}

// annotationValue returns the value of a parameter in an annotation.
func annotationValue(annotations []ast.Annotation, name, param string) string {
	for _, a := range annotations {
		if a.Name == name {
			if param == "" {
				// Return the "value" param or first param
				if v, ok := a.Params["value"]; ok {
					return v
				}
				if v, ok := a.Params[""]; ok {
					return v
				}
			}
			if v, ok := a.Params[param]; ok {
				return v
			}
		}
	}
	return ""
}

// extensibility returns the extensibility kind for a struct (FINAL, APPENDABLE, MUTABLE).
// Default is FINAL if not annotated.
func extensibility(s *ast.Struct) string {
	v := annotationValue(s.Annotations, "extensibility", "")
	if v == "" {
		if hasAnnotation(s.Annotations, "final") {
			return "FINAL"
		}
		if hasAnnotation(s.Annotations, "appendable") {
			return "APPENDABLE"
		}
		if hasAnnotation(s.Annotations, "mutable") {
			return "MUTABLE"
		}
		return "FINAL"
	}
	return strings.ToUpper(v)
}

// isOptional checks if a field has @optional annotation.
func isOptional(f ast.Field) bool {
	return hasAnnotation(f.Annotations, "optional")
}

// isKey checks if a field has @key annotation.
func isKey(f ast.Field) bool {
	return hasAnnotation(f.Annotations, "key")
}

// structHasKeyField returns true if any field in the struct has @key annotation.
func structHasKeyField(s *ast.Struct) bool {
	for _, f := range s.Fields {
		if isKey(f) {
			return true
		}
	}
	return false
}

// goFieldType returns the Go type for a field, wrapping in pointer if optional.
func goFieldType(f ast.Field) string {
	t := goType(f.Type)
	if isOptional(f) {
		return "*" + t
	}
	return t
}

// cdrAlignment returns the CDR alignment for a type (XCDR2: max 4).
func cdrAlignment(t ast.TypeRef) int {
	t = resolveUnderlying(t)
	switch v := t.(type) {
	case *ast.BasicType:
		switch v.Name {
		case "boolean", "octet", "char", "int8", "uint8":
			return 1
		case "int16", "uint16", "short":
			return 2
		case "int32", "uint32", "long", "float", "float32":
			return 4
		case "int64", "uint64", "double", "float64":
			return 4 // XCDR2: max alignment is 4
		}
	case *ast.StringType:
		return 4 // strings start with uint32 length
	case *ast.SequenceType:
		return 4 // sequences start with uint32 length
	case *ast.ArrayType:
		return cdrAlignment(v.ElemType)
	case *ast.NamedType:
		return 4 // conservative default for named types
	}
	return 1
}

// cdrWriteFunc returns the Encoder method name for a basic type.
func cdrWriteFunc(t ast.TypeRef) string {
	t = resolveUnderlying(t)
	switch v := t.(type) {
	case *ast.BasicType:
		switch v.Name {
		case "boolean":
			return "WriteBool"
		case "octet", "uint8":
			return "WriteUint8"
		case "char", "int8":
			return "WriteInt8"
		case "int16", "short":
			return "WriteInt16"
		case "uint16":
			return "WriteUint16"
		case "int32", "long":
			return "WriteInt32"
		case "uint32":
			return "WriteUint32"
		case "int64":
			return "WriteInt64"
		case "uint64":
			return "WriteUint64"
		case "float", "float32":
			return "WriteFloat32"
		case "double", "float64":
			return "WriteFloat64"
		}
	case *ast.StringType:
		return "WriteString"
	}
	return ""
}

// cdrReadFunc returns the Decoder method name for a basic type.
func cdrReadFunc(t ast.TypeRef) string {
	t = resolveUnderlying(t)
	switch v := t.(type) {
	case *ast.BasicType:
		switch v.Name {
		case "boolean":
			return "ReadBool"
		case "octet", "uint8":
			return "ReadUint8"
		case "char", "int8":
			return "ReadInt8"
		case "int16", "short":
			return "ReadInt16"
		case "uint16":
			return "ReadUint16"
		case "int32", "long":
			return "ReadInt32"
		case "uint32":
			return "ReadUint32"
		case "int64":
			return "ReadInt64"
		case "uint64":
			return "ReadUint64"
		case "float", "float32":
			return "ReadFloat32"
		case "double", "float64":
			return "ReadFloat64"
		}
	case *ast.StringType:
		return "ReadString"
	}
	return ""
}

// isPrimitive returns true if the type is a basic or string type (not compound).
// Follows typedef chains: a NamedType that resolves to a typedef of a primitive is primitive.
func isPrimitive(t ast.TypeRef) bool {
	t = resolveUnderlying(t)
	switch t.(type) {
	case *ast.BasicType, *ast.StringType:
		return true
	}
	return false
}

// isString returns true if the type is a StringType.
func isString(t ast.TypeRef) bool {
	_, ok := resolveUnderlying(t).(*ast.StringType)
	return ok
}

// isFixedPrimitive returns true if the type is a fixed-size primitive (not string).
func isFixedPrimitive(t ast.TypeRef) bool {
	_, ok := resolveUnderlying(t).(*ast.BasicType)
	return ok
}

// isSequence returns true if the type is a SequenceType.
func isSequence(t ast.TypeRef) bool {
	_, ok := resolveUnderlying(t).(*ast.SequenceType)
	return ok
}

// isArray returns true if the type is an ArrayType.
func isArray(t ast.TypeRef) bool {
	_, ok := resolveUnderlying(t).(*ast.ArrayType)
	return ok
}

// sequenceElemType returns the Go type string for the element type of a sequence.
func sequenceElemType(t ast.TypeRef) string {
	if seq, ok := resolveUnderlying(t).(*ast.SequenceType); ok {
		return goType(seq.ElemType)
	}
	return ""
}

// arrayElemType returns the Go type string for the element type of an array.
func arrayElemType(t ast.TypeRef) string {
	if arr, ok := resolveUnderlying(t).(*ast.ArrayType); ok {
		return goType(arr.ElemType)
	}
	return ""
}

// arraySize returns the size of an ArrayType.
func arraySize(t ast.TypeRef) int {
	if arr, ok := resolveUnderlying(t).(*ast.ArrayType); ok {
		return arr.Size
	}
	return 0
}

// seqElemTypeRef returns the element TypeRef of a sequence type.
func seqElemTypeRef(t ast.TypeRef) ast.TypeRef {
	if seq, ok := resolveUnderlying(t).(*ast.SequenceType); ok {
		return seq.ElemType
	}
	return nil
}

// arrElemTypeRef returns the element TypeRef of an array type.
func arrElemTypeRef(t ast.TypeRef) ast.TypeRef {
	if arr, ok := resolveUnderlying(t).(*ast.ArrayType); ok {
		return arr.ElemType
	}
	return nil
}

// enumValueInt returns the explicit value of an EnumValue, or -1 if unset.
func enumValueInt(v ast.EnumValue) int64 {
	if v.Value != nil {
		return *v.Value
	}
	return -1
}

// enumComputedValue returns the effective integer value for the enum value at
// the given index. Explicit values are used as-is; implicit values continue
// incrementing from the previous value. This avoids mixing iota with explicit
// values, which would produce incorrect constants.
func enumComputedValue(values []ast.EnumValue, index int) int64 {
	var next int64
	for i := 0; i <= index; i++ {
		if values[i].Value != nil {
			next = *values[i].Value
		}
		if i == index {
			return next
		}
		next++
	}
	return next
}

// hasExplicitValue reports whether an EnumValue has an explicit value.
func hasExplicitValue(v ast.EnumValue) bool {
	return v.Value != nil
}

// fieldMemberID returns the member ID for a field. Uses @id annotation if present,
// otherwise falls back to the field index.
func fieldMemberID(f ast.Field, index int) int {
	if f.ID != nil {
		return int(*f.ID)
	}
	return index
}

// unionDiscriminatorIsEnum returns true if the union's discriminator is an enum type.
func unionDiscriminatorIsEnum(u *ast.Union) bool {
	nt, ok := u.Discriminator.(*ast.NamedType)
	if !ok {
		return false
	}
	_, isEnum := nt.Resolved.(*ast.Enum)
	return isEnum
}

// unionDiscriminatorEnum returns the resolved Enum for the union's discriminator, or nil.
func unionDiscriminatorEnum(u *ast.Union) *ast.Enum {
	nt, ok := u.Discriminator.(*ast.NamedType)
	if !ok {
		return nil
	}
	e, _ := nt.Resolved.(*ast.Enum)
	return e
}

// unionDiscriminatorGoType returns the Go type string for the union's discriminator.
func unionDiscriminatorGoType(u *ast.Union) string {
	return goType(u.Discriminator)
}

// unionCaseGoConstant returns the Go constant expression for a union case label.
// For enum discriminators, it returns "EnumTypeLABEL" (matching enum template output).
// For integer discriminators, it returns the literal value.
//
// TODO: when the discriminator enum is in a different package, this needs to
// include the package qualifier (e.g., "common.FilterTypeEnumALL_D").
// Currently assumes the enum is in the same package as the union.
func unionCaseGoConstant(u *ast.Union, label string) string {
	e := unionDiscriminatorEnum(u)
	if e != nil {
		return pascalCase(e.Name) + label
	}
	return label
}

// unionCaseWrapperName returns the Go wrapper type name for a union case.
func unionCaseWrapperName(u *ast.Union, uc ast.UnionCase) string {
	return pascalCase(u.Name) + "_" + pascalCase(uc.Name)
}

// unionInterfaceName returns the sealed interface name for a union.
func unionInterfaceName(u *ast.Union) string {
	return "is" + pascalCase(u.Name) + "Value"
}

// unionDiscriminatorWriteFunc returns the CDR write method for the discriminator.
// Enum discriminators use uint32 per the CDR spec.
func unionDiscriminatorWriteFunc(u *ast.Union) string {
	if unionDiscriminatorIsEnum(u) {
		return "WriteUint32"
	}
	return cdrWriteFunc(u.Discriminator)
}

// unionDiscriminatorReadFunc returns the CDR read method for the discriminator.
// Enum discriminators use uint32 per the CDR spec.
func unionDiscriminatorReadFunc(u *ast.Union) string {
	if unionDiscriminatorIsEnum(u) {
		return "ReadUint32"
	}
	return cdrReadFunc(u.Discriminator)
}

// unionDiscriminatorCastToWire returns the Go cast expression to convert
// a discriminator constant to the CDR wire type.
func unionDiscriminatorCastToWire(u *ast.Union) string {
	if unionDiscriminatorIsEnum(u) {
		return "uint32"
	}
	return goType(u.Discriminator)
}

// unionSwitchExpr returns the Go switch expression for decoding a union discriminator.
// For enum: "EnumType(disc)", for integer: "disc".
func unionSwitchExpr(u *ast.Union) string {
	if unionDiscriminatorIsEnum(u) {
		return goType(u.Discriminator) + "(disc)"
	}
	return "disc"
}

// unionDefaultDiscriminatorGoType returns the Go type for the discriminator field
// stored in a default case wrapper. Enum uses uint32; integer uses the Go type.
func unionDefaultDiscriminatorGoType(u *ast.Union) string {
	if unionDiscriminatorIsEnum(u) {
		return "uint32"
	}
	return goType(u.Discriminator)
}

// unionHasDefaultCase returns true if the union has a default case.
func unionHasDefaultCase(u *ast.Union) bool {
	return u.DefaultCase != nil
}

// isNestedStruct returns true if the struct has @nested annotation (and not @nested(FALSE)).
// Nested types should not have top-level MarshalCDR/UnmarshalCDR methods.
func isNestedStruct(s *ast.Struct) bool {
	for _, a := range s.Annotations {
		if a.Name == "nested" {
			if v, ok := a.Params["value"]; ok && strings.EqualFold(v, "FALSE") {
				return false
			}
			return true
		}
	}
	return false
}

// structTypeInfoBytes computes the XTypes TypeInformation for a struct and
// returns it as a Go []byte literal string for use in templates.
func structTypeInfoBytes(ctx *xtypes.ComputeContext, s *ast.Struct, modulePath []string) string {
	data, err := ctx.BuildTypeInformation(s, modulePath)
	if err != nil {
		return fmt.Sprintf("nil /* typeinfo error: %v */", err)
	}
	return xtypes.FormatByteLiteral(data)
}

// unionTypeInfoBytes computes the XTypes TypeInformation for a union and
// returns it as a Go []byte literal string for use in templates.
func unionTypeInfoBytes(ctx *xtypes.ComputeContext, u *ast.Union, modulePath []string) string {
	data, err := ctx.BuildUnionTypeInformation(u, modulePath)
	if err != nil {
		return fmt.Sprintf("nil /* typeinfo error: %v */", err)
	}
	return xtypes.FormatByteLiteral(data)
}

// cdrSerializedSize returns the fixed serialized size for a type, or 0 if variable.
func cdrSerializedSize(t ast.TypeRef) int {
	return cdrSerializedSizeRec(t, nil)
}

// cdrSerializedSizeRec returns the fixed serialized size for a type, or 0 if variable.
// It recursively resolves NamedTypes (structs, enums, typedefs).
// visited prevents infinite recursion on circular struct references.
func cdrSerializedSizeRec(t ast.TypeRef, visited map[*ast.Struct]bool) int {
	t = resolveUnderlying(t)
	switch v := t.(type) {
	case *ast.BasicType:
		switch v.Name {
		case "boolean", "octet", "char", "int8", "uint8":
			return 1
		case "int16", "uint16", "short":
			return 2
		case "int32", "uint32", "long", "float", "float32":
			return 4
		case "int64", "uint64", "double", "float64":
			return 8
		}
	case *ast.ArrayType:
		elemSize := cdrSerializedSizeRec(v.ElemType, visited)
		if elemSize > 0 {
			return elemSize * v.Size
		}
	case *ast.NamedType:
		if v.Resolved == nil {
			return 0
		}
		switch r := v.Resolved.(type) {
		case *ast.Enum:
			return 4 // CDR enums are uint32
		case *ast.Struct:
			if visited == nil {
				visited = make(map[*ast.Struct]bool)
			}
			if visited[r] {
				return 0 // circular reference
			}
			visited[r] = true
			return structFixedSize(r, visited)
		case *ast.Typedef:
			// resolveUnderlying stops at array/sequence typedefs (they have
			// their own EncodeCDR methods), but we still need their size.
			return cdrSerializedSizeRec(r.Type, visited)
		}
	}
	return 0 // variable size (string, sequence, etc.)
}

// structFixedSize returns the total serialized size of a FINAL struct
// if all fields are fixed-size. Returns 0 if any field is variable-size
// or if the struct has optional fields.
func structFixedSize(s *ast.Struct, visited map[*ast.Struct]bool) int {
	// Inherited fields come first but we can't resolve them here without
	// the full AST context. If the struct inherits, treat as variable.
	if s.Inherits != "" {
		return 0
	}
	pos := 0
	for _, f := range s.Fields {
		if isOptional(f) {
			return 0
		}
		align := cdrAlignment(f.Type)
		rem := pos % align
		if rem != 0 {
			pos += align - rem
		}
		size := cdrSerializedSizeRec(f.Type, visited)
		if size == 0 {
			return 0
		}
		pos += size
	}
	return pos
}

// keyFieldInfo holds code generation info for a single @key field.
type keyFieldInfo struct {
	FieldName    string
	StaticOffset int    // >= 0: statically computed, -1: needs runtime scan
	Size         int    // fixed serialized size (0 = variable)
	TypeHint     string // "KeyOpaque", "KeyInt32", etc.
	MemberID     int    // for MUTABLE extensibility
	FieldIndex   int    // declaration order index (including inherited fields)
}

// computeKeyFields computes offset information for all @key fields in a struct.
// If the struct inherits from a base type, inherited fields are included.
func computeKeyFields(s *ast.Struct) []keyFieldInfo {
	fields, resolved := allFields(s)
	ext := extensibility(s)

	var result []keyFieldInfo
	for i, f := range fields {
		if !isKey(f) {
			continue
		}

		kfi := keyFieldInfo{
			FieldName:  f.Name,
			Size:       cdrSerializedSizeRec(f.Type, nil),
			TypeHint:   keyTypeHint(f.Type),
			MemberID:   fieldMemberID(f, i),
			FieldIndex: i,
		}

		if ext == "MUTABLE" || !resolved {
			// MUTABLE: field order is not fixed, always runtime
			// Unresolved base: can't compute static offsets
			kfi.StaticOffset = -1
		} else if canStaticOffset(fields, i) {
			offset := staticOffset(fields, i)
			if ext == "APPENDABLE" {
				offset += 4 // DHEADER
			}
			kfi.StaticOffset = offset
		} else {
			kfi.StaticOffset = -1
		}

		result = append(result, kfi)
	}
	return result
}

// allFields returns all fields in declaration order (base → derived).
// The second return value is false if the base type could not be resolved.
//
// Limitation: ast.Struct.Inherits is a string name, and the generator does
// not currently have access to the resolver's type map. Therefore, when a
// struct inherits from a base type, this function always returns
// resolved=false, forcing runtime key extraction. To support static offsets
// for inherited structs, the resolver would need to populate a pointer to
// the base *ast.Struct (similar to NamedType.Resolved).
func allFields(s *ast.Struct) ([]ast.Field, bool) {
	if s.Inherits == "" {
		return s.Fields, true
	}
	return s.Fields, false
}

// canStaticOffset reports whether a static byte offset can be computed for
// the field at index upTo. All preceding fields must be non-optional and
// have a fixed serialized size.
func canStaticOffset(fields []ast.Field, upTo int) bool {
	for i := 0; i < upTo; i++ {
		if isOptional(fields[i]) {
			return false
		}
		if cdrSerializedSizeRec(fields[i].Type, nil) == 0 {
			return false
		}
	}
	return true
}

// staticOffset computes the cumulative byte offset to the field at index upTo,
// applying XCDR2 alignment rules (max 4 bytes).
func staticOffset(fields []ast.Field, upTo int) int {
	pos := 0
	for i := 0; i < upTo; i++ {
		align := cdrAlignment(fields[i].Type)
		rem := pos % align
		if rem != 0 {
			pos += align - rem
		}
		pos += cdrSerializedSizeRec(fields[i].Type, nil)
	}
	// Align for the target field itself
	if upTo < len(fields) {
		align := cdrAlignment(fields[upTo].Type)
		rem := pos % align
		if rem != 0 {
			pos += align - rem
		}
	}
	return pos
}

// keyTypeHint returns the KeyTypeHint constant name for a type.
func keyTypeHint(t ast.TypeRef) string {
	t = resolveUnderlying(t)
	switch v := t.(type) {
	case *ast.BasicType:
		switch v.Name {
		case "int32", "long", "uint32":
			return "KeyInt32"
		case "int64", "uint64":
			return "KeyInt64"
		}
	case *ast.StringType:
		return "KeyString"
	case *ast.ArrayType:
		if elem, ok := resolveUnderlying(v.ElemType).(*ast.BasicType); ok {
			if (elem.Name == "octet" || elem.Name == "uint8") && v.Size == 16 {
				return "KeyUUID"
			}
		}
	}
	return "KeyOpaque"
}

// needsRuntimeKeyExtract reports whether any key field requires runtime scanning.
func needsRuntimeKeyExtract(keyFields []keyFieldInfo) bool {
	for _, kf := range keyFields {
		if kf.StaticOffset < 0 {
			return true
		}
	}
	return false
}

// keyExtractFieldInfo holds per-field info for the key_extract template.
type keyExtractFieldInfo struct {
	Name        string
	IsKey       bool
	IsOptional  bool
	IsString    bool
	IsSequence  bool
	IsCompound  bool // struct/union type needing DecodeCDR
	FixedSize   int  // 0 = variable
	Alignment   int
	GoType      string
	TypeHint    string
	SeqElemSize int // fixed element size for sequences, 0 = variable
}

// buildKeyExtractFields builds the enriched field list for the key_extract template.
func buildKeyExtractFields(s *ast.Struct) []keyExtractFieldInfo {
	fields, _ := allFields(s)
	result := make([]keyExtractFieldInfo, len(fields))
	for i, f := range fields {
		underlying := resolveUnderlying(f.Type)
		info := keyExtractFieldInfo{
			Name:       f.Name,
			IsKey:      isKey(f),
			IsOptional: isOptional(f),
			IsString:   isString(f.Type),
			IsSequence: isSequence(f.Type),
			FixedSize:  cdrSerializedSizeRec(f.Type, nil),
			Alignment:  cdrAlignment(f.Type),
			GoType:     goType(f.Type),
			TypeHint:   keyTypeHint(f.Type),
		}

		// Check if compound type (struct/union that has DecodeCDR)
		if nt, ok := underlying.(*ast.NamedType); ok && nt.Resolved != nil {
			switch nt.Resolved.(type) {
			case *ast.Struct, *ast.Union:
				info.IsCompound = true
			}
		}

		// Sequence element size
		if seq, ok := underlying.(*ast.SequenceType); ok {
			info.SeqElemSize = cdrSerializedSizeRec(seq.ElemType, nil)
		}

		result[i] = info
	}
	return result
}

// emFieldSize delegates to cdr.EMFieldSize. Kept in generator package for
// use in template FuncMap and tests.
func emFieldSize(lc uint8, nextInt uint32) uint32 {
	return cdr.EMFieldSize(lc, nextInt)
}
