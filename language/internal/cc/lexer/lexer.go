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

// Package lexer provides a lexical analyzer for the C/C++ source code. It breaks the input into a sequence of tokens,
// which can then be processed by a parser.
//
// Lexer classifies tokens into several types (for e.g., easier filtering comments or whitespace) and tracks their
// location in the source code (for accurate error reporting).
package lexer

import "regexp"

// Represents a way of matching a specific token type using a regular expression.
type matcher struct {
	matchedType TokenType
	matchingRe  *regexp.Regexp
}

// Matching logic for all token types apart from TokenType_Word which is the default fallback type when no other
// matchers apply.
var matchers = []matcher{
	{matchedType: TokenType_Symbol, matchingRe: regexp.MustCompile(`(?:[!=<>]=?|&&?|\|\|?|[(){}\[\],;])`)},
	{matchedType: TokenType_PreprocessorDirective, matchingRe: regexp.MustCompile(`#[\t\v\f\r ]*\w+`)},
	{matchedType: TokenType_Newline, matchingRe: regexp.MustCompile(`\n`)},
	{matchedType: TokenType_Whitespace, matchingRe: regexp.MustCompile(`[\t\v\f\r ]+`)},
	{matchedType: TokenType_ContinueLine, matchingRe: regexp.MustCompile(`\\[\t\v\f\r ]*\n`)},
	{matchedType: TokenType_SingleLineComment, matchingRe: regexp.MustCompile(`//[^\n]*`)},
	{matchedType: TokenType_MultiLineComment, matchingRe: regexp.MustCompile(`(?s)/\*.*?\*/`)},
}

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
// returns TokenEmpty.
func (lx *Lexer) NextToken() Token {
	if len(lx.dataLeft) == 0 {
		return TokenEmpty
	}

	// If none of matchers apply, the token is qualified as TokenType_Word which ends at the beginning of the next token
	// recognized by one of matchers.
	wordEnd := len(lx.dataLeft)

	for _, m := range matchers {
		// Check full match.
		if match := m.matchingRe.FindIndex(lx.dataLeft); match != nil {
			tokenBegin := match[0]
			tokenEnd := match[1]

			if tokenBegin == 0 {
				token := Token{
					Type:     m.matchedType,
					Location: lx.cursor,
					Content:  string(lx.dataLeft[tokenBegin:tokenEnd]),
				}
				lx.consume(token.Content)
				return token
			} else {
				wordEnd = min(wordEnd, tokenBegin)
			}
		}
	}

	// Fallback to TokenType_Word.
	token := Token{
		Type:     TokenType_Word,
		Location: lx.cursor,
		Content:  string(lx.dataLeft[:wordEnd]),
	}
	lx.consume(token.Content)
	return token
}

// Return all tokens extracted from the input data.
func (lx *Lexer) Tokenize() []Token {
	var tokens []Token
	for len(lx.dataLeft) > 0 {
		tokens = append(tokens, lx.NextToken())
	}
	return tokens
}
