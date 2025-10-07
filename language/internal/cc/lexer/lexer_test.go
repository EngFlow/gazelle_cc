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
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextToken(t *testing.T) {
	testCases := []struct {
		input           []byte
		expectedType    TokenType
		expectedContent string
		expectedError   error
	}{
		{
			input:         []byte(""),
			expectedError: io.EOF,
		},
		{
			input:           []byte("&&"),
			expectedType:    TokenType_Symbol,
			expectedContent: "&&",
		},
		{
			input:           []byte("#include \"file.h\""),
			expectedType:    TokenType_PreprocessorDirective,
			expectedContent: "#include",
		},
		{
			input:           []byte("#   define VARIABLE 123"),
			expectedType:    TokenType_PreprocessorDirective,
			expectedContent: "#   define",
		},
		{
			input:           []byte("\n\n"),
			expectedType:    TokenType_Newline,
			expectedContent: "\n",
		},
		{
			input:           []byte("\t\t abc"),
			expectedType:    TokenType_Whitespace,
			expectedContent: "\t\t ",
		},
		{
			input:           []byte("\\\n MACRO_CONTINUED"),
			expectedType:    TokenType_ContinueLine,
			expectedContent: "\\\n",
		},
		{
			input:           []byte("\\    \n MACRO_CONTINUED"),
			expectedType:    TokenType_ContinueLine,
			expectedContent: "\\    \n",
		},
		{
			input:         []byte("\\ unexpected \n MACRO_CONTINUED"),
			expectedError: ErrContinueLineInvalid,
		},
		{
			input:         []byte("\\ unexpected \n MACRO_CONTINUED \\\n MACRO_CONTINUED_AGAIN"),
			expectedError: ErrContinueLineInvalid,
		},
		{
			input:           []byte("// This is a single line comment"),
			expectedType:    TokenType_SingleLineComment,
			expectedContent: "// This is a single line comment",
		},
		{
			input:           []byte("// This is a single line comment\nint main()"),
			expectedType:    TokenType_SingleLineComment,
			expectedContent: "// This is a single line comment",
		},
		{
			input:           []byte("/*\n  This is a multi line comment\n*/\nint main()"),
			expectedType:    TokenType_MultiLineComment,
			expectedContent: "/*\n  This is a multi line comment\n*/",
		},
		{
			// TODO handle string literals as whole tokens
			input:           []byte(`"This is a string literal"`),
			expectedType:    TokenType_Word,
			expectedContent: `"This`,
		},
		{
			// TODO handle raw string literals as whole tokens
			input:           []byte(`R"(abc)" fake-end)"`),
			expectedType:    TokenType_Word,
			expectedContent: `R"`,
		},
		{
			input:           []byte("identifier123;"),
			expectedType:    TokenType_Word,
			expectedContent: "identifier123",
		},
	}

	for _, tc := range testCases {
		lx := NewLexer(tc.input)
		token, err := lx.NextToken()
		assert.Equal(t, tc.expectedType, token.Type, "unexpected type for input: %q", tc.input)
		assert.Equal(t, tc.expectedContent, token.Content, "unexpected content for input: %q", tc.input)
		assert.ErrorIs(t, err, tc.expectedError, "unexpected error for input: %q", tc.input)
	}
}

func TestTokenize(t *testing.T) {
	testCases := []struct {
		input          []byte
		expectedTokens []Token
		expectedError  error
	}{
		{
			input:          []byte(""),
			expectedTokens: nil,
			expectedError:  nil,
		},
		{
			input: []byte("int main() { return 0; }"),
			expectedTokens: []Token{
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
			expectedError: nil,
		},
		{
			input: []byte("/*\nint main() { return 0; }\n*/\nint main() { return 0; }"),
			expectedTokens: []Token{
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
			expectedError: nil,
		},
		{
			input: []byte("#define SQUARE(x)\\\n((x)*(x))"),
			expectedTokens: []Token{
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
			expectedError: nil,
		},
		{
			input: []byte("int main() { /*\n return 0; }"),
			expectedTokens: []Token{
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 1}, Content: "int"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 4}, Content: " "},
				{Type: TokenType_Word, Location: Cursor{Line: 1, Column: 5}, Content: "main"},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 9}, Content: "("},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 10}, Content: ")"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 11}, Content: " "},
				{Type: TokenType_Symbol, Location: Cursor{Line: 1, Column: 12}, Content: "{"},
				{Type: TokenType_Whitespace, Location: Cursor{Line: 1, Column: 13}, Content: " "},
			},
			expectedError: ErrMultiLineCommentUnterminated,
		},
	}

	for _, tc := range testCases {
		lx := NewLexer(tc.input)
		tokens, err := lx.Tokenize()
		assert.Equal(t, tc.expectedTokens, tokens, "unexpected tokens for input: %q", tc.input)
		assert.ErrorIs(t, err, tc.expectedError, "unexpected error for input: %q", tc.input)
	}
}
