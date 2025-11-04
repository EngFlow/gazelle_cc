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

type (
	// Result of matching a specific token type against the input data. Results
	// are comparable to prioritize which token to extract next.
	matchingResult struct {
		matchedType TokenType
		beginIndex  int
		endIndex    int
	}

	Lexer struct {
		// Read-only full source code to be tokenized.
		sourceCode []byte

		// Current byte offset in sourceCode where the next token extraction
		// should begin.
		index int

		// Current cursor position in the source code, synchronized with index.
		cursor Cursor

		// Partial matching results, at most one per TokenType. Once a specific
		// TokenType is consumed, a new matchingResult for it is generated (as
		// long as there are more occurrences of that TokenType in the remaining
		// input).
		partialResults *collections.PriorityQueue[matchingResult]
	}
)

func (r matchingResult) Less(other matchingResult) bool {
	if r.beginIndex != other.beginIndex {
		// Prefer earlier matches.
		return r.beginIndex < other.beginIndex
	}
	if r.endIndex != other.endIndex {
		// When they start at the same position, prefer longer matches.
		return r.endIndex > other.endIndex
	}

	// When they match the same range, prefer lower TokenType values. E.g.
	// "defined" matches both PreprocessorDefined and Identifier, so we want to
	// prefer PreprocessorDefined.
	return r.matchedType < other.matchedType
}

func match(tokenType TokenType, content []byte, offset int) *matchingResult {
	if matched := matchingRules[tokenType].FindIndex(content[offset:]); matched != nil {
		return &matchingResult{
			matchedType: tokenType,
			beginIndex:  offset + matched[0],
			endIndex:    offset + matched[1],
		}
	}
	return nil
}

func initPartialResults(sourceCode []byte) *collections.PriorityQueue[matchingResult] {
	initResults := make([]matchingResult, 0, len(matchingRules))
	for tokenType := range matchingRules {
		if matched := match(tokenType, sourceCode, 0); matched != nil {
			initResults = append(initResults, *matched)
		}
	}
	return collections.NewPriorityQueue(initResults)
}

func NewLexer(sourceCode []byte) *Lexer {
	return &Lexer{
		sourceCode:     sourceCode,
		cursor:         CursorInit,
		partialResults: initPartialResults(sourceCode),
	}
}

// Update the lexer state accordingly to the extracted token content.
func (lx *Lexer) consume(content string) {
	lx.index += len(content)
	lx.cursor = lx.cursor.AdvancedBy(content)
}

func (lx *Lexer) shouldUpdatePartialResults() bool {
	return !lx.partialResults.Empty() && lx.partialResults.Peek().beginIndex < lx.index
}

func (lx *Lexer) updateEarliestPartialResult() {
	consumedType := lx.partialResults.Pop().matchedType
	if matched := match(consumedType, lx.sourceCode, lx.index); matched != nil {
		lx.partialResults.Push(*matched)
	}
}

// Ensure that partialResults contains only matches that start not earlier than
// lx.index.
func (lx *Lexer) updatePartialResults() {
	for lx.shouldUpdatePartialResults() {
		lx.updateEarliestPartialResult()
	}
}

// Return the next token extracted from the beginning of the input data left to
// process. If no more tokens are left, returns TokenEOF.
func (lx *Lexer) NextToken() Token {
	if lx.index == len(lx.sourceCode) {
		return TokenEOF
	}

	lx.updatePartialResults()

	tokenBegin := lx.index
	tokenEnd := len(lx.sourceCode)
	tokenType := TokenType_Unassigned

	if !lx.partialResults.Empty() {
		if earliest := lx.partialResults.Peek(); earliest.beginIndex == lx.index {
			// Something matched at the beginning, so return that token.
			tokenBegin = earliest.beginIndex
			tokenEnd = earliest.endIndex
			tokenType = earliest.matchedType
		} else {
			// If nothing matched at the beginning of the text, return an
			// unassigned token up to the next match. If nothing matched
			// anywhere, use the rest of the text.
			tokenEnd = earliest.beginIndex
		}
	}

	result := Token{Type: tokenType, Location: lx.cursor, Content: string(lx.sourceCode[tokenBegin:tokenEnd])}
	lx.consume(result.Content)
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
