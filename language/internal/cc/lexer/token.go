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

type TokenType int

const (
	// Special token type indicating the end of the input stream.
	TokenType_EOF TokenType = iota

	// Every complete token that is not one of the other types, e.g. identifiers, keywords.
	TokenType_Word

	// Single newline character '\n'. Newlines require special handling because they mark the end of a preprocessor directive.
	TokenType_Newline

	// One or more whitespace characters, other than newlines.
	TokenType_Whitespace

	// Line continuation sequence, a backslash '\' followed by a newline character '\n' (with optional whitespace characters between).
	TokenType_ContinueLine

	// Identifier or keyword, a letter or underscore followed by letters, digits or underscores.
	TokenType_Identifier

	// Integer literal in base decimal, hexadecimal, octal or binary, e.g. 123, 0x1A3F, 0755, 0b1101.
	TokenType_LiteralInteger

	// String literal, enclosed in double quotes, e.g. "example".
	TokenType_LiteralString

	// Single-line comment, starting with // and ending at the end of the line.
	TokenType_CommentSingleLine

	// Multi-line comment, starting with /* and ending with */.
	TokenType_CommentMultiLine

	// Preprocessor directives, a hash '#' followed by the directive name (with optional whitespace characters between).

	TokenType_PreprocessorDefine
	TokenType_PreprocessorElif
	TokenType_PreprocessorElifdef
	TokenType_PreprocessorElifndef
	TokenType_PreprocessorElse
	TokenType_PreprocessorEndif
	TokenType_PreprocessorIf
	TokenType_PreprocessorIfdef
	TokenType_PreprocessorIfndef
	TokenType_PreprocessorInclude
	TokenType_PreprocessorIncludeNext
	TokenType_PreprocessorUndef

	// Subset of expression operators.

	TokenType_OperatorEqual
	TokenType_OperatorGreater
	TokenType_OperatorGreaterOrEqual
	TokenType_OperatorLess
	TokenType_OperatorLessOrEqual
	TokenType_OperatorLogicalAnd
	TokenType_OperatorLogicalNot
	TokenType_OperatorLogicalOr
	TokenType_OperatorNotEqual

	// Subset of symbols separating subexpressions.

	TokenType_BraceLeft
	TokenType_BraceRight
	TokenType_BracketLeft
	TokenType_BracketRight
	TokenType_Comma
	TokenType_ParenthesisLeft
	TokenType_ParenthesisRight
	TokenType_Semicolon
)

type Token struct {
	Type     TokenType
	Location Cursor
	Content  string
}

var TokenEOF = Token{Type: TokenType_EOF}
