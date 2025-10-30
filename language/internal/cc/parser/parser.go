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

// Package parser implements a lightweight scanner / parser that extracts high-level information from a C/C++ translation unit
// without requiring a full pre-processor or compiler front-end.  It recognises:
//
//   - `#include` lines (both angle-bracket and quoted form)
//   - Conditional compilation guards formed with `#if[*]`, `#ifdef`, `#ifndef` and friends, and converts the boolean logic into an ExprAST declared in the same package.
//   - The presence of a `main()` function – useful for distinguishing executables from libraries.
//
// The parser is not a complete C/C++ pre-processor – it only understands enough of the grammar to serve the purposes of gazelle_cc and deliberately ignores tokens that are irrelevant for dependency extraction.
package parser

import (
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/EngFlow/gazelle_cc/language/internal/cc/lexer"
)

func isRelevantTokenType(token lexer.Token) bool {
	switch token.Type {
	case lexer.TokenType_Unassigned, lexer.TokenType_Whitespace, lexer.TokenType_ContinueLine, lexer.TokenType_CommentSingleLine, lexer.TokenType_CommentMultiLine:
		return false
	default:
		return true
	}
}

// ParseSource reads and parses C/C++ source, returning structured SourceInfo.
func ParseSource(input []byte) (SourceInfo, error) {
	allTokens := lexer.NewLexer(input).AllTokens()
	filteredTokens := collections.FilterSeq(allTokens, isRelevantTokenType)
	p := parser{tokensLeft: slices.Collect(filteredTokens)}
	directives, err := p.parseDirectivesUntil(func(tokenType lexer.TokenType) bool { return tokenType == lexer.TokenType_EOF })
	p.sourceInfo.Directives = directives

	return p.sourceInfo, err
}

// ParseSourceFile opens filename and feeds its contents to the extractor.
func ParseSourceFile(filename string) (SourceInfo, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return SourceInfo{}, err
	}
	return ParseSource(content)
}

type (
	parseRule struct {
		precedence   precedence
		prefixParser prefixParseFn
		infixParser  infixParserFn
	}
	prefixParseFn func(p *parser, operator lexer.TokenType) (Expr, error)
	infixParserFn func(p *parser, operator lexer.TokenType, lhs Expr) (Expr, error)
	precedence    int
)

const (
	precedenceLowest  precedence = iota
	precedenceOr                 // ||
	precedenceAnd                // &&
	precedenceCompare            // ==, !=, <, <=, >, >=
	precedenceBang               // ! (prefix)
	precedenceParens             // (
)

// exprKeywordsPrecedence maps operator tokens to their precedence and parser
// functions. This is initialized in init() to avoid cyclic reference errors at
// package init time.
var exprKeywordsPrecedence map[lexer.TokenType]parseRule

