package xtypes

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/shirou/go-dds-idlgen/internal/ast"
)

// computeResult caches the computed TypeIdentifier and serialized size
// for a named type to avoid redundant computation.
type computeResult struct {
	MinimalTID  TypeIdentifier
	MinimalSize uint32
	CompleteTID TypeIdentifier
	CompleteSize uint32
}

// ComputeContext holds memoized results for TypeIdentifier computation.
type ComputeContext struct {
	cache map[string]computeResult // fully qualified name -> result
}

// NewComputeContext creates a new ComputeContext.
func NewComputeContext() *ComputeContext {
	return &ComputeContext{
		cache: make(map[string]computeResult),
	}
}

// ComputeNameHash computes the 4-byte NameHash from a name string.
// NameHash = MD5(name)[0:4].
func ComputeNameHash(name string) NameHash {
	h := md5.Sum([]byte(name))
	var nh NameHash
	copy(nh[:], h[:4])
	return nh
}

// HashMemberID computes a hash-based member ID from a name.
// hashMemberID = uint32_LE(MD5(name)[0:4]) & 0x0FFFFFFF
func HashMemberID(name string) uint32 {
	nh := ComputeNameHash(name)
	return binary.LittleEndian.Uint32(nh[:]) & 0x0FFFFFFF
}

// ComputeTypeIdentifier computes the minimal TypeIdentifier for an AST TypeRef.
func (ctx *ComputeContext) ComputeTypeIdentifier(t ast.TypeRef, scope []string) (*TypeIdentifier, error) {
	t = resolveUnderlying(t)

	switch v := t.(type) {
	case *ast.BasicType:
		return primitiveTypeIdentifier(v.Name), nil

	case *ast.StringType:
		return stringTypeIdentifier(v.Bound), nil

	case *ast.SequenceType:
		elemTID, err := ctx.ComputeTypeIdentifier(v.ElemType, scope)
		if err != nil {
			return nil, fmt.Errorf("sequence element: %w", err)
		}
		return sequenceTypeIdentifier(v.Bound, elemTID), nil

	case *ast.ArrayType:
		elemTID, err := ctx.ComputeTypeIdentifier(v.ElemType, scope)
		if err != nil {
			return nil, fmt.Errorf("array element: %w", err)
		}
		return arrayTypeIdentifier(v.Size, elemTID), nil

	case *ast.NamedType:
		return ctx.computeNamedTypeIdentifier(v, scope, false)

	default:
		return &TypeIdentifier{Discriminator: TK_NONE}, nil
	}
}

// ComputeCompleteTypeIdentifier computes the complete TypeIdentifier for an AST TypeRef.
func (ctx *ComputeContext) ComputeCompleteTypeIdentifier(t ast.TypeRef, scope []string) (*TypeIdentifier, error) {
	t = resolveUnderlying(t)

	switch v := t.(type) {
	case *ast.BasicType:
		return primitiveTypeIdentifier(v.Name), nil

	case *ast.StringType:
		return stringTypeIdentifier(v.Bound), nil

	case *ast.SequenceType:
		elemTID, err := ctx.ComputeCompleteTypeIdentifier(v.ElemType, scope)
		if err != nil {
			return nil, fmt.Errorf("sequence element: %w", err)
		}
		return sequenceTypeIdentifierComplete(v.Bound, elemTID), nil

	case *ast.ArrayType:
		elemTID, err := ctx.ComputeCompleteTypeIdentifier(v.ElemType, scope)
		if err != nil {
			return nil, fmt.Errorf("array element: %w", err)
		}
		return arrayTypeIdentifierComplete(v.Size, elemTID), nil

	case *ast.NamedType:
		return ctx.computeNamedTypeIdentifier(v, scope, true)

	default:
		return &TypeIdentifier{Discriminator: TK_NONE}, nil
	}
}

