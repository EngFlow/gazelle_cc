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
	"bytes"
	"iter"
	"regexp"
	"strings"
)

var (
	reContinueLine           = regexp.MustCompile(`^\\[\t\v\f\r ]*\n`)
	rePreprocessorSystemPath = regexp.MustCompile(`^<[\w-+./]+>`)
	reLiteralInteger         = regexp.MustCompile(`^(?i)0x[0-9a-f]+|0b[01]+|0[0-7]*|[1-9][0-9]*`)
	reLiteralString          = regexp.MustCompile(`^"(?:[^"\\\n]|\\.)*"`)
	reIdentifier             = regexp.MustCompile(`^(?i)[a-z_][a-z0-9_]*`)
	reTokenBeginning         = regexp.MustCompile(`[\s\\"/#=><!&|{}[\],();\w]`)

	preprocessorDirectives = []struct {
		keyword   string
		tokenType TokenType
	}{
		// longer keywords listed first to ensure proper matching
		{"include_next", TokenType_PreprocessorIncludeNext},
		{"elifndef", TokenType_PreprocessorElifndef},
		{"elifdef", TokenType_PreprocessorElifdef},
		{"include", TokenType_PreprocessorInclude},
		{"define", TokenType_PreprocessorDefine},
		{"ifndef", TokenType_PreprocessorIfndef},
		{"endif", TokenType_PreprocessorEndif},
		{"ifdef", TokenType_PreprocessorIfdef},
		{"undef", TokenType_PreprocessorUndef},
		{"elif", TokenType_PreprocessorElif},
		{"else", TokenType_PreprocessorElse},
		{"if", TokenType_PreprocessorIf},
	}
)

type (
	Lexer struct {
		dataLeft []byte
		cursor   Cursor
	}
	lexeme struct {
		tokenType TokenType
		length    int
	}
)

func NewLexer(sourceCode []byte) *Lexer {
	return &Lexer{dataLeft: sourceCode, cursor: CursorInit}
}

// Find the index of the first non-whitespace character in the data slice.
// Returns len(data) if all characters are whitespace.
func findNonWhitespace(data []byte) int {
	for i, b := range data {
		if !strings.ContainsAny(string(b), " \t\v\f\r") {
			return i
		}
	}
	return len(data)
}

// Update the lexer state accordingly to the extracted token content.
func (lx *Lexer) consume(lxm lexeme) Token {
	token := Token{
		Type:     lxm.tokenType,
		Location: lx.cursor,
		Content:  string(lx.dataLeft[:lxm.length]),
	}
	lx.dataLeft = lx.dataLeft[lxm.length:]
	lx.cursor = lx.cursor.AdvancedBy(token.Content)
	return token
}

