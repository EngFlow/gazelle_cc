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
	"bufio"
	"fmt"
	"io"
	"iter"
)

type Lexer interface {
	// Reads the next token from the input. Returns false when there are no more tokens, either by reaching the end of the input or an error.
	Read() (Token, bool)

	// First non-EOF error that was encountered by the Lexer.
	Err() error
}

// Returns an iterator that yields the tokens in order.
func AllTokens(l Lexer) iter.Seq[Token] {
	return func(yield func(Token) bool) {
		for {
			token, ok := l.Read()
			if !ok || !yield(token) {
				return
			}
		}
	}
}

type lexer struct {
	scanner *bufio.Scanner
	cursor  Cursor
}

func NewLexer(r io.Reader) Lexer {
	return &lexer{
		scanner: newScanner(r),
		cursor:  CursorInit,
	}
}

func (l *lexer) Read() (Token, bool) {
	if !l.scanner.Scan() {
		return Token{}, false
	}

	content := l.scanner.Bytes()
	token := Token{
		Type:     prequalifyToken(chunk{data: content, complete: true}),
		Location: l.cursor,
		Content:  string(content),
	}

	if token.Type == TokenType_Incomplete {
		panic(fmt.Errorf("internal error: lexer produced incomplete token at %v: %q", l.cursor, content))
	}

	l.cursor = l.cursor.AdvanceBy(string(content))
	return token, true
}

func (l *lexer) Err() error {
	if err := l.scanner.Err(); err != nil {
		return fmt.Errorf("%v: %w", l.cursor, l.scanner.Err())
	} else {
		return nil
	}
}

type filteredLexer struct {
	inner     Lexer
	allowList TokenTypeSet
}

func NewFilteredLexer(inner Lexer, allowList TokenTypeSet) Lexer {
	return &filteredLexer{
		inner:     inner,
		allowList: allowList,
	}
}

func (l *filteredLexer) Read() (Token, bool) {
	for token := range AllTokens(l.inner) {
		if l.allowList.Contains(token.Type) {
			return token, true
		}
	}

	return Token{}, false
}

func (l *filteredLexer) Err() error {
	return l.inner.Err()
}

type BufferedLexer struct {
	inner     Lexer
	lookAhead *Token
}

func NewBufferedLexer(inner Lexer) *BufferedLexer {
	return &BufferedLexer{
		inner:     inner,
		lookAhead: nil,
	}
}

func (l *BufferedLexer) Read() (Token, bool) {
	if l.lookAhead != nil {
		token := *l.lookAhead
		l.lookAhead = nil
		return token, true
	}

	return l.inner.Read()
}

func (l *BufferedLexer) Peek() (Token, bool) {
	if l.lookAhead != nil {
		return *l.lookAhead, true
	}

	token, ok := l.inner.Read()
	if !ok {
		return Token{}, false
	}

	l.lookAhead = &token
	return token, true
}

// Returns true if the next token is exactly 'expected'.
func (l *BufferedLexer) LookAheadIs(expected string) bool {
	token, ok := l.Peek()
	return ok && token.Content == expected
}

// Reads the next token and checks it matches 'expected', returning error otherwise.
func (l *BufferedLexer) Consume(expected string) error {
	token, ok := l.Read()
	if !ok {
		if l.Err() != nil {
			return fmt.Errorf("%v: expected '%v' but encountered error: %w", token.Location, expected, l.Err())
		}
		return fmt.Errorf("%v: expected '%v' but reached end of input", token.Location, expected)
	}
	if token.Content != expected {
		return fmt.Errorf("%v: expected '%v' but found '%v'", token.Location, expected, token.Content)
	}
	return nil
}

// MustConsume is like Consume but panics on error (use for parser-internal invariants).
func (l *BufferedLexer) MustConsume(expected string) {
	if err := l.Consume(expected); err != nil {
		panic(err)
	}
}

func (l *BufferedLexer) Err() error {
	return l.inner.Err()
}