// computeNamedTypeIdentifier handles struct, union, enum references.
func (ctx *ComputeContext) computeNamedTypeIdentifier(nt *ast.NamedType, scope []string, complete bool) (*TypeIdentifier, error) {
	fqn := fullyQualifiedName(nt.Name, scope)

	if result, ok := ctx.cache[fqn]; ok {
		if complete {
			tid := result.CompleteTID
			return &tid, nil
		}
		tid := result.MinimalTID
		return &tid, nil
	}

	switch resolved := nt.Resolved.(type) {
	case *ast.Struct:
		return ctx.computeStructTypeIdentifier(resolved, fqn, scope, complete)
	case *ast.Union:
		return ctx.computeUnionTypeIdentifier(resolved, fqn, scope, complete)
	case *ast.Enum:
		return ctx.computeEnumTypeIdentifier(resolved, fqn, scope, complete)
	case *ast.Typedef:
		if complete {
			return ctx.ComputeCompleteTypeIdentifier(resolved.Type, scope)
		}
		return ctx.ComputeTypeIdentifier(resolved.Type, scope)
	default:
		return &TypeIdentifier{Discriminator: TK_NONE}, nil
	}
}

// computeStructTypeIdentifier builds both Minimal and Complete type objects,
// serializes them, and computes EquivalenceHashes.
func (ctx *ComputeContext) computeStructTypeIdentifier(s *ast.Struct, fqn string, scope []string, complete bool) (*TypeIdentifier, error) {
	flags := structFlags(s)

	// Build members (shared between minimal and complete)
	var minMembers []MinimalStructMember
	var compMembers []CompleteStructMember

	for i, f := range s.Fields {
		minTID, err := ctx.ComputeTypeIdentifier(f.Type, scope)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", f.Name, err)
		}
		compTID, err := ctx.ComputeCompleteTypeIdentifier(f.Type, scope)
		if err != nil {
			return nil, fmt.Errorf("field %s (complete): %w", f.Name, err)
		}

		memberID := uint32(i)
		if f.ID != nil {
			memberID = *f.ID
		}

		var memberFlags uint16 = MemberFlagTryConstruct1
		if hasAnnotation(f.Annotations, "optional") {
			memberFlags |= MemberFlagIsOptional
		}
		if hasAnnotation(f.Annotations, "key") {
			memberFlags |= MemberFlagIsKey
		}

		minMembers = append(minMembers, MinimalStructMember{
			Common: CommonStructMember{
				MemberID:    memberID,
				MemberFlags: memberFlags,
				TypeID:      *minTID,
			},
			Detail: MinimalMemberDetail{
				NameHash: ComputeNameHash(f.Name),
			},
		})

		compMembers = append(compMembers, CompleteStructMember{
			Common: CommonStructMember{
				MemberID:    memberID,
				MemberFlags: memberFlags,
				TypeID:      *compTID,
			},
			Detail: CompleteMemberDetail{
				Name: f.Name,
			},
		})
	}

	// Serialize and hash minimal
	minObj := &MinimalTypeObject{
		Kind: TOK_STRUCTURE,
		StructType: &MinimalStructType{
			StructFlags: flags,
			Header: MinimalStructHeader{
				BaseType: TypeIdentifier{Discriminator: TK_NONE},
			},
			Members: minMembers,
		},
	}
	minSerialized := SerializeTypeObject(minObj)
	minHash := md5.Sum(minSerialized)
	var minEqHash EquivalenceHash
	copy(minEqHash[:], minHash[:14])

	// Serialize and hash complete
	compObj := &CompleteTypeObject{
		Kind: TOK_STRUCTURE,
		StructType: &CompleteStructType{
			StructFlags: flags,
			Header: CompleteStructHeader{
				BaseType: TypeIdentifier{Discriminator: TK_NONE},
				Detail: CompleteTypeDetail{
					TypeName: fqn,
				},
			},
			Members: compMembers,
		},
	}
	compSerialized := SerializeCompleteTypeObject(compObj)
	compHash := md5.Sum(compSerialized)
	var compEqHash EquivalenceHash
	copy(compEqHash[:], compHash[:14])

	minTID := TypeIdentifier{Discriminator: EK_MINIMAL, Hash: minEqHash}
	compTID := TypeIdentifier{Discriminator: EK_COMPLETE, Hash: compEqHash}

	ctx.cache[fqn] = computeResult{
		MinimalTID:   minTID,
		MinimalSize:  uint32(len(minSerialized)),
		CompleteTID:  compTID,
		CompleteSize: uint32(len(compSerialized)),
	}

	if complete {
		return &compTID, nil
	}
	return &minTID, nil
}

