package parser

import (
	"fmt"
	"strings"
)

// TokenKind represents the type of a lexer token.
type TokenKind int

const (
	TokenEOF TokenKind = iota
	TokenIdent
	TokenIntLiteral
	TokenFloatLiteral
	TokenStringLiteral
	TokenCharLiteral

	// Punctuation tokens.
	TokenLBrace    // {
	TokenRBrace    // }
	TokenLParen    // (
	TokenRParen    // )
	TokenLBracket  // [
	TokenRBracket  // ]
	TokenLAngle    // <
	TokenRAngle    // >
	TokenSemicolon // ;
	TokenColon     // :
	TokenComma     // ,
	TokenEquals    // =
	TokenAt        // @
	TokenHash      // #
	TokenScopeSep  // ::
)

var tokenKindNames = map[TokenKind]string{
	TokenEOF:           "EOF",
	TokenIdent:         "identifier",
	TokenIntLiteral:    "integer literal",
	TokenFloatLiteral:  "float literal",
	TokenStringLiteral: "string literal",
	TokenCharLiteral:   "char literal",
	TokenLBrace:        "{",
	TokenRBrace:        "}",
	TokenLParen:        "(",
	TokenRParen:        ")",
	TokenLBracket:      "[",
	TokenRBracket:      "]",
	TokenLAngle:        "<",
	TokenRAngle:        ">",
	TokenSemicolon:     ";",
	TokenColon:         ":",
	TokenComma:         ",",
	TokenEquals:        "=",
	TokenAt:            "@",
	TokenHash:          "#",
	TokenScopeSep:      "::",
}

func (k TokenKind) String() string {
	if s, ok := tokenKindNames[k]; ok {
		return s
	}
	return fmt.Sprintf("TokenKind(%d)", int(k))
}

// Token represents a single lexer token.
type Token struct {
	Kind  TokenKind
	Value string
	Line  int
	Col   int
}

func (t Token) String() string {
	if t.Value != "" {
		return fmt.Sprintf("%s(%q)", t.Kind, t.Value)
	}
	return t.Kind.String()
}

// Lexer tokenizes IDL source input.
type Lexer struct {
	src      []byte
	pos      int
	line     int
	col      int
	filename string
}

// NewLexer creates a new Lexer for the given source.
func NewLexer(src []byte, filename string) *Lexer {
	return &Lexer{
		src:      src,
		pos:      0,
		line:     1,
		col:      1,
		filename: filename,
	}
}

func (l *Lexer) peek() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) advance() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	ch := l.src[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.src) && isSpace(l.src[l.pos]) {
		l.advance()
	}
}

func (l *Lexer) skipLineComment() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
}

func (l *Lexer) skipBlockComment() {
	// Already consumed "/*".
	for l.pos < len(l.src) {
		if l.src[l.pos] == '*' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/' {
			l.advance() // *
			l.advance() // /
			return
		}
		l.advance()
	}
}

func (l *Lexer) skipWhitespaceAndComments() {
	for {
		l.skipWhitespace()
		if l.pos+1 < len(l.src) && l.src[l.pos] == '/' && l.src[l.pos+1] == '/' {
			l.advance()
			l.advance()
			l.skipLineComment()
			continue
		}
		if l.pos+1 < len(l.src) && l.src[l.pos] == '/' && l.src[l.pos+1] == '*' {
			l.advance()
			l.advance()
			l.skipBlockComment()
			continue
		}
		break
	}
}

// Next returns the next token from the input.
func (l *Lexer) Next() Token {
	l.skipWhitespaceAndComments()

	if l.pos >= len(l.src) {
		return Token{Kind: TokenEOF, Line: l.line, Col: l.col}
	}

	startLine := l.line
	startCol := l.col
	ch := l.src[l.pos]

	// String literal.
	if ch == '"' {
		return l.readStringLiteral(startLine, startCol)
	}

	// Char literal.
	if ch == '\'' {
		return l.readCharLiteral(startLine, startCol)
	}

	// Number literal.
	if isDigit(ch) || (ch == '-' && l.pos+1 < len(l.src) && isDigit(l.src[l.pos+1])) {
		return l.readNumber(startLine, startCol)
	}

	// Identifier or keyword.
	if isIdentStart(ch) {
		return l.readIdent(startLine, startCol)
	}

	// Punctuation.
	switch ch {
	case '{':
		l.advance()
		return Token{Kind: TokenLBrace, Value: "{", Line: startLine, Col: startCol}
	case '}':
		l.advance()
		return Token{Kind: TokenRBrace, Value: "}", Line: startLine, Col: startCol}
	case '(':
		l.advance()
		return Token{Kind: TokenLParen, Value: "(", Line: startLine, Col: startCol}
	case ')':
		l.advance()
		return Token{Kind: TokenRParen, Value: ")", Line: startLine, Col: startCol}
	case '[':
		l.advance()
		return Token{Kind: TokenLBracket, Value: "[", Line: startLine, Col: startCol}
	case ']':
		l.advance()
		return Token{Kind: TokenRBracket, Value: "]", Line: startLine, Col: startCol}
	case '<':
		l.advance()
		return Token{Kind: TokenLAngle, Value: "<", Line: startLine, Col: startCol}
	case '>':
		l.advance()
		return Token{Kind: TokenRAngle, Value: ">", Line: startLine, Col: startCol}
	case ';':
		l.advance()
		return Token{Kind: TokenSemicolon, Value: ";", Line: startLine, Col: startCol}
	case ':':
		l.advance()
		if l.pos < len(l.src) && l.src[l.pos] == ':' {
			l.advance()
			return Token{Kind: TokenScopeSep, Value: "::", Line: startLine, Col: startCol}
		}
		return Token{Kind: TokenColon, Value: ":", Line: startLine, Col: startCol}
	case ',':
		l.advance()
		return Token{Kind: TokenComma, Value: ",", Line: startLine, Col: startCol}
	case '=':
		l.advance()
		return Token{Kind: TokenEquals, Value: "=", Line: startLine, Col: startCol}
	case '@':
		l.advance()
		return Token{Kind: TokenAt, Value: "@", Line: startLine, Col: startCol}
	case '#':
		l.advance()
		return Token{Kind: TokenHash, Value: "#", Line: startLine, Col: startCol}
	}

	// Unknown character: skip it and return an identifier token with the character.
	l.advance()
	return Token{Kind: TokenIdent, Value: string(ch), Line: startLine, Col: startCol}
}

