// Copyright 2026 EngFlow Inc. All rights reserved.
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
	"bytes"
	"slices"
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
			expected: TokenEOF,
		},
		{
			input:    []byte("&&"),
			expected: Token{Type: TokenType_OperatorLogicalAnd, Location: CursorInit, Content: "&&"},
		},
		{
			input:    []byte("#include \"file.h\""),
			expected: Token{Type: TokenType_PreprocessorInclude, Location: CursorInit, Content: "#include"},
		},
		{
			input:    []byte("#   define VARIABLE 123"),
			expected: Token{Type: TokenType_PreprocessorDefine, Location: CursorInit, Content: "#   define"},
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
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: "\\"},
		},
		{
			input:    []byte("// This is a single line comment"),
			expected: Token{Type: TokenType_CommentSingleLine, Location: CursorInit, Content: "// This is a single line comment"},
		},
		{
			input:    []byte("// This is a single line comment\nint main()"),
			expected: Token{Type: TokenType_CommentSingleLine, Location: CursorInit, Content: "// This is a single line comment"},
		},
		{
			input:    []byte("/*\n  This is a multi line comment\n*/\nint main()"),
			expected: Token{Type: TokenType_CommentMultiLine, Location: CursorInit, Content: "/*\n  This is a multi line comment\n*/"},
		},
		{
			input:    []byte(`"This is a string literal"`),
			expected: Token{Type: TokenType_LiteralString, Location: CursorInit, Content: `"This is a string literal"`},
		},
		{
			input:    []byte(`"I contain a \"quoted\" text"`),
			expected: Token{Type: TokenType_LiteralString, Location: CursorInit, Content: `"I contain a \"quoted\" text"`},
		},
		{
			input:    []byte(`"I contain a '\\' backslash"`),
			expected: Token{Type: TokenType_LiteralString, Location: CursorInit, Content: `"I contain a '\\' backslash"`},
		},
		{
			input:    []byte(`L"wide string literal"`),
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: `L"wide string literal"`},
		},
		{
			input:    []byte(`u8"utf-8 string literal"`),
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: `u8"utf-8 string literal"`},
		},
		{
			input:    []byte(`u"utf-16 string literal"`),
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: `u"utf-16 string literal"`},
		},
		{
			input:    []byte(`U"utf-32 string literal"`),
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: `U"utf-32 string literal"`},
		},
		{
			input:    []byte(`R"(abc)" fake-end)"`),
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: `R"(abc)"`},
		},
		{
			input:    []byte(`R"delim(abc)delim" fake-end)"`),
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: `R"delim(abc)delim"`},
		},
		{
			input:    []byte(`R"delim(abc fake-end)" )delim"`),
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: `R"delim(abc fake-end)" )delim"`},
		},
		{
			input:    []byte(`LR"(wide raw string literal)"`),
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: `LR"(wide raw string literal)"`},
		},
		{
			input:    []byte(`u8R"(utf-8 raw string literal)"`),
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: `u8R"(utf-8 raw string literal)"`},
		},
		{
			input:    []byte(`uR"(utf-16 raw string literal)"`),
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: `uR"(utf-16 raw string literal)"`},
		},
		{
			input:    []byte(`UR"(utf-32 raw string literal)"`),
			expected: Token{Type: TokenType_Unassigned, Location: CursorInit, Content: `UR"(utf-32 raw string literal)"`},
		},
		{
			input:    []byte("identifier123;"),
			expected: Token{Type: TokenType_Identifier, Location: CursorInit, Content: "identifier123"},
		},
		{
			input:    []byte("12345;"),
			expected: Token{Type: TokenType_LiteralInteger, Location: CursorInit, Content: "12345"},
		},
		{
			input:    []byte("0x1A3F;"),
			expected: Token{Type: TokenType_LiteralInteger, Location: CursorInit, Content: "0x1A3F"},
		},
		{
			input:    []byte("0755;"),
			expected: Token{Type: TokenType_LiteralInteger, Location: CursorInit, Content: "0755"},
		},
		{
			input:    []byte("0b1101;"),
			expected: Token{Type: TokenType_LiteralInteger, Location: CursorInit, Content: "0b1101"},
		},
	}

	for _, tc := range testCases {
		lx := NewLexer(tc.input)
		assert.Equal(t, tc.expected, lx.NextToken(), "input: %q", tc.input)
	}
}