// computeUnionTypeIdentifier builds both Minimal and Complete union type objects.
func (ctx *ComputeContext) computeUnionTypeIdentifier(u *ast.Union, fqn string, scope []string, complete bool) (*TypeIdentifier, error) {
	discMinTID, err := ctx.ComputeTypeIdentifier(u.Discriminator, scope)
	if err != nil {
		return nil, fmt.Errorf("union discriminator: %w", err)
	}
	discCompTID, err := ctx.ComputeCompleteTypeIdentifier(u.Discriminator, scope)
	if err != nil {
		return nil, fmt.Errorf("union discriminator (complete): %w", err)
	}

	flags := unionFlags(u)

	var minMembers []MinimalUnionMember
	var compMembers []CompleteUnionMember

	allCases := u.Cases
	if u.DefaultCase != nil {
		allCases = append(allCases, *u.DefaultCase)
	}

	for i, c := range allCases {
		caseMinTID, err := ctx.ComputeTypeIdentifier(c.Type, scope)
		if err != nil {
			return nil, fmt.Errorf("union case %s: %w", c.Name, err)
		}
		caseCompTID, err := ctx.ComputeCompleteTypeIdentifier(c.Type, scope)
		if err != nil {
			return nil, fmt.Errorf("union case %s (complete): %w", c.Name, err)
		}

		var labels []int32
		isDefault := u.DefaultCase != nil && i == len(u.Cases)
		if !isDefault {
			for _, lbl := range c.Labels {
				labels = append(labels, parseLabelValue(lbl))
			}
		}

		var memberFlags uint16 = MemberFlagTryConstruct1
		if isDefault {
			memberFlags |= MemberFlagIsDefault
		}

		common := CommonUnionMember{
			MemberID:    uint32(i),
			MemberFlags: memberFlags,
			TypeID:      *caseMinTID,
			LabelCount:  uint32(len(labels)),
			Labels:      labels,
		}

		minMembers = append(minMembers, MinimalUnionMember{
			Common: common,
			Detail: MinimalMemberDetail{
				NameHash: ComputeNameHash(c.Name),
			},
		})

		compCommon := CommonUnionMember{
			MemberID:    uint32(i),
			MemberFlags: memberFlags,
			TypeID:      *caseCompTID,
			LabelCount:  uint32(len(labels)),
			Labels:      labels,
		}
		compMembers = append(compMembers, CompleteUnionMember{
			Common: compCommon,
			Detail: CompleteMemberDetail{
				Name: c.Name,
			},
		})
	}

	// Minimal
	minObj := &MinimalTypeObject{
		Kind: TOK_UNION,
		UnionType: &MinimalUnionType{
			UnionFlags: flags,
			DiscCommon: CommonDiscriminatorMember{
				MemberFlags: MemberFlagTryConstruct1,
				TypeID:      *discMinTID,
			},
			Members: minMembers,
		},
	}
	minSerialized := SerializeTypeObject(minObj)
	minHash := md5.Sum(minSerialized)
	var minEqHash EquivalenceHash
	copy(minEqHash[:], minHash[:14])

	// Complete
	compObj := &CompleteTypeObject{
		Kind: TOK_UNION,
		UnionType: &CompleteUnionType{
			UnionFlags: flags,
			Header: CompleteUnionHeader{
				Detail: CompleteTypeDetail{TypeName: fqn},
			},
			DiscMember: CompleteDiscriminatorMember{
				Common: CommonDiscriminatorMember{
					MemberFlags: MemberFlagTryConstruct1,
					TypeID:      *discCompTID,
				},
			},
			Members: compMembers,
		},
	}
	compSerialized := SerializeCompleteTypeObject(compObj)
	compHash := md5.Sum(compSerialized)
	var compEqHash EquivalenceHash
	copy(compEqHash[:], compHash[:14])

	minTID := TypeIdentifier{Discriminator: EK_MINIMAL, Hash: minEqHash}
	compTID := TypeIdentifier{Discriminator: EK_COMPLETE, Hash: compEqHash}

	ctx.cache[fqn] = computeResult{
		MinimalTID:   minTID,
		MinimalSize:  uint32(len(minSerialized)),
		CompleteTID:  compTID,
		CompleteSize: uint32(len(compSerialized)),
	}

	if complete {
		return &compTID, nil
	}
	return &minTID, nil
}

