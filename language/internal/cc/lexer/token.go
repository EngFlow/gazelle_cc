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

import "errors"

type TokenType int

const (
	// Every complete token that is not one of the other types, e.g. identifiers, keywords.
	TokenType_Word TokenType = iota

	// One of predefined fixed-size sequences of characters, e.g. '(', '==', ';', '&&'.
	TokenType_Symbol

	// Preprocessor directive, a hash '#' followed by the directive name (with optional whitespace characters between).
	TokenType_PreprocessorDirective

	// Single newline character '\n'. Newlines require special handling because they mark the end of a preprocessor directive.
	TokenType_Newline

	// One or more whitespace characters, other than newlines.
	TokenType_Whitespace

	// Line continuation sequence, a backslash '\' followed by a newline character '\n' (with optional whitespace characters between).
	TokenType_ContinueLine

	// Single-line comment, starting with // and ending at the end of the line.
	TokenType_SingleLineComment

	// Multi-line comment, starting with /* and ending with */.
	TokenType_MultiLineComment
)

var (
	ErrContinueLineInvalid          = errors.New("invalid characters after line continuation backslash")
	ErrMultiLineCommentUnterminated = errors.New("unterminated multi-line comment")
)

type Token struct {
	Type     TokenType
	Location Cursor
	Content  string
}
