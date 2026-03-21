package parser

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/shirou/go-dds-idlgen/internal/ast"
)

// Parser performs recursive descent parsing of IDL source.
type Parser struct {
	lexer  *Lexer
	tok    Token
	errors []error
}

// NewParser creates a new Parser for the given source bytes.
func NewParser(filename string, src []byte) *Parser {
	p := &Parser{
		lexer: NewLexer(src, filename),
	}
	p.next() // prime the first token
	return p
}

func (p *Parser) next() {
	p.tok = p.lexer.Next()
}

func (p *Parser) accept(kind TokenKind) bool {
	if p.tok.Kind == kind {
		p.next()
		return true
	}
	return false
}

func (p *Parser) expect(kind TokenKind) Token {
	tok := p.tok
	if tok.Kind != kind {
		p.addError(fmt.Sprintf("expected %s, got %s", kind, tok))
	}
	p.next()
	return tok
}

func (p *Parser) addError(msg string) {
	p.errors = append(p.errors, p.lexer.ErrorAt(p.tok.Line, p.tok.Col, msg))
}

// ParseFile parses a complete IDL file and returns the AST.
func (p *Parser) ParseFile() (*ast.File, error) {
	file := &ast.File{
		Name: p.lexer.filename,
	}

	for p.tok.Kind != TokenEOF {
		if p.tok.Kind == TokenHash {
			inc := p.parseDirective()
			if inc.Path != "" {
				file.Includes = append(file.Includes, inc)
			}
			continue
		}

		// Collect annotations before definitions.
		annotations := p.parseAnnotations()

		if p.tok.Kind == TokenEOF {
			break
		}

		def := p.parseDefinition(annotations)
		if def != nil {
			file.Definitions = append(file.Definitions, def)
		}
	}

	if len(p.errors) > 0 {
		return file, combineErrors(p.errors)
	}
	return file, nil
}

func (p *Parser) parseDirective() ast.Include {
	p.expect(TokenHash) // consume #
	if p.tok.Kind != TokenIdent {
		p.addError("expected directive name after '#'")
		return ast.Include{}
	}

	switch p.tok.Value {
	case "include":
		return p.parseIncludeDirective()
	case "ifndef", "define":
		p.next() // consume directive name
		if p.tok.Kind == TokenIdent {
			p.next() // consume macro name
		}
		return ast.Include{}
	case "endif":
		p.next() // consume "endif"
		return ast.Include{}
	default:
		p.addError(fmt.Sprintf("unsupported preprocessor directive #%s", p.tok.Value))
		p.next()
		return ast.Include{}
	}
}

func (p *Parser) parseIncludeDirective() ast.Include {
	p.next() // consume "include"

	// Handle both "file.idl" and <file.idl>.
	if p.tok.Kind == TokenStringLiteral {
		val := p.tok.Value
		p.next()
		return ast.Include{Path: val, System: false}
	}
	if p.tok.Kind == TokenLAngle {
		// Read until matching >.
		p.next()
		var buf strings.Builder
		for p.tok.Kind != TokenRAngle && p.tok.Kind != TokenEOF {
			buf.WriteString(p.tok.Value)
			p.next()
		}
		if p.tok.Kind == TokenRAngle {
			p.next()
		}
		return ast.Include{Path: buf.String(), System: true}
	}

	p.addError("expected string literal or <...> after #include")
	return ast.Include{}
}

func (p *Parser) parseAnnotations() []ast.Annotation {
	var anns []ast.Annotation
	for p.tok.Kind == TokenAt {
		p.next() // consume @
		if p.tok.Kind != TokenIdent {
			p.addError("expected annotation name after '@'")
			break
		}
		ann := ast.Annotation{
			Name: p.tok.Value,
		}
		p.next()

		// Optional parameters in parentheses.
		if p.tok.Kind == TokenLParen {
			p.next()
			ann.Params = make(map[string]string)
			if p.tok.Kind != TokenRParen {
				p.parseAnnotationParams(ann.Params)
			}
			p.expect(TokenRParen)
		}

		anns = append(anns, ann)
	}
	return anns
}