// computeEnumTypeIdentifier builds both Minimal and Complete enum type objects.
func (ctx *ComputeContext) computeEnumTypeIdentifier(e *ast.Enum, fqn string, scope []string, complete bool) (*TypeIdentifier, error) {
	var minLiterals []MinimalEnumLiteral
	var compLiterals []CompleteEnumLiteral

	var nextVal uint32
	for i, v := range e.Values {
		if v.Value != nil {
			nextVal = uint32(*v.Value)
		}

		// First literal (index 0) gets IS_DEFAULT flag per Cyclone DDS behavior
		var litFlags uint16
		if i == 0 {
			litFlags = MemberFlagIsDefault
		}

		minLiterals = append(minLiterals, MinimalEnumLiteral{
			Common: CommonEnumLiteral{
				Value: nextVal,
				Flags: litFlags,
			},
			Detail: MinimalMemberDetail{
				NameHash: ComputeNameHash(v.Name),
			},
		})

		compLiterals = append(compLiterals, CompleteEnumLiteral{
			Common: CommonEnumLiteral{
				Value: nextVal,
				Flags: litFlags,
			},
			Detail: CompleteMemberDetail{
				Name: v.Name,
			},
		})

		nextVal++
	}

	// Minimal
	minObj := &MinimalTypeObject{
		Kind: TOK_ENUM,
		EnumType: &MinimalEnumType{
			EnumFlags: TypeFlagIsFinal,
			Header:    MinimalEnumHeader{BitBound: 32},
			Literals:  minLiterals,
		},
	}
	minSerialized := SerializeTypeObject(minObj)
	minHash := md5.Sum(minSerialized)
	var minEqHash EquivalenceHash
	copy(minEqHash[:], minHash[:14])

	// Complete
	compObj := &CompleteTypeObject{
		Kind: TOK_ENUM,
		EnumType: &CompleteEnumType{
			EnumFlags: TypeFlagIsFinal,
			Header: CompleteEnumHeader{
				BitBound: 32,
				Detail:   CompleteTypeDetail{TypeName: fqn},
			},
			Literals: compLiterals,
		},
	}
	compSerialized := SerializeCompleteTypeObject(compObj)
	compHash := md5.Sum(compSerialized)
	var compEqHash EquivalenceHash
	copy(compEqHash[:], compHash[:14])

	minTID := TypeIdentifier{Discriminator: EK_MINIMAL, Hash: minEqHash}
	compTID := TypeIdentifier{Discriminator: EK_COMPLETE, Hash: compEqHash}

	ctx.cache[fqn] = computeResult{
		MinimalTID:   minTID,
		MinimalSize:  uint32(len(minSerialized)),
		CompleteTID:  compTID,
		CompleteSize: uint32(len(compSerialized)),
	}

	if complete {
		return &compTID, nil
	}
	return &minTID, nil
}

