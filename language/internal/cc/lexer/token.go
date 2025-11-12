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
	// Special token type indicating the end of the input stream (or default
	// value when an error is returned).
	TokenType_EOF TokenType = iota

	// Every complete token that is not one of the other types.
	//
	// This is a fallback type. Lexer covers only a subset of C/C++ syntax.
	// Every token without its dedicated TokenType is classified as Unassigned.
	TokenType_Unassigned

	// Single newline character '\n'. Newlines require special handling because
	// they mark the end of a preprocessor directive.
	TokenType_Newline

	// One or more whitespace characters, other than newlines.
	TokenType_Whitespace

	// Line continuation sequence, a backslash '\' followed by a newline
	// character '\n' (with optional whitespace characters between).
	TokenType_ContinueLine

	// Preprocessor system include path, enclosed in angle brackets, e.g.
	// <stdio.h>.
	TokenType_PreprocessorSystemPath

	// The special keyword "defined", used in preprocessor conditional
	// expressions.
	TokenType_PreprocessorDefined

	// Identifier or keyword, a letter or underscore followed by letters, digits
	// or underscores.
	TokenType_Identifier

	// Integer literal in base decimal, hexadecimal, octal or binary, e.g. 123,
	// 0x1A3F, 0755, 0b1101.
	TokenType_LiteralInteger

	// String literal, enclosed in double quotes, e.g. "example".
	TokenType_LiteralString

	// Single-line comment, starting with // and ending at the end of the line.
	TokenType_CommentSingleLine

	// Multi-line comment, starting with /* and ending with */.
	TokenType_CommentMultiLine

	// Preprocessor directives, a hash '#' followed by the directive name (with
	// optional whitespace characters between).

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

// TODO: we can generate appropriate string representations using tools like
// https://github.com/dmarkham/enumer
func (t TokenType) String() string {
	switch t {
	case TokenType_EOF:
		return "end of file"
	case TokenType_Newline:
		return "newline"
	case TokenType_Whitespace:
		return "whitespace"
	case TokenType_ContinueLine:
		return `line continuation backslash '\'`
	case TokenType_PreprocessorSystemPath:
		return "<system_include_path>"
	case TokenType_PreprocessorDefined:
		return "keyword 'defined'"
	case TokenType_Identifier:
		return "identifier"
	case TokenType_LiteralInteger:
		return "integer literal"
	case TokenType_LiteralString:
		return `"string literal"`
	case TokenType_CommentSingleLine:
		return "single-line comment"
	case TokenType_CommentMultiLine:
		return "multi-line comment"
	case TokenType_PreprocessorDefine:
		return "directive '#define'"
	case TokenType_PreprocessorElif:
		return "directive '#elif'"
	case TokenType_PreprocessorElifdef:
		return "directive '#elifdef'"
	case TokenType_PreprocessorElifndef:
		return "directive '#elifndef'"
	case TokenType_PreprocessorElse:
		return "directive '#else'"
	case TokenType_PreprocessorEndif:
		return "directive '#endif'"
	case TokenType_PreprocessorIf:
		return "directive '#if'"
	case TokenType_PreprocessorIfdef:
		return "directive '#ifdef'"
	case TokenType_PreprocessorIfndef:
		return "directive '#ifndef'"
	case TokenType_PreprocessorInclude:
		return "directive '#include'"
	case TokenType_PreprocessorIncludeNext:
		return "directive '#include_next'"
	case TokenType_PreprocessorUndef:
		return "directive '#undef'"
	case TokenType_OperatorEqual:
		return "operator '=='"
	case TokenType_OperatorGreater:
		return "operator '>'"
	case TokenType_OperatorGreaterOrEqual:
		return "operator '>='"
	case TokenType_OperatorLess:
		return "operator '<'"
	case TokenType_OperatorLessOrEqual:
		return "operator '<='"
	case TokenType_OperatorLogicalAnd:
		return "operator '&&'"
	case TokenType_OperatorLogicalNot:
		return "operator '!'"
	case TokenType_OperatorLogicalOr:
		return "operator '||'"
	case TokenType_OperatorNotEqual:
		return "operator '!='"
	case TokenType_BraceLeft:
		return "symbol '{'"
	case TokenType_BraceRight:
		return "symbol '}'"
	case TokenType_BracketLeft:
		return "symbol '['"
	case TokenType_BracketRight:
		return "symbol ']'"
	case TokenType_Comma:
		return "symbol ','"
	case TokenType_ParenthesisLeft:
		return "symbol '('"
	case TokenType_ParenthesisRight:
		return "symbol ')'"
	case TokenType_Semicolon:
		return "symbol ';'"
	default:
		return "unknown token"
	}
}

func (t TokenType) IsPreprocessorDirective() bool {
	return t >= TokenType_PreprocessorDefine && t <= TokenType_PreprocessorUndef
}

type Token struct {
	Type     TokenType
	Location Cursor
	Content  string
}

var TokenEOF = Token{Type: TokenType_EOF}
