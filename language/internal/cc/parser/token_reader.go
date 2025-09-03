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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"unicode"
)

func isParenthesis(char rune) bool {
	switch char {
	case '(', ')', '[', ']', '{', '}':
		return true
	default:
		return false
	}
}

func isEOL(char byte) bool { return char == '\n' }

func isComma(char byte) bool { return char == ',' }

const EOL = "<EOL>"

// bufio.SplitFunc that skips both whitespaces, line comments (//...) and block comments (/*...*/)
// The tokenizer splits not only by whitespace seperated words but also by: parenthesis, curly/square brackets
func tokenizer(data []byte, atEOF bool) (advance int, token []byte, err error) {
	i := 0
	for i < len(data) {
		char := data[i]
		switch {
		case isEOL(char):
			return i + 1, []byte(EOL), nil
		// Skip line comments
		case bytes.HasPrefix(data[i:], []byte("//")):
			i += 2
			for i < len(data) && !isEOL(data[i]) {
				i++
			}
		// Skip block comments
		case bytes.HasPrefix(data[i:], []byte("/*")):
			i += 2
			for i < len(data)-1 {
				if bytes.HasPrefix(data[i:], []byte("*/")) {
					i += 2
					break
				}
				i++
			}
		// Skip whitespace
		case unicode.IsSpace(rune(char)):
			i++

		case isParenthesis(rune(char)) || isComma(char):
			return i + 1, data[i : i+1], nil

		case char == '!' || char == '=' || char == '<' || char == '>':
			// two-character operator?
			if i+1 < len(data) && data[i+1] == '=' {
				return i + 2, data[i : i+2], nil //  "==", "!=", "<=", ">="
			}
			return i + 1, data[i : i+1], nil // "!", "<", ">"

		default:
			start := i
			for i < len(data) {
				char := rune(data[i])
				if isEOL(data[i]) ||
					char == '!' || char == '=' || char == '<' || char == '>' ||
					unicode.IsSpace(char) || isParenthesis(char) || isComma(data[i]) {
					return i, data[start:i], nil
				}
				i++
			}
			return i, data[start:i], nil
		}
	}

	if atEOF {
		return len(data), nil, io.EOF
	}
	return i, nil, nil
}

// Thin wrapper around bufio.Scanner that provides `peek` and `next“ primitives while automatically skipping the ubiquitous newline marker except when explicitly requested.
// When an algorithm needs to honour line boundaries (e.g. parseExpr) it calls nextInternal/peekInternal instead.
type tokenReader struct {
	scanner   *bufio.Scanner
	buf       *string // one‑token look‑ahead; nil when empty
	lastToken string  // previously read token; nil when empty
	atEOF     bool    // has reader reached the EOF
}

// newTokenReader constructs a tokenReader using the provided reader and our tokenizer.
func newTokenReader(r io.Reader) *tokenReader {
	sc := bufio.NewScanner(r)
	sc.Split(tokenizer)
	return &tokenReader{scanner: sc}
}

// next returns the next token, skipping EOL markers by default.
func (tr *tokenReader) next() (string, bool) { return tr.nextInternal(true, false) }

// peek returns the next token without consuming it, skipping EOL markers by default.
func (tr *tokenReader) peek() (string, bool) { return tr.peekInternal(true, false) }

// lookAheadIs returns true if the next token is exactly 'expected'.
func (tr *tokenReader) lookAheadIs(expected string) bool {
	got, defined := tr.peek()
	return defined && got == expected
}

// consume reads the next token and checks it matches 'expected', returning error otherwise.
func (tr *tokenReader) consume(expected string) error {
	got, defined := tr.next()
	if !defined {
		return fmt.Errorf("expected '%v' but reached end of input", expected)
	}
	if got != expected {
		return fmt.Errorf("expected '%v' but found '%v'", expected, got)
	}
	return nil
}

// mustConsume is like consume but panics on error (use for parser-internal invariants).
func (tr *tokenReader) mustConsume(expected string) {
	if err := tr.consume(expected); err != nil {
		panic(err)
	}
}

// fetch retrieves the next raw token from the scanner (or from the lookahead buffer).
func (tr *tokenReader) fetch() (string, bool) {
	if tr.buf != nil {
		tok := *tr.buf
		tr.buf = nil
		return tok, true
	}
	if !tr.scanner.Scan() {
		tr.atEOF = true
		return "", false
	}
	return tr.scanner.Text(), true
}

// nextInternal reads and consumes the next token, with options to keep EOLs or line-continuation backslashes.
func (tr *tokenReader) nextInternal(keepEOL bool, keepEndlineSlash bool) (string, bool) {
	for {
		tok, ok := tr.fetch()
		if !ok {
			return "", false
		}
		if !keepEOL && tok == EOL {
			continue // skip
		}
		if !keepEndlineSlash && tok == "\\" {
			next, ok := tr.peekInternal(true, true)
			if ok && next == EOL {
				tr.consume(EOL)
				continue // skip
			}
		}
		tr.lastToken = tok
		return tok, true
	}
}

// returns the next token but does not consume the input, optionally filtering out EOL markers. The bool flag indicates if data was available
func (tr *tokenReader) peekInternal(keepEOL bool, skipEndlineSlash bool) (string, bool) {
	if tr.buf != nil {
		if !keepEOL && *tr.buf == EOL {
			return tr.next() // ensure skip semantics
		}
		return *tr.buf, true
	}
	tok, ok := tr.nextInternal(keepEOL, skipEndlineSlash)
	if !ok {
		return "", false
	}
	tr.buf = &tok
	return tok, true
}
