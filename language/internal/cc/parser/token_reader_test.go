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
