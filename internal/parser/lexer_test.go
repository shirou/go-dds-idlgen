package parser

import (
	"testing"
)

func TestLexerIdents(t *testing.T) {
	src := `module struct enum`
	lex := NewLexer([]byte(src), "test.idl")

	tok := lex.Next()
	if tok.Kind != TokenIdent || tok.Value != "module" {
		t.Fatalf("expected ident 'module', got %s", tok)
	}
	tok = lex.Next()
	if tok.Kind != TokenIdent || tok.Value != "struct" {
		t.Fatalf("expected ident 'struct', got %s", tok)
	}
	tok = lex.Next()
	if tok.Kind != TokenIdent || tok.Value != "enum" {
		t.Fatalf("expected ident 'enum', got %s", tok)
	}
	tok = lex.Next()
	if tok.Kind != TokenEOF {
		t.Fatalf("expected EOF, got %s", tok)
	}
}

func TestLexerIntLiterals(t *testing.T) {
	src := `42 0xFF 0777 0`
	lex := NewLexer([]byte(src), "test.idl")

	tests := []struct {
		value string
	}{
		{"42"},
		{"0xFF"},
		{"0777"},
		{"0"},
	}

	for _, tt := range tests {
		tok := lex.Next()
		if tok.Kind != TokenIntLiteral {
			t.Fatalf("expected int literal, got %s", tok)
		}
		if tok.Value != tt.value {
			t.Fatalf("expected value %q, got %q", tt.value, tok.Value)
		}
	}
}

func TestLexerFloatLiteral(t *testing.T) {
	src := `3.14 1.0e10 2E-3`
	lex := NewLexer([]byte(src), "test.idl")

	tests := []string{"3.14", "1.0e10", "2E-3"}
	for _, want := range tests {
		tok := lex.Next()
		if tok.Kind != TokenFloatLiteral {
			t.Fatalf("expected float literal, got %s", tok)
		}
		if tok.Value != want {
			t.Fatalf("expected %q, got %q", want, tok.Value)
		}
	}
}

func TestLexerStringLiteral(t *testing.T) {
	src := `"hello world" "escape\n\t\"done"`
	lex := NewLexer([]byte(src), "test.idl")

	tok := lex.Next()
	if tok.Kind != TokenStringLiteral || tok.Value != "hello world" {
		t.Fatalf("unexpected: %s", tok)
	}
	tok = lex.Next()
	if tok.Kind != TokenStringLiteral || tok.Value != "escape\n\t\"done" {
		t.Fatalf("unexpected: %s value=%q", tok, tok.Value)
	}
}

func TestLexerCharLiteral(t *testing.T) {
	src := `'a' '\n'`
	lex := NewLexer([]byte(src), "test.idl")

	tok := lex.Next()
	if tok.Kind != TokenCharLiteral || tok.Value != "a" {
		t.Fatalf("unexpected: %s", tok)
	}
	tok = lex.Next()
	if tok.Kind != TokenCharLiteral || tok.Value != "\n" {
		t.Fatalf("unexpected: %s value=%q", tok, tok.Value)
	}
}

func TestLexerComments(t *testing.T) {
	src := `// line comment
module /* block
comment */ struct`
	lex := NewLexer([]byte(src), "test.idl")

	tok := lex.Next()
	if tok.Kind != TokenIdent || tok.Value != "module" {
		t.Fatalf("expected 'module', got %s", tok)
	}
	tok = lex.Next()
	if tok.Kind != TokenIdent || tok.Value != "struct" {
		t.Fatalf("expected 'struct', got %s", tok)
	}
}

func TestLexerScopeSep(t *testing.T) {
	src := `Foo::Bar::Baz`
	lex := NewLexer([]byte(src), "test.idl")

	expect := []struct {
		kind  TokenKind
		value string
	}{
		{TokenIdent, "Foo"},
		{TokenScopeSep, "::"},
		{TokenIdent, "Bar"},
		{TokenScopeSep, "::"},
		{TokenIdent, "Baz"},
	}
	for _, e := range expect {
		tok := lex.Next()
		if tok.Kind != e.kind || tok.Value != e.value {
			t.Fatalf("expected %s %q, got %s %q", e.kind, e.value, tok.Kind, tok.Value)
		}
	}
}

func TestLexerPunctuation(t *testing.T) {
	src := `{ } ( ) [ ] < > ; : , = @ #`
	lex := NewLexer([]byte(src), "test.idl")

	expect := []TokenKind{
		TokenLBrace, TokenRBrace, TokenLParen, TokenRParen,
		TokenLBracket, TokenRBracket, TokenLAngle, TokenRAngle,
		TokenSemicolon, TokenColon, TokenComma, TokenEquals,
		TokenAt, TokenHash,
	}
	for _, kind := range expect {
		tok := lex.Next()
		if tok.Kind != kind {
			t.Fatalf("expected %s, got %s", kind, tok.Kind)
		}
	}
}

func TestLexerLineCol(t *testing.T) {
	src := "abc\ndef"
	lex := NewLexer([]byte(src), "test.idl")

	tok := lex.Next()
	if tok.Line != 1 || tok.Col != 1 {
		t.Fatalf("expected 1:1, got %d:%d", tok.Line, tok.Col)
	}
	tok = lex.Next()
	if tok.Line != 2 || tok.Col != 1 {
		t.Fatalf("expected 2:1, got %d:%d", tok.Line, tok.Col)
	}
}

func TestLexerHexLiteral(t *testing.T) {
	src := `0xDEAD 0X1a2B`
	lex := NewLexer([]byte(src), "test.idl")

	tok := lex.Next()
	if tok.Kind != TokenIntLiteral || tok.Value != "0xDEAD" {
		t.Fatalf("unexpected: %s", tok)
	}
	tok = lex.Next()
	if tok.Kind != TokenIntLiteral || tok.Value != "0X1a2B" {
		t.Fatalf("unexpected: %s", tok)
	}
}