func TestAllTokens(t *testing.T) {
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
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 1}, Content: "int"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 4}, Content: " "},
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 5}, Content: "main"},
				{Type: TokenType_ParenthesisLeft, Location: Cursor{Line: 1, Column: 9}, Content: "("},
				{Type: TokenType_ParenthesisRight, Location: Cursor{Line: 1, Column: 10}, Content: ")"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 11}, Content: " "},
				{Type: TokenType_BraceLeft, Location: Cursor{Line: 1, Column: 12}, Content: "{"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 13}, Content: " "},
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 14}, Content: "return"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 20}, Content: " "},
				{Type: TokenType_LiteralInteger, Location: Cursor{Line: 1, Column: 21}, Content: "0"},
				{Type: TokenType_Semicolon, Location: Cursor{Line: 1, Column: 22}, Content: ";"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 23}, Content: " "},
				{Type: TokenType_BraceRight, Location: Cursor{Line: 1, Column: 24}, Content: "}"},
			},
		},
		{
			input: []byte("/*\nint main() { return 0; }\n*/\nint main() { return 0; }"),
			expected: []Token{
				{Type: TokenType_CommentMultiLine, Location: Cursor{Line: 1, Column: 1}, Content: "/*\nint main() { return 0; }\n*/"},
				{Type: TokenType_Newline, Location: Cursor{Line: 3, Column: 3}, Content: "\n"},
				{Type: TokenType_Identifier, Location: Cursor{Line: 4, Column: 1}, Content: "int"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 4, Column: 4}, Content: " "},
				{Type: TokenType_Identifier, Location: Cursor{Line: 4, Column: 5}, Content: "main"},
				{Type: TokenType_ParenthesisLeft, Location: Cursor{Line: 4, Column: 9}, Content: "("},
				{Type: TokenType_ParenthesisRight, Location: Cursor{Line: 4, Column: 10}, Content: ")"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 4, Column: 11}, Content: " "},
				{Type: TokenType_BraceLeft, Location: Cursor{Line: 4, Column: 12}, Content: "{"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 4, Column: 13}, Content: " "},
				{Type: TokenType_Identifier, Location: Cursor{Line: 4, Column: 14}, Content: "return"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 4, Column: 20}, Content: " "},
				{Type: TokenType_LiteralInteger, Location: Cursor{Line: 4, Column: 21}, Content: "0"},
				{Type: TokenType_Semicolon, Location: Cursor{Line: 4, Column: 22}, Content: ";"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 4, Column: 23}, Content: " "},
				{Type: TokenType_BraceRight, Location: Cursor{Line: 4, Column: 24}, Content: "}"},
			},
		},
		{
			input: []byte("#define SQUARE(x)\\\n((x)*(x))"),
			expected: []Token{
				{Type: TokenType_PreprocessorDefine, Location: Cursor{Line: 1, Column: 1}, Content: "#define"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 8}, Content: " "},
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 9}, Content: "SQUARE"},
				{Type: TokenType_ParenthesisLeft, Location: Cursor{Line: 1, Column: 15}, Content: "("},
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 16}, Content: "x"},
				{Type: TokenType_ParenthesisRight, Location: Cursor{Line: 1, Column: 17}, Content: ")"},
				{Type: TokenType_ContinueLine, Location: Cursor{Line: 1, Column: 18}, Content: "\\\n"},
				{Type: TokenType_ParenthesisLeft, Location: Cursor{Line: 2, Column: 1}, Content: "("},
				{Type: TokenType_ParenthesisLeft, Location: Cursor{Line: 2, Column: 2}, Content: "("},
				{Type: TokenType_Identifier, Location: Cursor{Line: 2, Column: 3}, Content: "x"},
				{Type: TokenType_ParenthesisRight, Location: Cursor{Line: 2, Column: 4}, Content: ")"},
				{Type: TokenType_Unassigned, Location: Cursor{Line: 2, Column: 5}, Content: "*"},
				{Type: TokenType_ParenthesisLeft, Location: Cursor{Line: 2, Column: 6}, Content: "("},
				{Type: TokenType_Identifier, Location: Cursor{Line: 2, Column: 7}, Content: "x"},
				{Type: TokenType_ParenthesisRight, Location: Cursor{Line: 2, Column: 8}, Content: ")"},
				{Type: TokenType_ParenthesisRight, Location: Cursor{Line: 2, Column: 9}, Content: ")"},
			},
		},
		{
			input: []byte("int main() { /*\n"),
			expected: []Token{
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 1}, Content: "int"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 4}, Content: " "},
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 5}, Content: "main"},
				{Type: TokenType_ParenthesisLeft, Location: Cursor{Line: 1, Column: 9}, Content: "("},
				{Type: TokenType_ParenthesisRight, Location: Cursor{Line: 1, Column: 10}, Content: ")"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 11}, Content: " "},
				{Type: TokenType_BraceLeft, Location: Cursor{Line: 1, Column: 12}, Content: "{"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 13}, Content: " "},
				{Type: TokenType_Unassigned, Location: Cursor{Line: 1, Column: 14}, Content: "/*"},
				{Type: TokenType_Newline, Location: Cursor{Line: 1, Column: 16}, Content: "\n"},
			},
		},
		{
			input: []byte("word/*unterminated comment"),
			expected: []Token{
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 1}, Content: "word"},
				{Type: TokenType_Unassigned, Location: Cursor{Line: 1, Column: 5}, Content: "/*"},
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 7}, Content: "unterminated"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 19}, Content: " "},
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 20}, Content: "comment"},
			},
		},
		{
			input: []byte("/*😎*/ // This starts at column 7"),
			expected: []Token{
				{Type: TokenType_CommentMultiLine, Location: Cursor{Line: 1, Column: 1}, Content: "/*😎*/"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 6}, Content: " "},
				{Type: TokenType_CommentSingleLine, Location: Cursor{Line: 1, Column: 7}, Content: "// This starts at column 7"},
			},
		},
		{
			input: []byte("#if defined(__APPLE__) && __cplusplus >= 201103L"),
			expected: []Token{
				{Type: TokenType_PreprocessorIf, Location: Cursor{Line: 1, Column: 1}, Content: "#if"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 4}, Content: " "},
				{Type: TokenType_PreprocessorDefined, Location: Cursor{Line: 1, Column: 5}, Content: "defined"},
				{Type: TokenType_ParenthesisLeft, Location: Cursor{Line: 1, Column: 12}, Content: "("},
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 13}, Content: "__APPLE__"},
				{Type: TokenType_ParenthesisRight, Location: Cursor{Line: 1, Column: 22}, Content: ")"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 23}, Content: " "},
				{Type: TokenType_OperatorLogicalAnd, Location: Cursor{Line: 1, Column: 24}, Content: "&&"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 26}, Content: " "},
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 27}, Content: "__cplusplus"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 38}, Content: " "},
				{Type: TokenType_OperatorGreaterOrEqual, Location: Cursor{Line: 1, Column: 39}, Content: ">="},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 41}, Content: " "},
				{Type: TokenType_LiteralInteger, Location: Cursor{Line: 1, Column: 42}, Content: "201103"},
				{Type: TokenType_Identifier, Location: Cursor{Line: 1, Column: 48}, Content: "L"},
			},
		},
		{
			input: []byte(`#include "mylib.h"`),
			expected: []Token{
				{Type: TokenType_PreprocessorInclude, Location: Cursor{Line: 1, Column: 1}, Content: "#include"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 9}, Content: " "},
				{Type: TokenType_LiteralString, Location: Cursor{Line: 1, Column: 10}, Content: `"mylib.h"`},
			},
		},
		{
			input: []byte("#include <gtest/gtest.h>"),
			expected: []Token{
				{Type: TokenType_PreprocessorInclude, Location: Cursor{Line: 1, Column: 1}, Content: "#include"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 9}, Content: " "},
				{Type: TokenType_PreprocessorSystemPath, Location: Cursor{Line: 1, Column: 10}, Content: "<gtest/gtest.h>"},
			},
		},
	}

	for _, tc := range testCases {
		lx := NewLexer(tc.input)
		assert.Equal(t, tc.expected, slices.Collect(lx.AllTokens()), "input: %q", tc.input)
	}
}

func runBenchmark(b *testing.B, input []byte) {
	b.Helper()
	for b.Loop() {
		_ = slices.Collect(NewLexer(input).AllTokens())
	}
}

func BenchmarkRepeatedToken(b *testing.B) {
	runBenchmark(b, bytes.Repeat([]byte(";"), 1000))
}

const helloWorldInput = `
#include <iostream>

int main(int argc, char **argv) {
    std::cout << "Hello, World!" << std::endl;
	return 0;
}
`

func BenchmarkHelloWorld(b *testing.B) {
	runBenchmark(b, []byte(helloWorldInput))
}

func BenchmarkRepeatedHelloWorld(b *testing.B) {
	runBenchmark(b, bytes.Repeat([]byte(helloWorldInput), 100))
}
