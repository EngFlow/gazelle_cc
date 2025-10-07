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
	"fmt"
	"io"
	"regexp"
)

// Represents a way of matching a specific token type using a regular expression.
type matcher struct {
	// Type assigned to a token when this matcher matches.
	matchedType TokenType

	// Regular expression used to match the full token content.
	matchingRe *regexp.Regexp

	// Optional. If set and matches, the matcher will treat matchingRe as obligatory and none of the other matchers will
	// be considered. This is useful for reporting incomplete tokens, e.g. unterminated multi-line comments.
	checkedPrefix string

	// Optional. Error to return if the checkedPrefix is set and matches, but the full regex does not match.
	checkedPrefixFailure error
}

// Matching logic for all token types apart from TokenType_Word which is the default fallback type when no other
// matchers apply.
var matchers = []matcher{
	{
		matchedType: TokenType_Symbol,
		matchingRe:  regexp.MustCompile(`(?:[!=<>]=?|&&?|\|\|?|[(){}\[\],;])`),
	},
	{
		matchedType: TokenType_PreprocessorDirective,
		matchingRe:  regexp.MustCompile(`#[\t\v\f\r ]*\w+`),
	},
	{
		matchedType: TokenType_Newline,
		matchingRe:  regexp.MustCompile(`\n`),
	},
	{
		matchedType: TokenType_Whitespace,
		matchingRe:  regexp.MustCompile(`[\t\v\f\r ]+`),
	},
	{
		matchedType:          TokenType_ContinueLine,
		matchingRe:           regexp.MustCompile(`\\[\t\v\f\r ]*\n`),
		checkedPrefix:        `\`,
		checkedPrefixFailure: ErrContinueLineInvalid,
	},
	{
		matchedType: TokenType_SingleLineComment,
		matchingRe:  regexp.MustCompile(`//[^\n]*`),
	},
	{
		matchedType:          TokenType_MultiLineComment,
		matchingRe:           regexp.MustCompile(`(?s)/\*.*?\*/`),
		checkedPrefix:        "/*",
		checkedPrefixFailure: ErrMultiLineCommentUnterminated,
	},
}

type Lexer struct {
	dataLeft []byte
	cursor   Cursor
}

func NewLexer(sourceCode []byte) *Lexer {
	return &Lexer{
		dataLeft: sourceCode,
		cursor:   CursorInit,
	}
}

// Wrap an error with the current cursor location for better context.
func (lx *Lexer) makeError(err error) error {
	return fmt.Errorf("%v: %w", lx.cursor, err)
}

// Update the lexer state accordingly to the extracted token.
func (lx *Lexer) consume(token *Token) {
	lx.dataLeft = lx.dataLeft[len(token.Content):]
	lx.cursor = lx.cursor.AdvancedBy(token.Content)
}

// Return the next token extracted from the beginning of the input data left to process.
func (lx *Lexer) NextToken() (token Token, err error) {
	defer lx.consume(&token)

	if len(lx.dataLeft) == 0 {
		err = lx.makeError(io.EOF)
		return
	}

	// If none of matchers apply, the token is qualified as TokenType_Word which ends at the beginning of the next token
	// recognized by one of matchers.
	wordEnd := len(lx.dataLeft)

	for _, m := range matchers {
		match := m.matchingRe.FindIndex(lx.dataLeft)

		// Prefix matches but the full token does not match.
		if len(m.checkedPrefix) > 0 && bytes.HasPrefix(lx.dataLeft, []byte(m.checkedPrefix)) && (match == nil || match[0] != 0) {
			err = lx.makeError(m.checkedPrefixFailure)
			return
		}

		if match == nil {
			continue
		}

		tokenBegin := match[0]
		tokenEnd := match[1]
		if tokenBegin == 0 {
			token = Token{
				Type:     m.matchedType,
				Location: lx.cursor,
				Content:  string(lx.dataLeft[tokenBegin:tokenEnd]),
			}
			return
		}

		wordEnd = min(wordEnd, tokenBegin)
	}

	// Fallback to TokenType_Word.
	token = Token{
		Type:     TokenType_Word,
		Location: lx.cursor,
		Content:  string(lx.dataLeft[:wordEnd]),
	}
	return
}

// Return all tokens extracted from the input data. If an error occurs during tokenization, returns the tokens extracted
// so far along with the error.
func (lx *Lexer) Tokenize() ([]Token, error) {
	var tokens []Token
	for len(lx.dataLeft) > 0 {
		token, err := lx.NextToken()
		if err != nil {
			return tokens, err
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}