// BuildTypeInformation computes the full TypeInformation for a struct type
// and returns the serialized bytes (no encapsulation header).
func (ctx *ComputeContext) BuildTypeInformation(s *ast.Struct, scope []string) ([]byte, error) {
	fqn := fullyQualifiedName(s.Name, scope)

	// Compute both minimal and complete
	_, err := ctx.computeStructTypeIdentifier(s, fqn, scope, false)
	if err != nil {
		return nil, err
	}

	result := ctx.cache[fqn]

	// Collect dependencies
	var minDeps, compDeps []TypeIdWithSize
	ctx.collectDependencies(s, scope, &minDeps, &compDeps, make(map[string]bool))

	ti := TypeInformation{
		Minimal: TypeIdWithSizeSeq{
			TypeIDWithSize: TypeIdWithSize{
				TypeID:                   result.MinimalTID,
				TypeObjectSerializedSize: result.MinimalSize,
			},
			DependentTypeIDs: minDeps,
		},
		Complete: TypeIdWithSizeSeq{
			TypeIDWithSize: TypeIdWithSize{
				TypeID:                   result.CompleteTID,
				TypeObjectSerializedSize: result.CompleteSize,
			},
			DependentTypeIDs: compDeps,
		},
	}

	return SerializeTypeInformation(&ti), nil
}

// BuildUnionTypeInformation computes the full TypeInformation for a union type.
func (ctx *ComputeContext) BuildUnionTypeInformation(u *ast.Union, scope []string) ([]byte, error) {
	fqn := fullyQualifiedName(u.Name, scope)

	_, err := ctx.computeUnionTypeIdentifier(u, fqn, scope, false)
	if err != nil {
		return nil, err
	}

	result := ctx.cache[fqn]

	var minDeps, compDeps []TypeIdWithSize
	ctx.collectUnionDependencies(u, scope, &minDeps, &compDeps, make(map[string]bool))

	ti := TypeInformation{
		Minimal: TypeIdWithSizeSeq{
			TypeIDWithSize: TypeIdWithSize{
				TypeID:                   result.MinimalTID,
				TypeObjectSerializedSize: result.MinimalSize,
			},
			DependentTypeIDs: minDeps,
		},
		Complete: TypeIdWithSizeSeq{
			TypeIDWithSize: TypeIdWithSize{
				TypeID:                   result.CompleteTID,
				TypeObjectSerializedSize: result.CompleteSize,
			},
			DependentTypeIDs: compDeps,
		},
	}

	return SerializeTypeInformation(&ti), nil
}

// collectDependencies gathers TypeIdWithSize for all non-primitive types
// referenced by a struct's fields, for both minimal and complete.
func (ctx *ComputeContext) collectDependencies(s *ast.Struct, scope []string, minDeps, compDeps *[]TypeIdWithSize, seen map[string]bool) {
	for _, f := range s.Fields {
		ctx.collectTypeDependencies(f.Type, scope, minDeps, compDeps, seen)
	}
}

// collectUnionDependencies gathers dependencies for a union's member types.
func (ctx *ComputeContext) collectUnionDependencies(u *ast.Union, scope []string, minDeps, compDeps *[]TypeIdWithSize, seen map[string]bool) {
	ctx.collectTypeDependencies(u.Discriminator, scope, minDeps, compDeps, seen)
	for _, c := range u.Cases {
		ctx.collectTypeDependencies(c.Type, scope, minDeps, compDeps, seen)
	}
	if u.DefaultCase != nil {
		ctx.collectTypeDependencies(u.DefaultCase.Type, scope, minDeps, compDeps, seen)
	}
}

// collectTypeDependencies recursively collects non-primitive type dependencies.
func (ctx *ComputeContext) collectTypeDependencies(t ast.TypeRef, scope []string, minDeps, compDeps *[]TypeIdWithSize, seen map[string]bool) {
	t = resolveUnderlying(t)

	switch v := t.(type) {
	case *ast.SequenceType:
		ctx.collectTypeDependencies(v.ElemType, scope, minDeps, compDeps, seen)
	case *ast.ArrayType:
		ctx.collectTypeDependencies(v.ElemType, scope, minDeps, compDeps, seen)
	case *ast.NamedType:
		fqn := fullyQualifiedName(v.Name, scope)
		if seen[fqn] {
			return
		}
		seen[fqn] = true

		if result, ok := ctx.cache[fqn]; ok {
			*minDeps = append(*minDeps, TypeIdWithSize{
				TypeID:                   result.MinimalTID,
				TypeObjectSerializedSize: result.MinimalSize,
			})
			*compDeps = append(*compDeps, TypeIdWithSize{
				TypeID:                   result.CompleteTID,
				TypeObjectSerializedSize: result.CompleteSize,
			})
		}

		// Recurse into the named type's own dependencies
		switch resolved := v.Resolved.(type) {
		case *ast.Struct:
			ctx.collectDependencies(resolved, scope, minDeps, compDeps, seen)
		case *ast.Union:
			ctx.collectUnionDependencies(resolved, scope, minDeps, compDeps, seen)
		}
	}
}

