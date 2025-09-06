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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func (ch chunk) String() string {
	return fmt.Sprintf("chunk{data: %q, complete: %v}", ch.data, ch.complete)
}

func TestPrequalifyToken(t *testing.T) {
	testCases := []struct {
		input    chunk
		expected TokenType
	}{
		{
			input:    chunk{data: []byte("")},
			expected: TokenType_Incomplete,
		},
		{
			input:    chunk{data: []byte("#")},
			expected: TokenType_Word,
		},
		{
			input:    chunk{data: []byte("\n")},
			expected: TokenType_Newline,
		},
		{
			input:    chunk{data: []byte(" ")},
			expected: TokenType_Whitespace,
		},
		{
			input:    chunk{data: []byte("\\")},
			expected: TokenType_ContinueLine,
		},
		{
			input:    chunk{data: []byte("// single line comment")},
			expected: TokenType_SingleLineComment,
		},
		{
			input:    chunk{data: []byte("/* multi line comment")},
			expected: TokenType_MultiLineComment,
		},
		{
			input:    chunk{data: []byte("/"), complete: false},
			expected: TokenType_Incomplete,
		},
		{
			input:    chunk{data: []byte("/"), complete: true},
			expected: TokenType_Word,
		},
		{
			input:    chunk{data: []byte(`"string`)},
			expected: TokenType_StringLiteral,
		},
		{
			input:    chunk{data: []byte(`R"(raw string`)},
			expected: TokenType_RawStringLiteral,
		},
		{
			// 'R' could be the start of a raw string literal, or it could be a word, need more data to decide
			input:    chunk{data: []byte("R"), complete: false},
			expected: TokenType_Incomplete,
		},
		{
			input:    chunk{data: []byte("R"), complete: true},
			expected: TokenType_Word,
		},
		{
			input:    chunk{data: []byte("RR")},
			expected: TokenType_Word,
		},

		{
			input:    chunk{data: []byte("/ 5")},
			expected: TokenType_Word,
		},
		{
			input:    chunk{data: []byte("<iostream>")},
			expected: TokenType_Symbol,
		},
		{
			input:    chunk{data: []byte("int main()")},
			expected: TokenType_Word,
		},
	}

	for _, tc := range testCases {
		result := prequalifyToken(tc.input)
		assert.Equal(t, tc.expected, result, "Input: %v", tc.input)
	}
}

