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

import (
	"bytes"
	"regexp"
)

type (
	// Abstraction over regexp.Regexp allows providing an alternative implementation.
	matcher interface {
		// Return a two-element slice of integers defining the location of the leftmost match in content of this
		// matcher. The match itself is at content[indices[0]:indices[1]]. A return value of nil indicates no match.
		FindIndex(content []byte) (indices []int)
	}

	// Implementation of matcher for fixed strings. No need to use regexp.Regexp for such simple cases.
	fixedString string

	// Represents a way of matching a specific token type.
	matchingRule struct {
		matchedType  TokenType
		matchingImpl matcher
	}

	// Lexer breaks the input C/C++ source code into a sequence of tokens.
	Lexer struct {
		dataLeft []byte
		cursor   Cursor
	}
)

func (fs fixedString) FindIndex(content []byte) []int {
	if begin := bytes.Index(content, []byte(fs)); begin >= 0 {
		return []int{begin, begin + len(fs)}
	}
	return nil
}

// Matching logic for all token types apart from:
// - TokenType_Word which is the default fallback type when no other matchingRule apply.
// - TokenType_EOF which is returned when no input data is left to process and it is never used for another purpose.
var matchingRules = []matchingRule{
	{matchedType: TokenType_Symbol, matchingImpl: regexp.MustCompile(`(?:[!=<>]=?|&&?|\|\|?|[(){}\[\],;])`)},
	{matchedType: TokenType_PreprocessorDirective, matchingImpl: regexp.MustCompile(`#[\t\v\f\r ]*\w+`)},
	{matchedType: TokenType_Newline, matchingImpl: fixedString("\n")},
	{matchedType: TokenType_Whitespace, matchingImpl: regexp.MustCompile(`[\t\v\f\r ]+`)},
	{matchedType: TokenType_ContinueLine, matchingImpl: regexp.MustCompile(`\\[\t\v\f\r ]*\n`)},
	{matchedType: TokenType_SingleLineComment, matchingImpl: regexp.MustCompile(`//[^\n]*`)},
	{matchedType: TokenType_MultiLineComment, matchingImpl: regexp.MustCompile(`(?s)/\*.*?\*/`)},
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
	tokenType := TokenType_Word
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
		// If nothing matched at the beginning of the text, return a word token up to the next match. If nothing matched
		// anywhere, use the rest of the text.
		result = Token{Type: TokenType_Word, Location: lx.cursor, Content: string(lx.dataLeft[:tokenBegin])}
	}

	lx.consume(result.Content)
	return result
}

// Return all tokens extracted from the input data.
func (lx *Lexer) Tokenize() []Token {
	var tokens []Token
	for len(lx.dataLeft) > 0 {
		tokens = append(tokens, lx.NextToken())
	}
	return tokens
}