func (p *Parser) parseAnnotationParams(params map[string]string) {
	for {
		if p.tok.Kind == TokenRParen || p.tok.Kind == TokenEOF {
			return
		}

		// Could be a simple value like @extensibility(MUTABLE)
		// or a key=value pair like @id(value = 5)
		key := p.tok.Value
		p.next()

		if p.tok.Kind == TokenEquals {
			p.next() // consume =
			val := p.tok.Value
			p.next()
			params[key] = val
		} else {
			// Simple value: store as "value" key.
			params["value"] = key
		}

		if p.tok.Kind == TokenComma {
			p.next()
		}
	}
}

func (p *Parser) parseDefinition(annotations []ast.Annotation) ast.Definition {
	if p.tok.Kind == TokenSemicolon {
		p.next()
		return nil
	}

	if p.tok.Kind != TokenIdent {
		p.addError(fmt.Sprintf("expected definition keyword, got %s", p.tok))
		p.next()
		return nil
	}

	switch p.tok.Value {
	case "module":
		return p.parseModule()
	case "struct":
		return p.parseStruct(annotations)
	case "enum":
		return p.parseEnum(annotations)
	case "typedef":
		return p.parseTypedef()
	case "const":
		return p.parseConst()
	case "union":
		return p.parseUnion(annotations)
	case "interface", "valuetype", "bitset", "bitmask":
		return p.parseSkipped()
	default:
		p.addError(fmt.Sprintf("unexpected keyword %q", p.tok.Value))
		p.next()
		return nil
	}
}

func (p *Parser) parseModule() *ast.Module {
	p.next() // consume "module"
	name := p.expect(TokenIdent).Value
	p.expect(TokenLBrace)

	mod := &ast.Module{Name: name}
	for p.tok.Kind != TokenRBrace && p.tok.Kind != TokenEOF {
		annotations := p.parseAnnotations()
		if p.tok.Kind == TokenRBrace || p.tok.Kind == TokenEOF {
			break
		}
		def := p.parseDefinition(annotations)
		if def != nil {
			mod.Definitions = append(mod.Definitions, def)
		}
	}
	p.expect(TokenRBrace)
	p.accept(TokenSemicolon)
	return mod
}

func (p *Parser) parseStruct(annotations []ast.Annotation) *ast.Struct {
	p.next() // consume "struct"
	name := p.expect(TokenIdent).Value

	s := &ast.Struct{
		Name:        name,
		Annotations: annotations,
	}

	// Optional inheritance: struct Derived : Base { ... }
	if p.tok.Kind == TokenColon {
		p.next()
		s.Inherits = p.parseScopedName()
	}

	p.expect(TokenLBrace)

	for p.tok.Kind != TokenRBrace && p.tok.Kind != TokenEOF {
		field := p.parseField()
		s.Fields = append(s.Fields, field)
	}
	p.expect(TokenRBrace)
	p.expect(TokenSemicolon)
	return s
}

func (p *Parser) parseField() ast.Field {
	// Collect field annotations.
	annotations := p.parseAnnotations()

	typeRef := p.parseTypeRef()
	name := p.expect(TokenIdent).Value

	// Check for array dimensions after the field name.
	typeRef = p.parseArrayDimensions(typeRef)

	p.expect(TokenSemicolon)

	f := ast.Field{
		Name:        name,
		Type:        typeRef,
		Annotations: annotations,
	}

	// Extract @id annotation if present.
	for _, ann := range annotations {
		if ann.Name == "id" {
			if v, ok := ann.Params["value"]; ok {
				if n, err := strconv.ParseUint(v, 10, 32); err == nil {
					id := uint32(n)
					f.ID = &id
				}
			}
		}
	}

	return f
}

func (p *Parser) parseEnum(annotations []ast.Annotation) *ast.Enum {
	p.next() // consume "enum"
	name := p.expect(TokenIdent).Value
	p.expect(TokenLBrace)

	e := &ast.Enum{
		Name:        name,
		Annotations: annotations,
	}

	for p.tok.Kind != TokenRBrace && p.tok.Kind != TokenEOF {
		ev := ast.EnumValue{
			Name: p.expect(TokenIdent).Value,
		}
		if p.tok.Kind == TokenEquals {
			p.next()
			valStr := p.tok.Value
			p.next()
			if n, err := strconv.ParseInt(valStr, 0, 64); err == nil {
				ev.Value = &n
			}
		}
		e.Values = append(e.Values, ev)
		if p.tok.Kind == TokenComma {
			p.next()
		}
	}
	p.expect(TokenRBrace)
	p.expect(TokenSemicolon)
	return e
}

