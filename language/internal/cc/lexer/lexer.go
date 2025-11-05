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

// Package lexer provides a lexical analyzer for the C/C++ source code. It
// breaks the input into a sequence of tokens, which can then be processed by a
// parser.
//
// Lexer classifies tokens into several types (for e.g., easier filtering
// comments or whitespace) and tracks their location in the source code (for
// accurate error reporting).
package lexer

import (
	"iter"

	"github.com/EngFlow/gazelle_cc/internal/collections"
)

type Lexer struct {
	// Read-only full source code to be tokenized.
	sourceCode []byte

	// Current byte offset in sourceCode where the next token extraction should
	// begin.
	index int

	// Current cursor position in the source code, synchronized with index.
	cursor Cursor

	// Partial matching results, at most one per TokenType, closest to the
	// current index but not before it. Once a specific TokenType is consumed, a
	// new matchingResult for it is generated (as long as there are more
	// occurrences of that TokenType in the remaining input).
	earliestMatches *collections.PriorityQueue[matchingResult]
}

func initEarliestMatches(sourceCode []byte) *collections.PriorityQueue[matchingResult] {
	initResults := collections.FilterMapSlice(matchingRules, func(rule matchingRule) (matchingResult, bool) {
		return rule.match(sourceCode, 0)
	})
	return collections.NewPriorityQueue(initResults)
}

func NewLexer(sourceCode []byte) *Lexer {
	return &Lexer{
		sourceCode:      sourceCode,
		cursor:          CursorInit,
		earliestMatches: initEarliestMatches(sourceCode),
	}
}

// Update the lexer state accordingly to the extracted token content.
func (lx *Lexer) consume(content string) {
	lx.index += len(content)
	lx.cursor = lx.cursor.AdvancedBy(content)
}

func (lx *Lexer) shouldUpdateEarliestMatches() bool {
	return !lx.earliestMatches.Empty() && lx.earliestMatches.Peek().beginIndex < lx.index
}

func (lx *Lexer) updateConsumedEarliestMatch() {
	if match, ok := lx.earliestMatches.Pop().rule.match(lx.sourceCode, lx.index); ok {
		lx.earliestMatches.Push(match)
	}
}

// Ensure that earliestMatches contains only matches that start at or after
// lx.index. This invariant must be maintained after consuming any token.
// Sometimes more than one match is updated when potential tokens overlap. E.g.,
// a comment may contain words which are normally matched as identifiers or
// other token types.
func (lx *Lexer) updateEarliestMatches() {
	for lx.shouldUpdateEarliestMatches() {
		lx.updateConsumedEarliestMatch()
	}
}

// Return the next token extracted from the beginning of the input data left to
// process. If no more tokens are left, returns TokenEOF.
func (lx *Lexer) NextToken() Token {
	if lx.index == len(lx.sourceCode) {
		return TokenEOF
	}

	tokenBegin := lx.index
	tokenEnd := len(lx.sourceCode)
	tokenType := TokenType_Unassigned
	if !lx.earliestMatches.Empty() {
		if earliest := lx.earliestMatches.Peek(); earliest.beginIndex == lx.index {
			// Something matched at the beginning, so return that token.
			tokenBegin = earliest.beginIndex
			tokenEnd = earliest.endIndex
			tokenType = earliest.rule.matchedType
		} else {
			// If nothing matched at the beginning of the text, return an
			// unassigned token up to the next match. If nothing matched
			// anywhere, use the rest of the text.
			tokenEnd = earliest.beginIndex
		}
	}

	result := Token{Type: tokenType, Location: lx.cursor, Content: string(lx.sourceCode[tokenBegin:tokenEnd])}
	lx.consume(result.Content)
	lx.updateEarliestMatches()
	return result
}

// Iterate through the all tokens extracted from the input data.
func (lx *Lexer) AllTokens() iter.Seq[Token] {
	return func(yield func(Token) bool) {
		for lx.index < len(lx.sourceCode) {
			if !yield(lx.NextToken()) {
				return
			}
		}
	}
}
