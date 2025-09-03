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

package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func readAllTokens(reader *tokenReader) []string {
	var tokens []string
	for {
		token, ok := reader.next()
		if !ok {
			break
		}
		tokens = append(tokens, token)
	}
	return tokens
}

func TestGetTokens(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{
			input:    "int main() { return 0; }",
			expected: []string{"int", "main", "(", ")", "{", "return", "0;", "}"},
		},
		{
			input:    "/* comment */ int a = 5;",
			expected: []string{"int", "a", "=", "5;"},
		},
		{
			input:    "// single line comment\nint b = 10;",
			expected: []string{"<EOL>", "int", "b", "=", "10;"},
		},
		{
			input:    "/* multi\nline\ncomment */ int c = 15;",
			expected: []string{"int", "c", "=", "15;"},
		},
	}

	for _, tc := range testCases {
		reader := newTokenReader(strings.NewReader(tc.input))
		actualTokens := readAllTokens(reader)
		assert.Equal(t, tc.expected, actualTokens, "Input:%v", tc.input)
	}
}

type tokenWithLineNumber struct {
	token      string
	lineNumber int
}

func readAllTokensWithLineNumbers(reader *tokenReader) []tokenWithLineNumber {
	var tokens []tokenWithLineNumber
	for {
		token, ok := reader.next()
		if !ok {
			break
		}
		tokens = append(tokens, tokenWithLineNumber{token: token, lineNumber: reader.lineNumber})
	}
	return tokens
}

func TestLineNumberTracking(t *testing.T) {
	testCases := []struct {
		input    string
		expected []tokenWithLineNumber
	}{
		{
			input: "#include <fmt/core.h>\n#include \"mylib.h\"",
			expected: []tokenWithLineNumber{
				{token: "#include", lineNumber: 1},
				{token: "<", lineNumber: 1},
				{token: "fmt/core.h", lineNumber: 1},
				{token: ">", lineNumber: 1},
				{token: "<EOL>", lineNumber: 2},
				{token: "#include", lineNumber: 2},
				{token: "\"mylib.h\"", lineNumber: 2},
			},
		},
		{
			input: `#include <iostream>

					/*
						a multiline comment
					*/
					#include <fmt/core.h>

					int main() {
						return 0;
					}`,
			expected: []tokenWithLineNumber{
				{token: "#include", lineNumber: 1},
				{token: "<", lineNumber: 1},
				{token: "iostream", lineNumber: 1},
				{token: ">", lineNumber: 1},
				{token: "<EOL>", lineNumber: 2},
				{token: "<EOL>", lineNumber: 3},
				{token: "<EOL>", lineNumber: 6},
				{token: "#include", lineNumber: 6},
				{token: "<", lineNumber: 6},
				{token: "fmt/core.h", lineNumber: 6},
				{token: ">", lineNumber: 6},
				{token: "<EOL>", lineNumber: 7},
				{token: "<EOL>", lineNumber: 8},
				{token: "int", lineNumber: 8},
				{token: "main", lineNumber: 8},
				{token: "(", lineNumber: 8},
				{token: ")", lineNumber: 8},
				{token: "{", lineNumber: 8},
				{token: "<EOL>", lineNumber: 9},
				{token: "return", lineNumber: 9},
				{token: "0;", lineNumber: 9},
				{token: "<EOL>", lineNumber: 10},
				{token: "}", lineNumber: 10},
			},
		},
	}

	for _, tc := range testCases {
		reader := newTokenReader(strings.NewReader(tc.input))
		actualTokens := readAllTokensWithLineNumbers(reader)
		assert.Equal(t, tc.expected, actualTokens, "Input:%v", tc.input)
	}
}