func (p *Parser) parseTypedef() *ast.Typedef {
	p.next() // consume "typedef"
	typeRef := p.parseTypeRef()
	name := p.expect(TokenIdent).Value

	// Check for array dimensions after the alias name.
	typeRef = p.parseArrayDimensions(typeRef)

	p.expect(TokenSemicolon)
	return &ast.Typedef{
		Name: name,
		Type: typeRef,
	}
}

func (p *Parser) parseConst() *ast.Const {
	p.next() // consume "const"
	typeRef := p.parseTypeRef()
	name := p.expect(TokenIdent).Value
	p.expect(TokenEquals)

	// Read the value. It can be an expression with identifiers, numbers, strings, etc.
	var valueParts []string
	for p.tok.Kind != TokenSemicolon && p.tok.Kind != TokenEOF {
		if p.tok.Kind == TokenStringLiteral {
			valueParts = append(valueParts, fmt.Sprintf("%q", p.tok.Value))
		} else {
			valueParts = append(valueParts, p.tok.Value)
		}
		p.next()
	}
	p.expect(TokenSemicolon)

	return &ast.Const{
		Name:  name,
		Type:  typeRef,
		Value: strings.Join(valueParts, " "),
	}
}

func (p *Parser) parseUnion(annotations []ast.Annotation) *ast.Union {
	p.next() // consume "union"
	name := p.expect(TokenIdent).Value

	// Expect "switch" keyword
	if p.tok.Kind != TokenIdent || p.tok.Value != "switch" {
		p.addError(fmt.Sprintf("expected 'switch' after union name, got %s", p.tok))
		return &ast.Union{Name: name, Annotations: annotations}
	}
	p.next() // consume "switch"

	// Parse discriminator type in parentheses
	p.expect(TokenLParen)
	discType := p.parseTypeRef()
	p.expect(TokenRParen)

	p.expect(TokenLBrace)

	u := &ast.Union{
		Name:          name,
		Discriminator: discType,
		Annotations:   annotations,
	}

	for p.tok.Kind != TokenRBrace && p.tok.Kind != TokenEOF {
		if p.tok.Kind == TokenIdent && p.tok.Value == "case" {
			uc := p.parseUnionCase()
			u.Cases = append(u.Cases, uc)
		} else if p.tok.Kind == TokenIdent && p.tok.Value == "default" {
			p.next() // consume "default"
			p.expect(TokenColon)
			caseType := p.parseTypeRef()
			caseName := p.expect(TokenIdent).Value
			p.expect(TokenSemicolon)
			u.DefaultCase = &ast.UnionCase{
				Type: caseType,
				Name: caseName,
			}
		} else {
			p.addError(fmt.Sprintf("expected 'case' or 'default' in union, got %s", p.tok))
			p.next()
		}
	}

	p.expect(TokenRBrace)
	p.expect(TokenSemicolon)
	return u
}

func (p *Parser) parseUnionCase() ast.UnionCase {
	var labels []string
	// Parse one or more "case LABEL:" prefixes (multiple labels for same branch)
	for p.tok.Kind == TokenIdent && p.tok.Value == "case" {
		p.next() // consume "case"
		label := p.parseUnionCaseLabel()
		labels = append(labels, label)
		p.expect(TokenColon)
	}

	caseType := p.parseTypeRef()
	caseName := p.expect(TokenIdent).Value
	p.expect(TokenSemicolon)

	return ast.UnionCase{
		Labels: labels,
		Type:   caseType,
		Name:   caseName,
	}
}

func (p *Parser) parseUnionCaseLabel() string {
	// Could be an integer literal, negative integer, or a scoped enum value name
	if p.tok.Kind == TokenIntLiteral {
		val := p.tok.Value
		p.next()
		return val
	}
	// Scoped or unscoped identifier (enum value)
	return p.parseScopedName()
}

func (p *Parser) parseSkipped() *ast.SkippedDecl {
	kind := p.tok.Value
	p.next() // consume keyword

	name := ""
	if p.tok.Kind == TokenIdent {
		name = p.tok.Value
		p.next()
	}

	// Skip tokens until we find a braced block or semicolon.
	// Some constructs (e.g. union ... switch (...)) have tokens before the brace.
	for p.tok.Kind != TokenLBrace && p.tok.Kind != TokenSemicolon && p.tok.Kind != TokenEOF {
		p.next()
	}
	if p.tok.Kind == TokenLBrace {
		p.skipBracedBlock()
	}
	p.accept(TokenSemicolon)

	return &ast.SkippedDecl{
		Kind:    kind,
		Name:    name,
		Warning: fmt.Sprintf("skipped %s %s: not supported", kind, name),
	}
}