func TestExtractToken(t *testing.T) {
	testCases := []struct {
		input        chunk
		expectedType TokenType
		expectedOk   []byte
		expectedErr  error
	}{
		{
			input:      chunk{data: []byte("((")},
			expectedOk: []byte("("),
		},
		{
			input:      chunk{data: []byte("(")},
			expectedOk: []byte("("),
		},
		{
			input:      chunk{data: []byte("&&")},
			expectedOk: []byte("&&"),
		},
		{
			input:      chunk{data: []byte("&"), complete: true},
			expectedOk: []byte("&"),
		},
		{
			input:      chunk{data: []byte("&"), complete: false},
			expectedOk: nil,
		},
		{
			input:      chunk{data: []byte("<=")},
			expectedOk: []byte("<="),
		},
		{
			input:      chunk{data: []byte("<"), complete: true},
			expectedOk: []byte("<"),
		},
		{
			input:      chunk{data: []byte("<"), complete: false},
			expectedOk: nil,
		},
		{
			input:      chunk{data: []byte("#include \"file.h\"")},
			expectedOk: []byte("#include"),
		},
		{
			input:      chunk{data: []byte("\n\n")},
			expectedOk: []byte("\n"),
		},
		{
			input:      chunk{data: []byte("\r\n")},
			expectedOk: []byte("\r"),
		},
		{
			input:      chunk{data: []byte("\t\t abc")},
			expectedOk: []byte("\t\t "),
		},
		{
			input:      chunk{data: []byte("\t\t "), complete: true},
			expectedOk: []byte("\t\t "),
		},
		{
			input:      chunk{data: []byte("\t\t "), complete: false},
			expectedOk: nil,
		},
		{
			input:      chunk{data: []byte("\\    \nSQUARE(x) ((x)*(x))")},
			expectedOk: []byte("\\    \n"),
		},
		{
			input:       chunk{data: []byte("\\"), complete: true},
			expectedErr: ErrContinueLineInvalid,
		},
		{
			input:      chunk{data: []byte("\\"), complete: false},
			expectedOk: nil,
		},
		{
			input:      chunk{data: []byte("// This is a single line comment\nint main()"), complete: true},
			expectedOk: []byte("// This is a single line comment"),
		},
		{
			input:      chunk{data: []byte("// This is a single line comment"), complete: true},
			expectedOk: []byte("// This is a single line comment"),
		},
		{
			input:      chunk{data: []byte("// This is a single line comment"), complete: false},
			expectedOk: nil,
		},
		{
			input:      chunk{data: []byte("/*\n  This is a multi line comment\n*/\nint main()"), complete: true},
			expectedOk: []byte("/*\n  This is a multi line comment\n*/"),
		},
		{
			input:       chunk{data: []byte("/*\n  This is a multi line comment"), complete: true},
			expectedErr: ErrMultiLineCommentUnterminated,
		},
		{
			input:      chunk{data: []byte("/*\n  This is a multi line comment"), complete: false},
			expectedOk: nil,
		},
		{
			input:      chunk{data: []byte(`""`), complete: true},
			expectedOk: []byte(`""`),
		},
		{
			input:      chunk{data: []byte(`"\""`), complete: true},
			expectedOk: []byte(`"\""`),
		},
		{
			input:      chunk{data: []byte(`"This is a string literal"`)},
			expectedOk: []byte(`"This is a string literal"`),
		},
		{
			input:      chunk{data: []byte(`"This is a string with an escaped quote: \" inside"`)},
			expectedOk: []byte(`"This is a string with an escaped quote: \" inside"`),
		},
		{
			input:       chunk{data: []byte(`"unterminated string literal`), complete: true},
			expectedErr: ErrStringLiteralUnterminated,
		},
		{
			input:      chunk{data: []byte(`"unterminated string literal`), complete: false},
			expectedOk: nil,
		},
		{
			input:       chunk{data: []byte("\"newline in string literal\n\"")},
			expectedErr: ErrStringLiteralUnterminated,
		},
		{
			input:      chunk{data: []byte(`"Escaped backslash \\"; "different string"`)},
			expectedOk: []byte(`"Escaped backslash \\"`),
		},
		{
			input:      chunk{data: []byte(`R"()"`)},
			expectedOk: []byte(`R"()"`),
		},
		{
			input:      chunk{data: []byte(`R"(abc)" fake-end)"`)},
			expectedOk: []byte(`R"(abc)"`),
		},
		{
			input:      chunk{data: []byte(`R"delim(This is a raw "(string)" with a custom delimiter)delim"`)},
			expectedOk: []byte(`R"delim(This is a raw "(string)" with a custom delimiter)delim"`),
		},
		{
			input:       chunk{data: []byte(`R"(unterminated raw string literal: missing parenthesis`), complete: true},
			expectedErr: ErrRawStringLiteralUnterminated,
		},
		{
			input:      chunk{data: []byte(`R"(unterminated raw string literal: missing parenthesis`), complete: false},
			expectedOk: nil,
		},
		{
			input:       chunk{data: []byte(`R"delim(unterminated raw string literal: missing quote)`), complete: true},
			expectedErr: ErrRawStringLiteralUnterminated,
		},
		{
			input:      chunk{data: []byte(`R"delim(unterminated raw string literal: missing quote)`), complete: false},
			expectedOk: nil,
		},
		{
			input:       chunk{data: []byte(`R"unterminated raw string literal: missing parenthesis"`), complete: true},
			expectedErr: ErrRawStringLiteralMissingOpeningDelimiter,
		},
		{
			input:      chunk{data: []byte(`R"unterminated raw string literal: missing parenthesis"`), complete: false},
			expectedOk: nil,
		},
		{
			input:      chunk{data: []byte("identifier123;"), complete: true},
			expectedOk: []byte("identifier123"),
		},
		{
			input:      chunk{data: []byte("identifier123"), complete: true},
			expectedOk: []byte("identifier123"),
		},
		{
			input:      chunk{data: []byte("identifier123"), complete: false},
			expectedOk: nil,
		},
		{
			input:      chunk{data: []byte("IDENTIFIER"), complete: true},
			expectedOk: []byte("IDENTIFIER"),
		},
		{
			input:      chunk{data: []byte("IDENTIFIER"), complete: false},
			expectedOk: nil,
		},
	}

	for _, tc := range testCases {
		result, err := extractToken(tc.input)
		assert.Equal(t, tc.expectedOk, result, "Input: %v", tc.input)
		assert.ErrorIs(t, tc.expectedErr, err, "Input: %v", tc.input)
	}
}
