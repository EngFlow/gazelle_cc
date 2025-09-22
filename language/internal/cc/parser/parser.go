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
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	ts "github.com/tree-sitter/go-tree-sitter"
	ts_cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
)

// ParseSource runs the extractor on an in‑memory buffer.
func ParseSource(input string) (SourceInfo, error) {
	return parse(strings.NewReader(input))
}

// ParseSourceFile opens `filename“ and feeds its contents to the extractor.
func ParseSourceFile(filename string) (SourceInfo, error) {
	file, err := os.Open(filename)
	if err != nil {
		return SourceInfo{}, err
	}
	defer file.Close()

	return parse(file)
}

type (
	parseRule struct {
		precedence   precedence
		prefixParser prefixParseFn
		infixParser  infixParserFn
	}
	prefixParseFn func(p *parser, token string) (Expr, error)
	infixParserFn func(p *parser, token string, left Expr) (Expr, error)
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

// exprKeywordsPrecedence maps operator tokens to their precedence and parser functions.
// This is initialized in init() to avoid cyclic reference errors at package init time.
var exprKeywordsPrecedence map[string]parseRule

func init() {
	exprKeywordsPrecedence = map[string]parseRule{
		"!":       {precedence: precedenceBang, prefixParser: parseUnaryBangOperator},
		"(":       {precedence: precedenceParens, prefixParser: parseUnaryOpenParenthesis, infixParser: parseBinaryApplyOperator},
		"defined": {precedence: precedenceLowest, prefixParser: parseDefinedExpr},
		"||":      {precedence: precedenceOr, infixParser: parseBinaryLogicOrOperator},
		"&&":      {precedence: precedenceAnd, infixParser: parseBinaryLogicAndOperator},
		"==":      {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
		"!=":      {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
		">":       {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
		">=":      {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
		"<":       {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
		"<=":      {precedence: precedenceCompare, infixParser: parseBinaryCompareOperator},
	}
}

// switch to enable logging of errors found when parsing sources
// used only for development purpuses, we don't log log errors in normal mode
const debug = false

// getPrefixParseFn returns a prefix parser for a token, or a default parser for identifiers/literals.
func getPrefixParseFn(token string) prefixParseFn {
	if rule, exists := exprKeywordsPrecedence[token]; exists && rule.prefixParser != nil {
		return rule.prefixParser
	}
	// Fallback: treat as identifier or integer literal
	return func(p *parser, token string) (Expr, error) {
		return parseValue(token)
	}
}

// parseExprPrecedence implements Pratt parsing for expressions, handling C preprocessor conditionals.
// minPrecedence controls operator binding (precedence climbing).
func (p *parser) parseExprPrecedence(minPrecedence precedence) (Expr, error) {
	token, err := p.nextToken()
	if err != nil {
		return nil, err
	}

	parsePrefix := getPrefixParseFn(token)
	result, err := parsePrefix(p, token)
	if err != nil {
		return nil, err
	}

	for {
		token, ok := p.tr.peek()
		if !ok {
			return result, nil // end of input
		}

		rule, exists := exprKeywordsPrecedence[token]
		if !exists || rule.precedence < minPrecedence {
			return result, nil // current operator binds less – stop and return
		}
		p.tr.mustConsume(token)
		result, err = rule.infixParser(p, token, result)
		if err != nil {
			return nil, err
		}
	}
}

func parseBinaryLogicOrOperator(p *parser, token string, lhs Expr) (Expr, error) {
	rhs, err := p.parseExprPrecedence(precedenceOr + 1)
	if err != nil {
		return nil, err
	}
	return Or{lhs, rhs}, nil
}

func parseBinaryLogicAndOperator(p *parser, token string, lhs Expr) (Expr, error) {
	rhs, err := p.parseExprPrecedence(precedenceAnd + 1)
	if err != nil {
		return nil, err
	}
	return And{lhs, rhs}, nil
}

func parseBinaryCompareOperator(p *parser, op string, lhs Expr) (Expr, error) {
	switch op {
	case "==", "!=", ">", ">=", "<", "<=":
		rhs, err := p.parseExprPrecedence(precedenceCompare + 1)
		if err != nil {
			return nil, err
		}
		return Compare{lhs, op, rhs}, nil
	default:
		panic(fmt.Sprintf("unknown binary compare operator %q", op))
	}
}

func parseBinaryApplyOperator(p *parser, _ string, lhs Expr) (Expr, error) {
	ident, ok := lhs.(Ident)
	if !ok {
		return nil, fmt.Errorf("expected identifier for apply operator, got %T", lhs)
	}

	args := []Expr{}
	for {
		token, ok := p.tr.peek()
		switch {
		case !ok || token == EOL:
			return nil, fmt.Errorf("unexpected end of input while parsing apply operator %q", ident)
		case token == ",":
			p.tr.mustConsume(token)
			continue
		case token == ")":
			p.tr.mustConsume(token)
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

func parseUnaryBangOperator(p *parser, _ string) (Expr, error) {
	inner, err := p.parseExprPrecedence(precedenceBang + 1)
	if err != nil {
		return nil, err
	}
	return Not{inner}, nil
}

func parseUnaryOpenParenthesis(p *parser, tok string) (Expr, error) {
	expr, err := p.parseExprPrecedence(precedenceLowest + 1)
	if err != nil {
		return nil, err
	}
	if err := p.tr.consume(")"); err != nil {
		return nil, err
	}
	return expr, nil
}

// parseIncludeDirective parses an #include or #include_next directive, extracting its path and kind (system/user).
func (p *parser) parseIncludeDirective(_ string) (Directive, error) {
	token, ok := p.tr.next()
	if !ok {
		return nil, nil
	}

	switch token {
	case "<":
		path, err := p.nextToken()
		if err != nil {
			return nil, err
		}
		err = p.tr.consume(">")
		if err != nil {
			return nil, fmt.Errorf("missing closing bracket: %v", err)
		}
		return IncludeDirective{Path: path, IsSystem: true, LineNumber: p.tr.lineNumber}, nil
	default:
		path := token
		if !strings.HasPrefix(path, "\"") || !strings.HasSuffix(path, "\"") {
			return nil, errors.New("malformed include, missing quotes")
		}
		unquoted := strings.Trim(path, "\"")
		if strings.Contains(unquoted, "\"") {
			return nil, errors.New("malformed include, quotes inside path")
		}
		return IncludeDirective{Path: unquoted, IsSystem: false, LineNumber: p.tr.lineNumber}, nil
	}
}

// parseDefinedExpr parses the `defined` operator for macro checks in #if expressions.
func parseDefinedExpr(p *parser, op string) (Expr, error) {
	var name Ident
	var err error
	switch {
	case p.tr.lookAheadIs("("):
		p.tr.mustConsume("(")
		name, err = p.parseIdent()
		if err != nil {
			return nil, err
		}
		if err := p.tr.consume(")"); err != nil {
			return nil, err
		}
	default:
		name, err = p.parseIdent()
		if err != nil {
			return nil, err
		}
	}
	return Defined{Name: name}, nil
}

type parser struct {
	tr         *tokenReader // Token reader for source
	sourceInfo SourceInfo   // Accumulated parser state
}

// parse reads and parses C/C++ source from an io.Reader, returning structured SourceInfo.
func parse(input io.Reader) (SourceInfo, error) {
	// TODO not used yet, only to verify that the tree-sitter library is linked correctly
	{
		parser := ts.NewParser()
		defer parser.Close()
		parser.SetLanguage(ts.NewLanguage(ts_cpp.Language()))
	}

	p := &parser{tr: newTokenReader(input)}
	directives, err := p.parseDirectivesUntil(func(_ string) bool { return p.tr.atEOF })
	p.sourceInfo.Directives = directives
	return p.sourceInfo, err
}

// parseDirectivesUntil reads tokens and parses directives until shouldStop returns true.
// It handles main(), #include, and preprocessor blocks, and builds the nested directive structure.
func (p *parser) parseDirectivesUntil(shouldStop func(token string) bool) ([]Directive, error) {
	directives := []Directive{}
	for {
		prev := p.tr.lastToken
		token, ok := p.tr.peek()
		if !ok {
			return directives, p.tr.scanner.Err()
		}

		if shouldStop(token) {
			return directives, nil
		}
		p.tr.mustConsume(token)

		switch {
		case strings.HasPrefix(token, "#"):
			if token == "#" {
				// `# directive` syntax, read and merge with next token
				directiveKind, err := p.nextToken()
				if err != nil {
					skipped, _ := p.readUntilEOL() // skip remaining part of directive
					if debug {
						log.Printf("Failed to parse %v directive: %v, skipping tokens until end of line: %v", token, err, skipped)
					}
					break
				}
				// parseDirective assumes full directive name including '#' prefix
				token = "#" + directiveKind
			}
			directive, err := p.parseDirective(token)
			if err != nil {
				skipped, _ := p.readUntilEOL() // skip remaining part of directive
				if debug {
					log.Printf("Failed to parse %v directive: %v, skipping tokens until end of line: %v", token, err, skipped)
				}
				break
			}
			directives = append(directives, directive)

		case token == "main":
			if next, exists := p.tr.next(); exists && next == "(" {
				if prev == "int" {
					p.sourceInfo.HasMain = true
				}
			}
		}
	}
}

// parseExpr parses a preprocessor expression (#if/#elif condition) as an Expr AST.
func (p *parser) parseExpr() (Expr, error) {
	return p.parseExprPrecedence(precedenceLowest)
}

// nextToken returns the next token or an error if EOF is reached.
func (p *parser) nextToken() (string, error) {
	token, ok := p.tr.next()
	if !ok {
		return "", errors.New("expected identifier, found EOF")
	}
	if token == EOL {
		return "", errors.New("expected token, found EOL")
	}
	return token, nil
}

// readUntilEOL skips all tokens until the end of the line, returning all read tokens as a slice.
func (p *parser) readUntilEOL() ([]string, error) {
	tokens := []string{}
	if p.tr.lastToken == EOL {
		return tokens, nil
	}
	for {
		token, ok := p.tr.next()
		if !ok {
			return tokens, p.tr.scanner.Err()
		}
		if token == EOL {
			return tokens, nil
		}
		tokens = append(tokens, token)
	}
}

// parseIdent reads the next identifier token.
func (p *parser) parseIdent() (Ident, error) {
	token, ok := p.tr.next()
	if !ok {
		return "", fmt.Errorf("expected identifier, found EOF")
	}
	if token == EOL {
		return "", fmt.Errorf("expected identifier, found EOL")
	}
	return Ident(token), nil
}

// isEndOfIfBranch checks if a token marks the end or transition of a #if block branch.
func isEndOfIfBranch(token string) bool {
	switch token {
	case "#endif", "#else", "#elif", "#elifdef", "#elifndef":
		return true
	default:
		return false
	}
}

// parseIfBranch parses a single #if/#ifdef/#ifndef/#elif/#elifdef/#elifndef branch and its body.
func (p *parser) parseIfBranch(directive string, kind BranchKind) (ConditionalBranch, error) {
	var cond Expr
	var err error

	switch directive {
	case "#ifdef", "#elifdef":
		ident, err := p.parseIdent()
		if err != nil {
			return ConditionalBranch{}, err
		}
		cond = Defined{ident}
	case "#ifndef", "#elifndef":
		ident, err := p.parseIdent()
		if err != nil {
			return ConditionalBranch{}, err
		}
		cond = Not{X: Defined{ident}}
	case "#if", "#elif":
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

// parseIfBlock parses an entire #if/#ifdef/#ifndef block (including #elif/#else/#endif) and all nested directives.
func (p *parser) parseIfBlock(startDirective string) (IfBlock, error) {
	var branches []ConditionalBranch

	firstBranch, err := p.parseIfBranch(startDirective, IfBranch)
	if err != nil {
		return IfBlock{}, err
	}
	branches = append(branches, firstBranch)

	for {
		token, ok := p.tr.peek()
		if !ok {
			return IfBlock{}, p.tr.scanner.Err()
		}

		switch token {
		case "#elif", "#elifdef", "#elifndef":
			p.tr.mustConsume(token)
			branch, err := p.parseIfBranch(token, ElifBranch)
			if err != nil {
				return IfBlock{}, err
			}
			branches = append(branches, branch)

		case "#else":
			p.tr.mustConsume(token)
			body, err := p.parseDirectivesUntil(func(tok string) bool { return tok == "#endif" })
			if err != nil {
				return IfBlock{}, err
			}
			branches = append(branches, ConditionalBranch{
				Kind:      ElseBranch,
				Condition: nil,
				Body:      body,
			})

		case "#endif":
			p.tr.mustConsume(token)
			return IfBlock{Branches: branches}, nil

		default:
			return IfBlock{}, fmt.Errorf("unexpected token %q inside #if block", token)
		}
	}
}

// parseDefineDirective parses a #define directive, capturing the macro name and tokens.
func (p *parser) parseDefineDirective() (DefineDirective, error) {
	ident, err := p.nextToken()
	if err != nil {
		return DefineDirective{}, err
	}
	defineArgs := []string{}
	if p.tr.lookAheadIs("(") {
		p.tr.mustConsume("(")
		// Function-like macro definition
	parseArgs:
		for {
			tok, err := p.nextToken()
			if err != nil {
				return DefineDirective{}, err
			}
			switch tok {
			case ")":
				break parseArgs // end of argument list
			case ",":
				// skip commas
				continue
			default:
				defineArgs = append(defineArgs, tok)
			}
		}
	}
	body, err := p.readUntilEOL()
	if err != nil {
		return DefineDirective{}, err
	}
	return DefineDirective{Name: ident, Args: defineArgs, Body: body}, nil
}

// parseUndefineDirective parses a #undef directive and its macro name.
func (p *parser) parseUndefineDirective() (UndefineDirective, error) {
	ident, err := p.nextToken()
	if err != nil {
		return UndefineDirective{}, err
	}
	return UndefineDirective{Name: ident}, nil
}

// parseDirective dispatches to the appropriate directive parser based on the token.
func (p *parser) parseDirective(token string) (Directive, error) {
	switch token {
	case "#include", "#include_next":
		return p.parseIncludeDirective(token)
	case "#ifdef", "#ifndef", "#if":
		return p.parseIfBlock(token)
	case "#define":
		return p.parseDefineDirective()
	case "#undef":
		return p.parseUndefineDirective()
	default:
		if isEndOfIfBranch(token) {
			return nil, fmt.Errorf("malformed input: unpaired #if condition token: %q", token)
		}
		return nil, fmt.Errorf("unknown directive: %q", token)
	}
}

// A valid macro identifier must follow these rules:
// * First character must be ‘_’ or a letter.
// * Subsequent characters may be ‘_’, letters, or decimal digits.
var macroIdentifierRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// A parsable integer literal can be in decimal, octal, or hex form, but may not include C specific suffixes.
var parsableIntegerRegex = regexp.MustCompile(`^(?:0[xX][0-9A-Fa-f]+|0[0-7]*|[1-9][0-9]*)$`)

// parseValue parses a token as an identifier or integer literal, for use in #if/#elif expressions.
func parseValue(token string) (Value, error) {
	if parsableIntegerRegex.MatchString(token) {
		if v, err := parseIntLiteral(token); err == nil {
			return ConstantInt(v), nil
		}
	}
	if macroIdentifierRegex.MatchString(token) {
		return Ident(token), nil
	}
	return nil, fmt.Errorf("token %q is neither identifier nor integer literal", token)
}

// parseIntLiteral parses an integer literal in decimal, octal, or hex form, ignoring C suffixes.
func parseIntLiteral(tok string) (int, error) {
	v, err := strconv.ParseInt(tok, 0, 32)
	return int(v), err
}