func (p *Parser) skipBracedBlock() {
	if p.tok.Kind != TokenLBrace {
		return
	}
	depth := 0
	for p.tok.Kind != TokenEOF {
		if p.tok.Kind == TokenLBrace {
			depth++
		}
		if p.tok.Kind == TokenRBrace {
			depth--
			if depth == 0 {
				p.next() // consume the closing brace
				return
			}
		}
		p.next()
	}
}

func (p *Parser) parseTypeRef() ast.TypeRef {
	if p.tok.Kind != TokenIdent {
		p.addError(fmt.Sprintf("expected type, got %s", p.tok))
		p.next()
		return &ast.BasicType{Name: "error"}
	}

	switch p.tok.Value {
	case "boolean", "octet", "char", "wchar",
		"int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64":
		name := p.tok.Value
		p.next()
		return &ast.BasicType{Name: name}

	case "short":
		p.next()
		return &ast.BasicType{Name: "int16"}

	case "long":
		p.next()
		if p.tok.Kind == TokenIdent && p.tok.Value == "long" {
			p.next()
			return &ast.BasicType{Name: "int64"}
		}
		return &ast.BasicType{Name: "int32"}

	case "unsigned":
		p.next()
		if p.tok.Kind == TokenIdent && p.tok.Value == "short" {
			p.next()
			return &ast.BasicType{Name: "uint16"}
		}
		if p.tok.Kind == TokenIdent && p.tok.Value == "long" {
			p.next()
			if p.tok.Kind == TokenIdent && p.tok.Value == "long" {
				p.next()
				return &ast.BasicType{Name: "uint64"}
			}
			return &ast.BasicType{Name: "uint32"}
		}
		p.addError("expected 'short' or 'long' after 'unsigned'")
		return &ast.BasicType{Name: "error"}

	case "float":
		p.next()
		return &ast.BasicType{Name: "float"}

	case "double":
		p.next()
		return &ast.BasicType{Name: "double"}

	case "string", "wstring":
		p.next()
		bound := 0
		if p.tok.Kind == TokenLAngle {
			p.next()
			if p.tok.Kind == TokenIntLiteral {
				bound, _ = strconv.Atoi(p.tok.Value)
				p.next()
			}
			p.expect(TokenRAngle)
		}
		return &ast.StringType{Bound: bound}

	case "sequence":
		p.next()
		p.expect(TokenLAngle)
		elemType := p.parseTypeRef()
		bound := 0
		if p.tok.Kind == TokenComma {
			p.next()
			if p.tok.Kind == TokenIntLiteral {
				bound, _ = strconv.Atoi(p.tok.Value)
				p.next()
			}
		}
		p.expect(TokenRAngle)
		return &ast.SequenceType{ElemType: elemType, Bound: bound}

	default:
		// Named type, potentially scoped.
		return &ast.NamedType{Name: p.parseScopedName()}
	}
}

func (p *Parser) parseScopedName() string {
	var parts []string
	parts = append(parts, p.tok.Value)
	p.next()
	for p.tok.Kind == TokenScopeSep {
		p.next() // consume ::
		if p.tok.Kind == TokenIdent {
			parts = append(parts, p.tok.Value)
			p.next()
		}
	}
	return strings.Join(parts, "::")
}

func (p *Parser) parseArrayDimensions(base ast.TypeRef) ast.TypeRef {
	result := base
	for p.tok.Kind == TokenLBracket {
		p.next() // consume [
		sizeStr := p.expect(TokenIntLiteral).Value
		size, _ := strconv.Atoi(sizeStr)
		p.expect(TokenRBracket)
		result = &ast.ArrayType{ElemType: result, Size: size}
	}
	return result
}

// Parse is a convenience function that parses the given IDL source and returns the AST.
func Parse(filename string, src []byte) (*ast.File, error) {
	p := NewParser(filename, src)
	return p.ParseFile()
}

func combineErrors(errs []error) error {
	return errors.Join(errs...)
}
