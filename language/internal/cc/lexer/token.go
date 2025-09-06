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

type TokenType int

const (
	TokenType_Incomplete        TokenType = iota // Token is too short to unambiguously determine its type.
	TokenType_Symbol                             // One of predefined fixed-size sequences of characters, e.g. '(', '==', ';', '&&'.
	TokenType_Newline                            // Single newline character '\n'.
	TokenType_Whitespace                         // One or more whitespace characters, other than newlines.
	TokenType_ContinueLine                       // Line continuation sequence, a backslash '\' followed by a newline character '\n'.
	TokenType_SingleLineComment                  // Single-line comment, starting with // and ending at the end of the line.
	TokenType_MultiLineComment                   // Multi-line comment, starting with /* and ending with */.
	TokenType_StringLiteral                      // String literal, starting and ending with ". May contain escape sequences.
	TokenType_RawStringLiteral                   // Raw string literal, starting with R"delimiter( and ending with )delimiter". May contain any characters except the closing sequence.
	TokenType_Word                               // Every complete token that is not one of the other types, e.g. identifiers, keywords.
)

type TokenTypeSet int

func NewTokenTypeSet(types ...TokenType) (ts TokenTypeSet) {
	for _, t := range types {
		ts |= (1 << t)
	}
	return
}

func (ts TokenTypeSet) Contains(t TokenType) bool {
	return ts&(1<<t) != 0
}

type Token struct {
	Type     TokenType
	Location Cursor
	Content  string
}