func init() {
	exprKeywordsPrecedence = map[lexer.TokenType]parseRule{
		lexer.TokenType_OperatorLogicalNot:     {precedence: precedenceBang, prefixParser: parseUnaryBangOperator},
		lexer.TokenType_ParenthesisLeft:        {precedence: precedenceParens, prefixParser: parseUnaryOpenParenthesis, infixParser: parseBinaryApplyOperator},
		lexer.TokenType_PreprocessorDefined:    {precedence: precedenceLowest, prefixParser: parseDefinedExpr},
		lexer.TokenType_OperatorLogicalOr:      {precedence: precedenceOr, infixParser: parseBinaryLogicOrOperator},
		lexer.TokenType_OperatorLogicalAnd:     {precedence: precedenceAnd, infixParser: parseBinaryLogicAndOperator},
		lexer.TokenType_OperatorEqual:          {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
		lexer.TokenType_OperatorNotEqual:       {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
		lexer.TokenType_OperatorGreater:        {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
		lexer.TokenType_OperatorGreaterOrEqual: {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
		lexer.TokenType_OperatorLess:           {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
		lexer.TokenType_OperatorLessOrEqual:    {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
	}
}

// switch to enable logging of errors found when parsing sources
// used only for development purpuses, we don't log log errors in normal mode
const debug = false

// parseExprPrecedence implements Pratt parsing for expressions, handling C
// preprocessor conditionals. minPrecedence controls operator binding
// (precedence climbing).
func (p *parser) parseExprPrecedence(minPrecedence precedence) (Expr, error) {
	token := p.nextToken()
	var result Expr
	var err error
	rule, exists := exprKeywordsPrecedence[token.Type]
	if exists && rule.prefixParser != nil {
		result, err = rule.prefixParser(p, token.Type)
	} else {
		result, err = parseValue(token)
	}

	if err != nil {
		return nil, err
	}

	for {
		rule, exists := exprKeywordsPrecedence[p.peekToken()]
		if !exists || rule.precedence < minPrecedence {
			return result, nil // current operator binds less – stop and return
		}

		result, err = rule.infixParser(p, p.nextToken().Type, result)
		if err != nil {
			return nil, err
		}
	}
}

func parseBinaryLogicOrOperator(p *parser, _ lexer.TokenType, lhs Expr) (Expr, error) {
	rhs, err := p.parseExprPrecedence(precedenceOr + 1)
	if err != nil {
		return nil, err
	}
	return Or{L: lhs, R: rhs}, nil
}

func parseBinaryLogicAndOperator(p *parser, _ lexer.TokenType, lhs Expr) (Expr, error) {
	rhs, err := p.parseExprPrecedence(precedenceAnd + 1)
	if err != nil {
		return nil, err
	}
	return And{L: lhs, R: rhs}, nil
}

func parseBinaryCompareOperator(p *parser, operator lexer.TokenType, lhs Expr) (Expr, error) {
	rhs, err := p.parseExprPrecedence(precedenceCompare + 1)
	if err != nil {
		return nil, err
	}
	return Compare{Left: lhs, Op: operator, Right: rhs}, nil
}

func parseBinaryApplyOperator(p *parser, _ lexer.TokenType, lhs Expr) (Expr, error) {
	ident, ok := lhs.(Ident)
	if !ok {
		return nil, fmt.Errorf("expected identifier for apply operator, got %T", lhs)
	}

	args := []Expr{}
	for {
		switch p.peekToken() {
		case lexer.TokenType_EOF, lexer.TokenType_Newline:
			return nil, fmt.Errorf("unexpected end of input while parsing apply operator %q", ident)
		case lexer.TokenType_Comma:
			p.nextToken()
			continue
		case lexer.TokenType_ParenthesisRight:
			p.nextToken()
			return Apply{Name: ident, Args: args}, nil
		default:
			arg, err := p.parseExprPrecedence(precedenceLowest)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
	}
}

func parseUnaryBangOperator(p *parser, _ lexer.TokenType) (Expr, error) {
	inner, err := p.parseExprPrecedence(precedenceBang + 1)
	if err != nil {
		return nil, err
	}
	return Not{X: inner}, nil
}

func parseUnaryOpenParenthesis(p *parser, _ lexer.TokenType) (Expr, error) {
	expr, err := p.parseExprPrecedence(precedenceLowest + 1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expectNextToken(lexer.TokenType_ParenthesisRight); err != nil {
		return nil, err
	}
	return expr, nil
}

// parseIncludeDirective parses an #include or #include_next directive,
// extracting its path and kind (system/user).
func (p *parser) parseIncludeDirective() (Directive, error) {
	switch p.peekToken() {
	// Handle #include <system_include.h>
	case lexer.TokenType_PreprocessorSystemPath:
		pathToken := p.nextToken()
		path := strings.TrimSuffix(strings.TrimPrefix(pathToken.Content, "<"), ">")
		return IncludeDirective{Path: path, IsSystem: true, LineNumber: pathToken.Location.Line}, nil
	// Handle #include "local_include.h"
	case lexer.TokenType_LiteralString:
		pathToken := p.nextToken()
		path := strings.Trim(pathToken.Content, `"`)
		return IncludeDirective{Path: path, IsSystem: false, LineNumber: pathToken.Location.Line}, nil
	default:
		return nil, errors.New("malformed include directive path")
	}
}

// parseDefinedExpr parses the `defined` operator for macro checks in #if
// expressions.
func parseDefinedExpr(p *parser, _ lexer.TokenType) (Expr, error) {
	var name Ident
	var err error
	if p.peekToken() == lexer.TokenType_ParenthesisLeft {
		p.nextToken()
		name, err = p.parseIdent()
		if err != nil {
			return nil, err
		}
		if _, err := p.expectNextToken(lexer.TokenType_ParenthesisRight); err != nil {
			return nil, err
		}
	} else {
		name, err = p.parseIdent()
		if err != nil {
			return nil, err
		}
	}
	return Defined{Name: name}, nil
}

type parser struct {
	tokensLeft []lexer.Token // Tokens yet to be processed
	sourceInfo SourceInfo    // Accumulated parser state
}

// Drop n tokens from the front of the input stream.
func (p *parser) dropTokens(n int) {
	p.tokensLeft = p.tokensLeft[n:]
}

// Return the next token type without consuming it, or TokenType_EOF if no
// tokens are left.
func (p *parser) peekToken() lexer.TokenType {
	if len(p.tokensLeft) == 0 {
		return lexer.TokenType_EOF
	}
	return p.tokensLeft[0].Type
}

// Return the next token and consume it, or TokenEmpty if no tokens are left.
func (p *parser) nextToken() lexer.Token {
	if len(p.tokensLeft) == 0 {
		return lexer.TokenEOF
	}

	token := p.tokensLeft[0]
	p.dropTokens(1)
	return token
}

// Return the next token and consume it if it matches expected type. Otherwise
// return an error, without consuming the token.
func (p *parser) expectNextToken(expected lexer.TokenType) (lexer.Token, error) {
	switch p.peekToken() {
	case expected:
		return p.nextToken(), nil
	case lexer.TokenType_EOF:
		return lexer.TokenEOF, fmt.Errorf("expected %s but reached end of input", expected)
	default:
		return lexer.TokenEOF, fmt.Errorf("expected %s but found %s", expected, p.peekToken())
	}
}

// parseDirectivesUntil reads tokens and parses directives until shouldStop
// returns true. It handles main(), #include, and preprocessor blocks, and
// builds the nested directive structure.
func (p *parser) parseDirectivesUntil(shouldStop func(token lexer.TokenType) bool) ([]Directive, error) {
	directives := []Directive{}
	for !shouldStop(p.peekToken()) {
		if p.tryParseMainFunction() {
			p.sourceInfo.HasMain = true
		}

		if tokenType := p.nextToken().Type; tokenType.IsPreprocessorDirective() {
			directive, err := p.parseDirective(tokenType)
			if err == nil {
				directives = append(directives, directive)
			} else {
				skipped := p.readUntilEOL() // skip remaining part of directive
				if debug {
					log.Printf("Failed to parse %v directive: %v, skipping tokens until end of line: %v", tokenType, err, skipped)
				}
			}
		}
	}

	return directives, nil
}

// parseExpr parses a preprocessor expression (#if/#elif condition) as an Expr
// AST.
func (p *parser) parseExpr() (Expr, error) {
	return p.parseExprPrecedence(precedenceLowest)
}

// readUntilEOL skips all tokens until the end of the line, returning all read
// tokens as a slice of strings.
func (p *parser) readUntilEOL() []string {
	newlineIndex := slices.IndexFunc(p.tokensLeft, func(token lexer.Token) bool { return token.Type == lexer.TokenType_Newline })
	dropIndex := newlineIndex + 1
	if newlineIndex < 0 {
		// no newline found, read until end of input
		newlineIndex = len(p.tokensLeft)
		dropIndex = len(p.tokensLeft)
	}

	result := collections.MapSlice(p.tokensLeft[:newlineIndex], func(token lexer.Token) string { return token.Content })
	p.dropTokens(dropIndex)
	return result
}

// parseIdent reads the next identifier token.
func (p *parser) parseIdent() (Ident, error) {
	token, err := p.expectNextToken(lexer.TokenType_Identifier)
	if err != nil {
		return "", err
	}
	return Ident(token.Content), nil
}

// isEndOfIfBranch checks if a token marks the end or transition of a #if block
// branch.
func isEndOfIfBranch(tokenType lexer.TokenType) bool {
	switch tokenType {
	case lexer.TokenType_PreprocessorElif, lexer.TokenType_PreprocessorElifdef, lexer.TokenType_PreprocessorElifndef, lexer.TokenType_PreprocessorElse, lexer.TokenType_PreprocessorEndif:
		return true
	default:
		return false
	}
}

// parseIfBranch parses a single #if/#ifdef/#ifndef/#elif/#elifdef/#elifndef
// branch and its body.
func (p *parser) parseIfBranch(directive lexer.TokenType, kind BranchKind) (ConditionalBranch, error) {
	var cond Expr
	var err error

	switch directive {
	case lexer.TokenType_PreprocessorIfdef, lexer.TokenType_PreprocessorElifdef:
		ident, err := p.parseIdent()
		if err != nil {
			return ConditionalBranch{}, err
		}
		cond = Defined{Name: ident}
	case lexer.TokenType_PreprocessorIfndef, lexer.TokenType_PreprocessorElifndef:
		ident, err := p.parseIdent()
		if err != nil {
			return ConditionalBranch{}, err
		}
		cond = Not{X: Defined{Name: ident}}
	case lexer.TokenType_PreprocessorIf, lexer.TokenType_PreprocessorElif:
		cond, err = p.parseExpr()
		if err != nil {
			return ConditionalBranch{}, err
		}
	default:
		return ConditionalBranch{}, fmt.Errorf("unsupported branch directive: %q", directive)
	}

	body, err := p.parseDirectivesUntil(isEndOfIfBranch)
	if err != nil {
		return ConditionalBranch{}, err
	}

	return ConditionalBranch{
		Kind:      kind,
		Condition: cond,
		Body:      body,
	}, nil
}

// parseIfBlock parses an entire #if/#ifdef/#ifndef block (including
// #elif/#else/#endif) and all nested directives.
func (p *parser) parseIfBlock(startDirective lexer.TokenType) (IfBlock, error) {
	var branches []ConditionalBranch

	firstBranch, err := p.parseIfBranch(startDirective, IfBranch)
	if err != nil {
		return IfBlock{}, err
	}
	branches = append(branches, firstBranch)

	for {
		tokenType := p.peekToken()
		switch tokenType {
		case lexer.TokenType_PreprocessorElif, lexer.TokenType_PreprocessorElifdef, lexer.TokenType_PreprocessorElifndef:
			p.nextToken()
			branch, err := p.parseIfBranch(tokenType, ElifBranch)
			if err != nil {
				return IfBlock{}, err
			}
			branches = append(branches, branch)

		case lexer.TokenType_PreprocessorElse:
			p.nextToken()
			body, err := p.parseDirectivesUntil(func(tokenType lexer.TokenType) bool { return tokenType == lexer.TokenType_PreprocessorEndif })
			if err != nil {
				return IfBlock{}, err
			}
			branches = append(branches, ConditionalBranch{
				Kind:      ElseBranch,
				Condition: nil,
				Body:      body,
			})

		case lexer.TokenType_PreprocessorEndif:
			p.nextToken()
			return IfBlock{Branches: branches}, nil

		default:
			return IfBlock{}, fmt.Errorf("unexpected token %v inside #if block", tokenType)
		}
	}
}

// parseDefineDirective parses a #define directive, capturing the macro name and
// tokens.
func (p *parser) parseDefineDirective() (DefineDirective, error) {
	ident, err := p.parseIdent()
	if err != nil {
		return DefineDirective{}, err
	}
	defineArgs := []string{}
	if p.peekToken() == lexer.TokenType_ParenthesisLeft {
		p.nextToken()
		// Function-like macro definition
	parseArgs:
		for {
			switch p.peekToken() {
			case lexer.TokenType_ParenthesisRight:
				// end of argument list
				p.nextToken()
				break parseArgs
			case lexer.TokenType_Comma:
				// skip commas
				p.nextToken()
				continue
			case lexer.TokenType_Identifier:
				// argument name
				defineArgs = append(defineArgs, p.nextToken().Content)
			default:
				return DefineDirective{}, fmt.Errorf("malformed macro argument list in #define for macro %q", ident)
			}
		}
	}
	return DefineDirective{Name: ident.String(), Args: defineArgs, Body: p.readUntilEOL()}, nil
}

// parseUndefineDirective parses a #undef directive and its macro name.
func (p *parser) parseUndefineDirective() (UndefineDirective, error) {
	ident, err := p.parseIdent()
	if err != nil {
		return UndefineDirective{}, err
	}
	return UndefineDirective{Name: ident.String()}, nil
}

// parseDirective dispatches to the appropriate directive parser based on the
// token.
func (p *parser) parseDirective(directive lexer.TokenType) (Directive, error) {
	switch directive {
	case lexer.TokenType_PreprocessorInclude, lexer.TokenType_PreprocessorIncludeNext:
		return p.parseIncludeDirective()
	case lexer.TokenType_PreprocessorIf, lexer.TokenType_PreprocessorIfdef, lexer.TokenType_PreprocessorIfndef:
		return p.parseIfBlock(directive)
	case lexer.TokenType_PreprocessorDefine:
		return p.parseDefineDirective()
	case lexer.TokenType_PreprocessorUndef:
		return p.parseUndefineDirective()
	default:
		if isEndOfIfBranch(directive) {
			return nil, fmt.Errorf("malformed input: unpaired #if condition token: %q", directive)
		}
		return nil, fmt.Errorf("unknown directive: %q", directive)
	}
}

func (p *parser) tryParseMainFunction() bool {
	if len(p.tokensLeft) >= 3 && p.tokensLeft[0].Content == "int" && p.tokensLeft[1].Content == "main" && p.tokensLeft[2].Content == "(" {
		p.dropTokens(3)
		return true
	}
	return false
}

// A valid macro identifier must follow these rules:
// * First character must be ‘_’ or a letter.
// * Subsequent characters may be ‘_’, letters, or decimal digits.
var macroIdentifierRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// parseValue parses a token as an identifier or integer literal, for use in
// #if/#elif expressions.
func parseValue(token lexer.Token) (Value, error) {
	switch token.Type {
	case lexer.TokenType_LiteralInteger:
		if v, err := parseIntLiteral(token.Content); err == nil {
			return ConstantInt(v), nil
		}
	case lexer.TokenType_Identifier:
		return Ident(token.Content), nil
	}
	return nil, fmt.Errorf("token %q is neither identifier nor integer literal", token.Content)
}

// parseIntLiteral parses an integer literal in decimal, octal, or hex form,
// ignoring C suffixes.
func parseIntLiteral(tok string) (int, error) {
	v, err := strconv.ParseInt(tok, 0, 32)
	return int(v), err
}
