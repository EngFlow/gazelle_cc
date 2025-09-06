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
	"bytes"
	"errors"
	"fmt"
	"io"
)

var (
	ErrContinueLineInvalid                     = errors.New("missing newline character after line continuation backslash")
	ErrMultiLineCommentUnterminated            = errors.New("unterminated multi-line comment")
	ErrRawStringLiteralMissingOpeningDelimiter = errors.New("missing opening delimiter '(' in raw string literal")
	ErrRawStringLiteralUnterminated            = errors.New("unterminated raw string literal")
	ErrStringLiteralUnterminated               = errors.New("unterminated string literal")
)

type chunk struct {
	data     []byte // chunk of the data to be tokenized, may be too short to form a complete token
	complete bool   // whether there is no more data to be read after this chunk
}

// determine token type basing on the first few characters of the chunk
func prequalifyToken(ch chunk) TokenType {
	if len(ch.data) == 0 {
		return TokenType_Incomplete
	}

	switch ch.data[0] {
	case '(', ')', '[', ']', '{', '}', ',', ';', '&', '|', '<', '>', '=', '!':
		return TokenType_Symbol
	case '\n':
		return TokenType_Newline
	case '\t', '\v', '\f', '\r', ' ':
		return TokenType_Whitespace
	case '\\':
		return TokenType_ContinueLine
	case '/':
		switch {
		case bytes.HasPrefix(ch.data, []byte("//")):
			return TokenType_SingleLineComment
		case bytes.HasPrefix(ch.data, []byte("/*")):
			return TokenType_MultiLineComment
		case len(ch.data) >= 2 || ch.complete:
			return TokenType_Word
		default:
			return TokenType_Incomplete
		}
	case '"':
		return TokenType_StringLiteral
	case 'R':
		switch {
		case bytes.HasPrefix(ch.data, []byte(`R"`)):
			return TokenType_RawStringLiteral
		case len(ch.data) >= 2 || ch.complete:
			return TokenType_Word
		default:
			return TokenType_Incomplete
		}
	default:
		return TokenType_Word
	}
}

func extractSymbolToken(ch chunk) []byte {
	switch ch.data[0] {
	case '(', ')', '[', ']', '{', '}', ',', ';':
		return ch.data[:1]
	case '&', '|':
		switch {
		case len(ch.data) >= 2 && ch.data[1] == ch.data[0]:
			return ch.data[:2]
		case len(ch.data) >= 2 || ch.complete:
			return ch.data[:1]
		default:
			return nil
		}
	case '<', '>', '=', '!':
		switch {
		case len(ch.data) >= 2 && ch.data[1] == '=':
			return ch.data[:2]
		case len(ch.data) >= 2 || ch.complete:
			return ch.data[:1]
		default:
			return nil
		}
	default:
		panic(fmt.Errorf("unhandled symbol %q", ch.data[0]))
	}
}

// applicable for tokens where one character class is repeated one or more times (like in regex "[abc]+")
func extractDynamicSizedToken(ch chunk, expected TokenType) []byte {
	for i := 1; i < len(ch.data); i++ {
		lookAheadType := prequalifyToken(chunk{data: ch.data[i:], complete: ch.complete})
		if lookAheadType == TokenType_Incomplete {
			return nil
		} else if lookAheadType != expected {
			return ch.data[:i]
		}
	}

	if ch.complete {
		return ch.data
	} else {
		return nil
	}
}

func extractWordToken(ch chunk) []byte {
	return extractDynamicSizedToken(ch, TokenType_Word)
}

func extractWhitespaceToken(ch chunk) []byte {
	return extractDynamicSizedToken(ch, TokenType_Whitespace)
}

func extractContinueLineToken(ch chunk) ([]byte, error) {
	// ignore characters between the backslash and the newline; whitespace characters usually trigger warnings in compilers
	if newline := bytes.IndexByte(ch.data, '\n'); newline >= 0 {
		return ch.data[:newline+1], nil
	}

	if ch.complete {
		return nil, ErrContinueLineInvalid
	} else {
		return nil, nil
	}
}

func extractSingleLineCommentToken(ch chunk) []byte {
	if newlineIndex := bytes.IndexByte(ch.data, '\n'); newlineIndex >= 0 {
		return ch.data[:newlineIndex]
	}

	if ch.complete {
		return ch.data
	} else {
		return nil
	}
}

func extractMultiLineCommentToken(ch chunk) ([]byte, error) {
	if endIndex := bytes.Index(ch.data, []byte("*/")); endIndex >= 0 {
		return ch.data[:endIndex+2], nil
	}

	if ch.complete {
		return nil, ErrMultiLineCommentUnterminated
	} else {
		return nil, nil
	}
}

func extractStringLiteralToken(ch chunk) ([]byte, error) {
	start := 1
	for {
		relIndex := bytes.IndexByte(ch.data[start:], '"')
		if relIndex < 0 {
			if ch.complete {
				return nil, ErrStringLiteralUnterminated
			} else {
				return nil, nil
			}
		}

		absIndex := start + relIndex
		// string literal must fit in one line
		if bytes.IndexByte(ch.data[start:absIndex], '\n') >= 0 {
			return nil, ErrStringLiteralUnterminated
		}
		// check if the quote is escaped
		if ch.data[absIndex-1] != '\\' || ch.data[absIndex-2] == '\\' {
			return ch.data[:absIndex+1], nil
		}

		start = absIndex + 1
	}
}

func extractRawStringLiteralToken(ch chunk) ([]byte, error) {
	start := bytes.IndexByte(ch.data, '(')
	if start < 0 {
		if ch.complete {
			return nil, ErrRawStringLiteralMissingOpeningDelimiter
		} else {
			return nil, nil
		}
	}

	customDelimiterName := ch.data[2:start]
	endDelimiter := make([]byte, 0, len(customDelimiterName)+len(`)"`))
	endDelimiter = append(endDelimiter, ')')
	endDelimiter = append(endDelimiter, customDelimiterName...)
	endDelimiter = append(endDelimiter, '"')

	endIndex := bytes.Index(ch.data, endDelimiter)
	if endIndex < 0 {
		if ch.complete {
			return nil, ErrRawStringLiteralUnterminated
		} else {
			return nil, nil
		}
	}

	return ch.data[:endIndex+len(endDelimiter)], nil
}

func extractToken(ch chunk) ([]byte, error) {
	switch prequalifyToken(ch) {
	case TokenType_Incomplete:
		return nil, nil
	case TokenType_Symbol:
		return extractSymbolToken(ch), nil
	case TokenType_Newline:
		return ch.data[:1], nil
	case TokenType_Whitespace:
		return extractWhitespaceToken(ch), nil
	case TokenType_ContinueLine:
		return extractContinueLineToken(ch)
	case TokenType_SingleLineComment:
		return extractSingleLineCommentToken(ch), nil
	case TokenType_MultiLineComment:
		return extractMultiLineCommentToken(ch)
	case TokenType_StringLiteral:
		return extractStringLiteralToken(ch)
	case TokenType_RawStringLiteral:
		return extractRawStringLiteralToken(ch)
	case TokenType_Word:
		return extractWordToken(ch), nil
	default:
		panic(fmt.Errorf("unhandled token type %v", prequalifyToken(ch)))
	}
}

func tokenizer(data []byte, atEOF bool) (advance int, token []byte, err error) {
	token, err = extractToken(chunk{data: data, complete: atEOF})
	advance = len(token)
	return
}

func newScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Split(tokenizer)
	return scanner
}
