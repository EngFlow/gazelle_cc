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

	// Represents a way of matching a specific token type.
	matchingRule struct {
		matchedType  TokenType
		matchingImpl matcher
	}

	// Result of matching a specific token type against the input data. Results
	// are comparable to prioritize which token to extract next.
	matchingResult struct {
		rule       matchingRule
		beginIndex int
		endIndex   int
	}
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

// Finds the leftmost match of the given token type in content starting at
// offset. If found, returns the matchingResult and true. Otherwise, returns
// false.
func (r matchingRule) match(content []byte, offset int) (matchingResult, bool) {
	if match := r.matchingImpl.FindIndex(content[offset:]); match != nil {
		return matchingResult{rule: r, beginIndex: match[0] + offset, endIndex: match[1] + offset}, true
	}
	return matchingResult{}, false
}

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
	return r.rule.matchedType < other.rule.matchedType
}

// Matching logic for all token types apart from:
//   - TokenType_EOF which is returned when no input data is left to process and
//     it is never used for another purpose.
//   - TokenType_Unassigned which is the default fallback type when no other
//     matchingRule apply.
var matchingRules = []matchingRule{
	{matchedType: TokenType_Newline, matchingImpl: fixedStringMatcher("\n")},
	{matchedType: TokenType_Whitespace, matchingImpl: regexp.MustCompile(`[\t\v\f\r ]+`)},
	{matchedType: TokenType_ContinueLine, matchingImpl: regexp.MustCompile(`\\[\t\v\f\r ]*\n`)},
	{matchedType: TokenType_PreprocessorSystemPath, matchingImpl: regexp.MustCompile(`<[\w-+./]+>`)},
	{matchedType: TokenType_PreprocessorDefined, matchingImpl: fixedStringMatcher("defined")},
	{matchedType: TokenType_Identifier, matchingImpl: regexp.MustCompile(`(?i)[a-z_][a-z0-9_]*`)},
	{matchedType: TokenType_LiteralInteger, matchingImpl: regexp.MustCompile(`(?i)0x[0-9a-f]+|0b[01]+|0[0-7]*|[1-9][0-9]*`)},
	{matchedType: TokenType_LiteralString, matchingImpl: regexp.MustCompile(`"(?:[^"\\\n]|\\.)*"`)},
	{matchedType: TokenType_CommentSingleLine, matchingImpl: regexp.MustCompile(`//[^\n]*`)},
	{matchedType: TokenType_CommentMultiLine, matchingImpl: regexp.MustCompile(`(?s)/\*.*?\*/`)},
	{matchedType: TokenType_PreprocessorDefine, matchingImpl: preprocessorMatcher("define")},
	{matchedType: TokenType_PreprocessorElif, matchingImpl: preprocessorMatcher("elif")},
	{matchedType: TokenType_PreprocessorElifdef, matchingImpl: preprocessorMatcher("elifdef")},
	{matchedType: TokenType_PreprocessorElifndef, matchingImpl: preprocessorMatcher("elifndef")},
	{matchedType: TokenType_PreprocessorElse, matchingImpl: preprocessorMatcher("else")},
	{matchedType: TokenType_PreprocessorEndif, matchingImpl: preprocessorMatcher("endif")},
	{matchedType: TokenType_PreprocessorIf, matchingImpl: preprocessorMatcher("if")},
	{matchedType: TokenType_PreprocessorIfdef, matchingImpl: preprocessorMatcher("ifdef")},
	{matchedType: TokenType_PreprocessorIfndef, matchingImpl: preprocessorMatcher("ifndef")},
	{matchedType: TokenType_PreprocessorInclude, matchingImpl: preprocessorMatcher("include")},
	{matchedType: TokenType_PreprocessorIncludeNext, matchingImpl: preprocessorMatcher("include_next")},
	{matchedType: TokenType_PreprocessorUndef, matchingImpl: preprocessorMatcher("undef")},
	{matchedType: TokenType_OperatorEqual, matchingImpl: fixedStringMatcher("==")},
	{matchedType: TokenType_OperatorGreater, matchingImpl: fixedStringMatcher(">")},
	{matchedType: TokenType_OperatorGreaterOrEqual, matchingImpl: fixedStringMatcher(">=")},
	{matchedType: TokenType_OperatorLess, matchingImpl: fixedStringMatcher("<")},
	{matchedType: TokenType_OperatorLessOrEqual, matchingImpl: fixedStringMatcher("<=")},
	{matchedType: TokenType_OperatorLogicalAnd, matchingImpl: fixedStringMatcher("&&")},
	{matchedType: TokenType_OperatorLogicalNot, matchingImpl: fixedStringMatcher("!")},
	{matchedType: TokenType_OperatorLogicalOr, matchingImpl: fixedStringMatcher("||")},
	{matchedType: TokenType_OperatorNotEqual, matchingImpl: fixedStringMatcher("!=")},
	{matchedType: TokenType_BraceLeft, matchingImpl: fixedStringMatcher("{")},
	{matchedType: TokenType_BraceRight, matchingImpl: fixedStringMatcher("}")},
	{matchedType: TokenType_BracketLeft, matchingImpl: fixedStringMatcher("[")},
	{matchedType: TokenType_BracketRight, matchingImpl: fixedStringMatcher("]")},
	{matchedType: TokenType_Comma, matchingImpl: fixedStringMatcher(",")},
	{matchedType: TokenType_ParenthesisLeft, matchingImpl: fixedStringMatcher("(")},
	{matchedType: TokenType_ParenthesisRight, matchingImpl: fixedStringMatcher(")")},
	{matchedType: TokenType_Semicolon, matchingImpl: fixedStringMatcher(";")},
}