func (l *Lexer) readStringLiteral(startLine, startCol int) Token {
	l.advance() // consume opening quote
	var buf strings.Builder
	for l.pos < len(l.src) && l.src[l.pos] != '"' {
		if l.src[l.pos] == '\\' && l.pos+1 < len(l.src) {
			l.advance() // backslash
			ch := l.advance()
			switch ch {
			case 'n':
				buf.WriteByte('\n')
			case 't':
				buf.WriteByte('\t')
			case 'r':
				buf.WriteByte('\r')
			case '\\':
				buf.WriteByte('\\')
			case '"':
				buf.WriteByte('"')
			default:
				buf.WriteByte('\\')
				buf.WriteByte(ch)
			}
		} else {
			buf.WriteByte(l.advance())
		}
	}
	if l.pos < len(l.src) {
		l.advance() // consume closing quote
	}
	return Token{Kind: TokenStringLiteral, Value: buf.String(), Line: startLine, Col: startCol}
}

func (l *Lexer) readCharLiteral(startLine, startCol int) Token {
	l.advance() // consume opening single quote
	var buf strings.Builder
	for l.pos < len(l.src) && l.src[l.pos] != '\'' {
		if l.src[l.pos] == '\\' && l.pos+1 < len(l.src) {
			l.advance()
			ch := l.advance()
			switch ch {
			case 'n':
				buf.WriteByte('\n')
			case 't':
				buf.WriteByte('\t')
			case '\\':
				buf.WriteByte('\\')
			case '\'':
				buf.WriteByte('\'')
			default:
				buf.WriteByte('\\')
				buf.WriteByte(ch)
			}
		} else {
			buf.WriteByte(l.advance())
		}
	}
	if l.pos < len(l.src) {
		l.advance() // consume closing single quote
	}
	return Token{Kind: TokenCharLiteral, Value: buf.String(), Line: startLine, Col: startCol}
}

func (l *Lexer) readNumber(startLine, startCol int) Token {
	var buf strings.Builder
	isFloat := false

	if l.src[l.pos] == '-' {
		buf.WriteByte(l.advance())
	}

	// Check for hex or octal prefix.
	if l.src[l.pos] == '0' && l.pos+1 < len(l.src) {
		next := l.src[l.pos+1]
		if next == 'x' || next == 'X' {
			buf.WriteByte(l.advance()) // 0
			buf.WriteByte(l.advance()) // x
			for l.pos < len(l.src) && isHexDigit(l.src[l.pos]) {
				buf.WriteByte(l.advance())
			}
			return Token{Kind: TokenIntLiteral, Value: buf.String(), Line: startLine, Col: startCol}
		}
	}

	// Decimal or octal digits.
	for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
		buf.WriteByte(l.advance())
	}

	// Fractional part.
	if l.pos < len(l.src) && l.src[l.pos] == '.' {
		isFloat = true
		buf.WriteByte(l.advance())
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			buf.WriteByte(l.advance())
		}
	}

	// Exponent part.
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		isFloat = true
		buf.WriteByte(l.advance())
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			buf.WriteByte(l.advance())
		}
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			buf.WriteByte(l.advance())
		}
	}

	kind := TokenIntLiteral
	if isFloat {
		kind = TokenFloatLiteral
	}
	return Token{Kind: kind, Value: buf.String(), Line: startLine, Col: startCol}
}

func (l *Lexer) readIdent(startLine, startCol int) Token {
	var buf strings.Builder
	for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
		buf.WriteByte(l.advance())
	}
	return Token{Kind: TokenIdent, Value: buf.String(), Line: startLine, Col: startCol}
}

func isSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentPart(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}

// isKeyword reports whether ident is a recognized IDL keyword.
func isKeyword(ident string) bool {
	switch ident {
	case "module", "struct", "enum", "typedef", "const",
		"sequence", "string", "wstring",
		"boolean", "octet", "char", "wchar",
		"short", "unsigned", "long", "float", "double",
		"int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64",
		"TRUE", "FALSE", "include",
		"interface", "union", "valuetype", "bitset", "bitmask", "annotation",
		"map":
		return true
	}
	return false
}

// ErrorAt formats a lexer error with file, line, and column information.
func (l *Lexer) ErrorAt(line, col int, msg string) error {
	return fmt.Errorf("%s:%d:%d: %s", l.filename, line, col, msg)
}
