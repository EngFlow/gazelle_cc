// Copyright 2025 EngFlow Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextToken(t *testing.T) {
	testCases := []struct {
		input    []byte
		expected Token
	}{
		{
			input:    []byte(""),
			expected: TokenEmpty,
		},
		{
			input:    []byte("&&"),
			expected: Token{Type: TokenType_Symbol, Location: CursorInit, Content: "&&"},
		},
		{
			input:    []byte("#include \"file.h\""),
			expected: Token{Type: TokenType_PreprocessorDirective, Location: CursorInit, Content: "#include"},
		},
		{
			input:    []byte("#   define VARIABLE 123"),
			expected: Token{Type: TokenType_PreprocessorDirective, Location: CursorInit, Content: "#   define"},
		},
		{
			input:    []byte("\n\n"),
			expected: Token{Type: TokenType_Newline, Location: CursorInit, Content: "\n"},
		},
		{
			input:    []byte("\t\t abc"),
			expected: Token{Type: TokenType_Whitespace, Location: CursorInit, Content: "\t\t "},
		},
		{
			input:    []byte("\\\n MACRO_CONTINUED"),
			expected: Token{Type: TokenType_ContinueLine, Location: CursorInit, Content: "\\\n"},
		},
		{
			input:    []byte("\\    \n MACRO_CONTINUED"),
			expected: Token{Type: TokenType_ContinueLine, Location: CursorInit, Content: "\\    \n"},
		},
		{
			input:    []byte("\\ unexpected \n MACRO_CONTINUED"),
			expected: Token{Type: TokenType_Word, Location: CursorInit, Content: "\\"},
		},
		{
			input:    []byte("// This is a single line comment"),
			expected: Token{Type: TokenType_SingleLineComment, Location: CursorInit, Content: "// This is a single line comment"},
		},
		{
			input:    []byte("// This is a single line comment\nint main()"),
			expected: Token{Type: TokenType_SingleLineComment, Location: CursorInit, Content: "// This is a single line comment"},
		},
		{
			input:    []byte("/*\n  This is a multi line comment\n*/\nint main()"),
			expected: Token{Type: TokenType_MultiLineComment, Location: CursorInit, Content: "/*\n  This is a multi line comment\n*/"},
		},
		{
			// TODO handle string literals as whole tokens
			input:    []byte(`"This is a string literal"`),
			expected: Token{Type: TokenType_Word, Location: CursorInit, Content: `"This`},
		},
		{
			// TODO handle raw string literals as whole tokens
			input:    []byte(`R"(abc)" fake-end)"`),
			expected: Token{Type: TokenType_Word, Location: CursorInit, Content: `R"`},
		},
		{
			input:    []byte("identifier123;"),
			expected: Token{Type: TokenType_Word, Location: CursorInit, Content: "identifier123"},
		},
	}

	for _, tc := range testCases {
		lx := NewLexer(tc.input)
		assert.Equal(t, tc.expected, lx.NextToken(), "input: %q", tc.input)
	}
}