// Return the next token extracted from the beginning of the input data left to
// process. If no more tokens are left, returns TokenEOF.
func (lx *Lexer) NextToken() Token {
	if len(lx.dataLeft) == 0 {
		return TokenEOF
	}

	lxm := lexeme{tokenType: TokenType_Unassigned, length: len(lx.dataLeft)}

	switch lx.dataLeft[0] {
	case '\n':
		lxm = lexeme{tokenType: TokenType_Newline, length: 1}
	case '\t', '\v', '\f', '\r', ' ':
		lxm = lexeme{tokenType: TokenType_Whitespace, length: findNonWhitespace(lx.dataLeft)}
	case '\\':
		if match := reContinueLine.Find(lx.dataLeft); match != nil {
			lxm = lexeme{tokenType: TokenType_ContinueLine, length: len(match)}
		}
	case '"':
		if match := reLiteralString.Find(lx.dataLeft); match != nil {
			lxm = lexeme{tokenType: TokenType_LiteralString, length: len(match)}
		}
	case '/':
		if bytes.HasPrefix(lx.dataLeft, []byte("//")) {
			end := bytes.IndexByte(lx.dataLeft, '\n')
			if end == -1 {
				end = len(lx.dataLeft)
			}
			lxm = lexeme{tokenType: TokenType_CommentSingleLine, length: end}
		} else if bytes.HasPrefix(lx.dataLeft, []byte("/*")) {
			if end := bytes.Index(lx.dataLeft, []byte("*/")); end >= 0 {
				lxm = lexeme{tokenType: TokenType_CommentMultiLine, length: end + 2}
			}
		}
	case '#':
		begin := findNonWhitespace(lx.dataLeft[1:]) + 1
		for _, directive := range preprocessorDirectives {
			if bytes.HasPrefix(lx.dataLeft[begin:], []byte(directive.keyword)) {
				lxm = lexeme{tokenType: directive.tokenType, length: begin + len(directive.keyword)}
				break
			}
		}
	case '=':
		if strings.HasPrefix(string(lx.dataLeft), "==") {
			lxm = lexeme{tokenType: TokenType_OperatorEqual, length: 2}
		}
	case '>':
		if bytes.HasPrefix(lx.dataLeft, []byte(">=")) {
			lxm = lexeme{tokenType: TokenType_OperatorGreaterOrEqual, length: 2}
		} else {
			lxm = lexeme{tokenType: TokenType_OperatorGreater, length: 1}
		}
	case '<':
		if match := rePreprocessorSystemPath.Find(lx.dataLeft); match != nil {
			lxm = lexeme{tokenType: TokenType_PreprocessorSystemPath, length: len(match)}
		} else if bytes.HasPrefix(lx.dataLeft, []byte("<=")) {
			lxm = lexeme{tokenType: TokenType_OperatorLessOrEqual, length: 2}
		} else {
			lxm = lexeme{tokenType: TokenType_OperatorLess, length: 1}
		}
	case '!':
		if bytes.HasPrefix(lx.dataLeft, []byte("!=")) {
			lxm = lexeme{tokenType: TokenType_OperatorNotEqual, length: 2}
		} else {
			lxm = lexeme{tokenType: TokenType_OperatorLogicalNot, length: 1}
		}
	case '&':
		if bytes.HasPrefix(lx.dataLeft, []byte("&&")) {
			lxm = lexeme{tokenType: TokenType_OperatorLogicalAnd, length: 2}
		}
	case '|':
		if bytes.HasPrefix(lx.dataLeft, []byte("||")) {
			lxm = lexeme{tokenType: TokenType_OperatorLogicalOr, length: 2}
		}
	case '{':
		lxm = lexeme{tokenType: TokenType_BraceLeft, length: 1}
	case '}':
		lxm = lexeme{tokenType: TokenType_BraceRight, length: 1}
	case '[':
		lxm = lexeme{tokenType: TokenType_BracketLeft, length: 1}
	case ']':
		lxm = lexeme{tokenType: TokenType_BracketRight, length: 1}
	case ',':
		lxm = lexeme{tokenType: TokenType_Comma, length: 1}
	case '(':
		lxm = lexeme{tokenType: TokenType_ParenthesisLeft, length: 1}
	case ')':
		lxm = lexeme{tokenType: TokenType_ParenthesisRight, length: 1}
	case ';':
		lxm = lexeme{tokenType: TokenType_Semicolon, length: 1}
	default:
		if match := reIdentifier.Find(lx.dataLeft); match != nil {
			if string(match) == "defined" {
				lxm = lexeme{tokenType: TokenType_PreprocessorDefined, length: len(match)}
			} else {
				lxm = lexeme{tokenType: TokenType_Identifier, length: len(match)}
			}
		} else if match := reLiteralInteger.Find(lx.dataLeft); match != nil {
			lxm = lexeme{tokenType: TokenType_LiteralInteger, length: len(match)}
		}
	}

	if lxm.tokenType == TokenType_Unassigned {
		// scan forward to some well-understood characters
		if begin := reTokenBeginning.FindIndex(lx.dataLeft[1:]); begin != nil {
			lxm.length = 1 + begin[0]
		}
	}

	return lx.consume(lxm)
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