// Helper functions

func primitiveTypeIdentifier(name string) *TypeIdentifier {
	var tk TypeKind
	switch name {
	case "boolean":
		tk = TK_BOOLEAN
	case "octet":
		tk = TK_BYTE
	case "uint8":
		tk = TK_UINT8
	case "int8":
		tk = TK_INT8
	case "char":
		tk = TK_CHAR8
	case "int16", "short":
		tk = TK_INT16
	case "uint16":
		tk = TK_UINT16
	case "int32", "long":
		tk = TK_INT32
	case "uint32":
		tk = TK_UINT32
	case "int64":
		tk = TK_INT64
	case "uint64":
		tk = TK_UINT64
	case "float", "float32":
		tk = TK_FLOAT32
	case "double", "float64":
		tk = TK_FLOAT64
	default:
		tk = TK_NONE
	}
	return &TypeIdentifier{Discriminator: tk}
}

func stringTypeIdentifier(bound int) *TypeIdentifier {
	if bound == 0 {
		return &TypeIdentifier{
			Discriminator: TI_STRING8_SMALL,
			StringSBound:  0,
		}
	}
	if bound <= 255 {
		return &TypeIdentifier{
			Discriminator: TI_STRING8_SMALL,
			StringSBound:  uint8(bound),
		}
	}
	return &TypeIdentifier{
		Discriminator: TI_STRING8_LARGE,
		StringLBound:  uint32(bound),
	}
}

// equivKindForElement returns the equiv_kind based on whether the element
// TypeIdentifier is fully descriptive. Primitives and strings are fully
// descriptive → EK_BOTH. Hashed types use the requested kind.
func equivKindForElement(elemTID *TypeIdentifier, kind byte) byte {
	if isFullyDescriptive(elemTID) {
		return EK_BOTH
	}
	return kind
}

// isFullyDescriptive returns true if the TypeIdentifier is fully descriptive
// (same in minimal and complete representations).
func isFullyDescriptive(tid *TypeIdentifier) bool {
	d := tid.Discriminator
	return isPrimitiveKind(d) ||
		d == TI_STRING8_SMALL || d == TI_STRING8_LARGE ||
		d == TI_STRING16_SMALL || d == TI_STRING16_LARGE
}

func sequenceTypeIdentifier(bound int, elemTID *TypeIdentifier) *TypeIdentifier {
	return makeSequenceTID(bound, elemTID, EK_MINIMAL)
}

func sequenceTypeIdentifierComplete(bound int, elemTID *TypeIdentifier) *TypeIdentifier {
	return makeSequenceTID(bound, elemTID, EK_COMPLETE)
}

func makeSequenceTID(bound int, elemTID *TypeIdentifier, kind byte) *TypeIdentifier {
	header := PlainCollectionHeader{
		EquivKind:    equivKindForElement(elemTID, kind),
		ElementFlags: MemberFlagTryConstruct1,
	}
	if bound <= 255 {
		return &TypeIdentifier{
			Discriminator: TI_PLAIN_SEQUENCE_SMALL,
			SeqSHeader:    header,
			SeqSBound:     uint8(bound),
			SeqSElemID:    elemTID,
		}
	}
	return &TypeIdentifier{
		Discriminator: TI_PLAIN_SEQUENCE_LARGE,
		SeqLHeader:    header,
		SeqLBound:     uint32(bound),
		SeqLElemID:    elemTID,
	}
}

