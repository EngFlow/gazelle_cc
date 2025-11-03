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

package lexer_v1

import "iter"

type Lexer struct {
	dataLeft []byte
	cursor   Cursor
}

func NewLexer(sourceCode []byte) *Lexer {
	return &Lexer{dataLeft: sourceCode, cursor: CursorInit}
}

// Update the lexer state accordingly to the extracted token content.
func (lx *Lexer) consume(content string) {
	lx.dataLeft = lx.dataLeft[len(content):]
	lx.cursor = lx.cursor.AdvancedBy(content)
}

// Return the next token extracted from the beginning of the input data left to process. If no more tokens are left,
// returns TokenEOF.
func (lx *Lexer) NextToken() Token {
	if len(lx.dataLeft) == 0 {
		return TokenEOF
	}

	// Try each matchingRule looking for the earliest match.
	tokenBegin := len(lx.dataLeft)
	tokenEnd := len(lx.dataLeft)
	tokenType := TokenType_Unassigned
	for _, rule := range matchingRules {
		match := rule.matchingImpl.FindIndex(lx.dataLeft)
		if match != nil && (match[0] < tokenBegin || ( /* prefer longer matches */ match[0] == tokenBegin && match[1] > tokenEnd)) {
			tokenBegin = match[0]
			tokenEnd = match[1]
			tokenType = rule.matchedType
		}
	}

	var result Token
	if tokenBegin == 0 {
		// Something matched at the beginning, so return that token.
		result = Token{Type: tokenType, Location: lx.cursor, Content: string(lx.dataLeft[tokenBegin:tokenEnd])}
	} else {
		// If nothing matched at the beginning of the text, return an unassigned token up to the next match. If nothing
		// matched anywhere, use the rest of the text.
		result = Token{Type: TokenType_Unassigned, Location: lx.cursor, Content: string(lx.dataLeft[:tokenBegin])}
	}

	lx.consume(result.Content)
	return result
}

// Iterate through the all tokens extracted from the input data.
func (lx *Lexer) AllTokens() iter.Seq[Token] {
	return func(yield func(Token) bool) {
		for len(lx.dataLeft) > 0 {
			if !yield(lx.NextToken()) {
				return
			}
		}
	}
}
