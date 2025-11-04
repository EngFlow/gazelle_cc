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
	"bytes"
	"regexp"
)

type (
	// Abstraction over regexp.Regexp allows providing an alternative
	// implementation.
	matcher interface {
		// Return a two-element slice of integers defining the location of the
		// leftmost match in content of this matcher. The match itself is at
		// content[indices[0]:indices[1]]. A return value of nil indicates no
		// match.
		FindIndex(content []byte) (indices []int)
	}

	// Matcher for fixed strings. No need to use regexp.Regexp for such simple
	// cases.
	fixedStringMatcher string
)

func (fs fixedStringMatcher) FindIndex(content []byte) []int {
	if begin := bytes.Index(content, []byte(fs)); begin >= 0 {
		return []int{begin, begin + len(fs)}
	}
	return nil
}

func preprocessorMatcher(directiveName string) matcher {
	return regexp.MustCompile(`#[\t\v\f\r ]*` + directiveName)
}

// Matching logic for all token types apart from:
//   - TokenType_EOF which is returned when no input data is left to process and
//     it is never used for another purpose.
//   - TokenType_Unassigned which is the default fallback type when no other
//     matchingRule apply.
var matchingRules = map[TokenType]matcher{
	TokenType_Newline:                 fixedStringMatcher("\n"),
	TokenType_Whitespace:              regexp.MustCompile(`[\t\v\f\r ]+`),
	TokenType_ContinueLine:            regexp.MustCompile(`\\[\t\v\f\r ]*\n`),
	TokenType_PreprocessorSystemPath:  regexp.MustCompile(`<[\w-+./]+>`),
	TokenType_PreprocessorDefined:     fixedStringMatcher("defined"),
	TokenType_Identifier:              regexp.MustCompile(`(?i)[a-z_][a-z0-9_]*`),
	TokenType_LiteralInteger:          regexp.MustCompile(`(?i)0x[0-9a-f]+|0b[01]+|0[0-7]*|[1-9][0-9]*`),
	TokenType_LiteralString:           regexp.MustCompile(`"(?:[^"\\\n]|\\.)*"`),
	TokenType_CommentSingleLine:       regexp.MustCompile(`//[^\n]*`),
	TokenType_CommentMultiLine:        regexp.MustCompile(`(?s)/\*.*?\*/`),
	TokenType_PreprocessorDefine:      preprocessorMatcher("define"),
	TokenType_PreprocessorElif:        preprocessorMatcher("elif"),
	TokenType_PreprocessorElifdef:     preprocessorMatcher("elifdef"),
	TokenType_PreprocessorElifndef:    preprocessorMatcher("elifndef"),
	TokenType_PreprocessorElse:        preprocessorMatcher("else"),
	TokenType_PreprocessorEndif:       preprocessorMatcher("endif"),
	TokenType_PreprocessorIf:          preprocessorMatcher("if"),
	TokenType_PreprocessorIfdef:       preprocessorMatcher("ifdef"),
	TokenType_PreprocessorIfndef:      preprocessorMatcher("ifndef"),
	TokenType_PreprocessorInclude:     preprocessorMatcher("include"),
	TokenType_PreprocessorIncludeNext: preprocessorMatcher("include_next"),
	TokenType_PreprocessorUndef:       preprocessorMatcher("undef"),
	TokenType_OperatorEqual:           fixedStringMatcher("=="),
	TokenType_OperatorGreater:         fixedStringMatcher(">"),
	TokenType_OperatorGreaterOrEqual:  fixedStringMatcher(">="),
	TokenType_OperatorLess:            fixedStringMatcher("<"),
	TokenType_OperatorLessOrEqual:     fixedStringMatcher("<="),
	TokenType_OperatorLogicalAnd:      fixedStringMatcher("&&"),
	TokenType_OperatorLogicalNot:      fixedStringMatcher("!"),
	TokenType_OperatorLogicalOr:       fixedStringMatcher("||"),
	TokenType_OperatorNotEqual:        fixedStringMatcher("!="),
	TokenType_BraceLeft:               fixedStringMatcher("{"),
	TokenType_BraceRight:              fixedStringMatcher("}"),
	TokenType_BracketLeft:             fixedStringMatcher("["),
	TokenType_BracketRight:            fixedStringMatcher("]"),
	TokenType_Comma:                   fixedStringMatcher(","),
	TokenType_ParenthesisLeft:         fixedStringMatcher("("),
	TokenType_ParenthesisRight:        fixedStringMatcher(")"),
	TokenType_Semicolon:               fixedStringMatcher(";"),
}