func arrayTypeIdentifier(size int, elemTID *TypeIdentifier) *TypeIdentifier {
	return makeArrayTID(size, elemTID, EK_MINIMAL)
}

func arrayTypeIdentifierComplete(size int, elemTID *TypeIdentifier) *TypeIdentifier {
	return makeArrayTID(size, elemTID, EK_COMPLETE)
}

func makeArrayTID(size int, elemTID *TypeIdentifier, kind byte) *TypeIdentifier {
	header := PlainCollectionHeader{
		EquivKind:    equivKindForElement(elemTID, kind),
		ElementFlags: MemberFlagTryConstruct1,
	}
	if size <= 255 {
		return &TypeIdentifier{
			Discriminator: TI_PLAIN_ARRAY_SMALL,
			ArrSHeader:    header,
			ArrSBounds:    []uint8{uint8(size)},
			ArrSElemID:    elemTID,
		}
	}
	return &TypeIdentifier{
		Discriminator: TI_PLAIN_ARRAY_LARGE,
		ArrLHeader:    header,
		ArrLBounds:    []uint32{uint32(size)},
		ArrLElemID:    elemTID,
	}
}

func structFlags(s *ast.Struct) uint16 {
	var flags uint16
	if hasAnnotation(s.Annotations, "mutable") || annotationValue(s.Annotations, "extensibility") == "MUTABLE" {
		flags = TypeFlagIsMutable
	} else if hasAnnotation(s.Annotations, "appendable") || annotationValue(s.Annotations, "extensibility") == "APPENDABLE" {
		flags = TypeFlagIsAppendable
	} else {
		flags = TypeFlagIsFinal
	}
	return flags
}

func unionFlags(u *ast.Union) uint16 {
	if hasAnnotation(u.Annotations, "mutable") || annotationValue(u.Annotations, "extensibility") == "MUTABLE" {
		return TypeFlagIsMutable
	}
	if hasAnnotation(u.Annotations, "appendable") || annotationValue(u.Annotations, "extensibility") == "APPENDABLE" {
		return TypeFlagIsAppendable
	}
	return TypeFlagIsFinal
}

func hasAnnotation(annotations []ast.Annotation, name string) bool {
	for _, a := range annotations {
		if a.Name == name {
			return true
		}
	}
	return false
}

func annotationValue(annotations []ast.Annotation, name string) string {
	for _, a := range annotations {
		if a.Name == name {
			if v, ok := a.Params["value"]; ok {
				return strings.ToUpper(v)
			}
			if v, ok := a.Params[""]; ok {
				return strings.ToUpper(v)
			}
		}
	}
	return ""
}

func fullyQualifiedName(name string, scope []string) string {
	if strings.Contains(name, "::") {
		return name
	}
	if len(scope) == 0 {
		return name
	}
	return strings.Join(scope, "::") + "::" + name
}

func resolveUnderlying(t ast.TypeRef) ast.TypeRef {
	for {
		nt, ok := t.(*ast.NamedType)
		if !ok {
			return t
		}
		td, ok := nt.Resolved.(*ast.Typedef)
		if !ok {
			return t
		}
		switch td.Type.(type) {
		case *ast.ArrayType, *ast.SequenceType:
			return t
		}
		t = td.Type
	}
}

func parseLabelValue(label string) int32 {
	var v int32
	fmt.Sscanf(label, "%d", &v)
	return v
}

// FormatByteLiteral formats a byte slice as a Go []byte literal string.
func FormatByteLiteral(data []byte) string {
	if len(data) == 0 {
		return "[]byte{}"
	}
	var b strings.Builder
	b.WriteString("[]byte{")
	for i, v := range data {
		if i > 0 {
			b.WriteString(", ")
		}
		if i%16 == 0 {
			b.WriteString("\n\t\t")
		}
		fmt.Fprintf(&b, "0x%02x", v)
	}
	b.WriteString(",\n\t}")
	return b.String()
}