func TestTokenize(t *testing.T) {
	testCases := []struct {
		input    []byte
		expected []Token
	}{
		{
			input:    []byte(""),
			expected: nil,
		},
		{
			input: []byte("int main() { return 0; }"),
			expected: []Token{
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 1}, Content: "int"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 4}, Content: " "},
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 5}, Content: "main"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 9}, Content: "("},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 10}, Content: ")"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 11}, Content: " "},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 12}, Content: "{"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 13}, Content: " "},
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 14}, Content: "return"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 20}, Content: " "},
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 21}, Content: "0"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 22}, Content: ";"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 23}, Content: " "},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 24}, Content: "}"},
			},
		},
		{
			input: []byte("/*\nint main() { return 0; }\n*/\nint main() { return 0; }"),
			expected: []Token{
				{Type: TokenType_MultiLineComment, Location: Cursor{Line: 1, Column: 1}, Content: "/*\nint main() { return 0; }\n*/"},
				{Type: TokenType_Newline, Location: Cursor{Line: 3, Column: 3}, Content: "\n"},
				{Type: TokenType_Word, Location: Cursor{Line: 4, Column: 1}, Content: "int"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 4, Column: 4}, Content: " "},
				{Type: TokenType_Word, Location: Cursor{Line: 4, Column: 5}, Content: "main"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 4, Column: 9}, Content: "("},
				{Type: TokenType_Symbol, Location: Cursor{Line: 4, Column: 10}, Content: ")"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 4, Column: 11}, Content: " "},
				{Type: TokenType_Symbol, Location: Cursor{Line: 4, Column: 12}, Content: "{"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 4, Column: 13}, Content: " "},
				{Type: TokenType_Word, Location: Cursor{Line: 4, Column: 14}, Content: "return"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 4, Column: 20}, Content: " "},
				{Type: TokenType_Word, Location: Cursor{Line: 4, Column: 21}, Content: "0"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 4, Column: 22}, Content: ";"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 4, Column: 23}, Content: " "},
				{Type: TokenType_Symbol, Location: Cursor{Line: 4, Column: 24}, Content: "}"},
			},
		},
		{
			input: []byte("#define SQUARE(x)\\\n((x)*(x))"),
			expected: []Token{
				{Type: TokenType_PreprocessorDirective, Location: Cursor{Line: 1, Column: 1}, Content: "#define"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 8}, Content: " "},
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 9}, Content: "SQUARE"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 15}, Content: "("},
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 16}, Content: "x"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 17}, Content: ")"},
				{Type: TokenType_ContinueLine, Location: Cursor{Line: 1, Column: 18}, Content: "\\\n"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 2, Column: 1}, Content: "("},
				{Type: TokenType_Symbol, Location: Cursor{Line: 2, Column: 2}, Content: "("},
				{Type: TokenType_Word, Location: Cursor{Line: 2, Column: 3}, Content: "x"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 2, Column: 4}, Content: ")"},
				{Type: TokenType_Word, Location: Cursor{Line: 2, Column: 5}, Content: "*"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 2, Column: 6}, Content: "("},
				{Type: TokenType_Word, Location: Cursor{Line: 2, Column: 7}, Content: "x"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 2, Column: 8}, Content: ")"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 2, Column: 9}, Content: ")"},
			},
		},
		{
			input: []byte("int main() { /*\n"),
			expected: []Token{
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 1}, Content: "int"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 4}, Content: " "},
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 5}, Content: "main"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 9}, Content: "("},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 10}, Content: ")"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 11}, Content: " "},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 12}, Content: "{"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 13}, Content: " "},
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 14}, Content: "/*"},
				{Type: TokenType_Newline, Location: Cursor{Line: 1, Column: 16}, Content: "\n"},
			},
		},
		{
			input: []byte("word/*unterminated comment"),
			expected: []Token{
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 1}, Content: "word/*unterminated"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 19}, Content: " "},
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 20}, Content: "comment"},
			},
		},
		{
			input: []byte("/*😎*/ // This starts at column 7"),
			expected: []Token{
				{Type: TokenType_MultiLineComment, Location: Cursor{Line: 1, Column: 1}, Content: "/*😎*/"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 6}, Content: " "},
				{Type: TokenType_SingleLineComment, Location: Cursor{Line: 1, Column: 7}, Content: "// This starts at column 7"},
			},
		},
	}

	for _, tc := range testCases {
		lx := NewLexer(tc.input)
		assert.Equal(t, tc.expected, lx.Tokenize(), "input: %q", tc.input)
	}
}

func TestExtractDirectiveName(t *testing.T) {
	testCases := []struct {
		input    Token
		expected string
	}{
		{
			input:    TokenEmpty,
			expected: "",
		},
		{
			input:    Token{Type: TokenType_Word, Location: CursorInit, Content: "identifier"},
			expected: "",
		},
		{
			input:    Token{Type: TokenType_PreprocessorDirective, Location: CursorInit, Content: "#include"},
			expected: "include",
		},
		{
			input:    Token{Type: TokenType_PreprocessorDirective, Location: CursorInit, Content: "#   define"},
			expected: "define",
		},
		{
			input:    Token{Type: TokenType_PreprocessorDirective, Location: CursorInit, Content: "#\tendif"},
			expected: "endif",
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, ExtractDirectiveName(tc.input), "input: %+v", tc.input)
	}
}
