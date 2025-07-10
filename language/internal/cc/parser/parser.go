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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/EngFlow/gazelle_cc/language/internal/cc/platform"
)

// SourceInfo contains the structural information extracted from a C/C++ source file.
type SourceInfo struct {
	Directives []Directive // Top-level parsed preprocessor directives (may be nested)
	HasMain    bool        // True if a main() function is detected
}

// CollectIncludes recursively traverses the directive tree and returns all IncludeDirective
// instances, flattening the nested IfBlock structure. This allows consumers to extract all
// discovered #include directives, regardless of conditional logic.
func (si SourceInfo) CollectIncludes() []IncludeDirective {
	var result []IncludeDirective
	var walk func([]Directive)
	walk = func(directives []Directive) {
		for _, d := range directives {
			switch v := d.(type) {
			case IncludeDirective:
				result = append(result, v)

			case IfBlock:
				for _, branch := range v.Branches {
					walk(branch.Body)
				}
			}
		}
	}
	walk(si.Directives)
	return result
}

// CollectIncludes recursively traverses the directive tree based on the successuflly evaluated conditions
// and returns all found IncludeDirective instances. This allows consumers to extract
// discovered #include directives based on given predefined environment
func (si SourceInfo) CollectReachableIncludes(macros platform.Macros) []IncludeDirective {
	var result []IncludeDirective
	var walk func([]Directive, platform.Macros)
	walk = func(directives []Directive, env platform.Macros) {
		for _, d := range directives {
			switch v := d.(type) {
			case IncludeDirective:
				result = append(result, v)

			case DefineDirective:
				intValue := 1
				if len(v.Tokens) > 0 {
					if value, err := parseIntLiteral(v.Tokens[0]); err == nil {
						intValue = value
					}
				}
				env[v.Name] = intValue

			case UndefineDirective:
				delete(env, v.Name)

			case IfBlock:
				for _, branch := range v.Branches {
					if branch.Condition == nil || branch.Condition.Eval(env) {
						walk(branch.Body, maps.Clone(env))
						break
					}
				}
			}
		}
	}
	walk(si.Directives, maps.Clone(macros))
	return result
}

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

// ParseMacros converts a slice of -D style macro definitions into a platform.Macros map,
// validating that each value is an integerliteral understood by the conditional-expression evaluator.
func ParseMacros(defs []string) (platform.Macros, error) {
	out := platform.Macros{}
	for _, d := range defs {
		d = strings.TrimPrefix(d, "-D") // tolerate gcc/clang style
		name, raw := d, ""              // default: bare macro

		if eq := strings.IndexByte(d, '='); eq >= 0 {
			name, raw = d[:eq], d[eq+1:]
		}

		if !macroIdentifierRegex.MatchString(name) {
			return out, fmt.Errorf("invalid macro name %q", name)
		}

		if raw == "" { // FOO -> FOO=1
			out[name] = 1
			continue
		}

		if !parsableIntegerRegex.MatchString(raw) {
			return nil, fmt.Errorf("macro %s=%v, only integer literal values are allowed", name, raw)
		}
		value, err := parseIntLiteral(raw)
		if err != nil {
			return out, fmt.Errorf("macro %s: %v", name, err)
		}
		out[name] = value
	}
	return out, nil
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
		"(":       {precedence: precedenceParens, prefixParser: parseUnaryOpenParenthesis},
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
		return IncludeDirective{Path: path, IsSystem: true}, nil
	default:
		path := token
		if !strings.HasPrefix(path, "\"") || !strings.HasSuffix(path, "\"") {
			return nil, errors.New("malformed include, missing quotes")
		}
		unquoted := strings.Trim(path, "\"")
		if strings.Contains(unquoted, "\"") {
			return nil, errors.New("malformed include, quotes inside path")
		}
		return IncludeDirective{Path: unquoted, IsSystem: false}, nil
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

func isParanthesis(char rune) bool {
	switch char {
	case '(', ')', '[', ']', '{', '}':
		return true
	default:
		return false
	}
}

func isEOL(char byte) bool { return char == '\n' }

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
			for i < len(data) && data[i] != '\n' {
				i++
			}
		// Skip block comments
		case bytes.HasPrefix(data[i:], []byte("/*")):
			i += 2
			for i < len(data)-1 {
				if data[i] == '*' && data[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
		// Skip whitespace
		case unicode.IsSpace(rune(char)):
			i++

		case isParanthesis(rune(char)):
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
					unicode.IsSpace(char) || isParanthesis(char) {
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

type parser struct {
	tr         *tokenReader // Token reader for source
	sourceInfo SourceInfo   // Accumulated parser state
}

// parse reads and parses C/C++ source from an io.Reader, returning structured SourceInfo.
func parse(input io.Reader) (SourceInfo, error) {
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
			directive, err := p.parseDirective(token)
			if err != nil {
				p.skipLine()
				// log.Printf("Failed to parse %v directive: %v, skipping tokens until end of line: %v", token, err, skipped)
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
		return "", fmt.Errorf("expected identifier, found EOF")
	}
	return token, nil
}

// skipLine skips all tokens until the end of the line, returning skipped tokens for error recovery.
func (p *parser) skipLine() ([]string, error) {
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
	tokens, err := p.skipLine()
	if err != nil {
		return DefineDirective{}, err
	}
	return DefineDirective{Name: ident, Tokens: tokens}, nil
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
		return nil, nil
	}
}

// parseValue parses a token as an identifier or integer literal, for use in #if/#elif expressions.
func parseValue(token string) (Value, error) {
	if macroIdentifierRegex.MatchString(token) {
		return Ident(token), nil
	}
	if v, err := parseIntLiteral(token); err == nil {
		return ConstantInt(v), nil
	}
	return nil, fmt.Errorf("token %q is neither identifier nor integer literal", token)
}

// parseIntLiteral parses an integer literal in decimal, octal, or hex form, ignoring C suffixes.
func parseIntLiteral(tok string) (int, error) {
	tok = strings.TrimRightFunc(tok, func(r rune) bool {
		return r == 'u' || r == 'U' || r == 'l' || r == 'L'
	})
	v, err := strconv.ParseInt(tok, 0, 64)
	return int(v), err
}

// A valid macro identifier must follow these rules:
// * First character must be ‘_’ or a letter.
// * Subsequent characters may be ‘_’, letters, or decimal digits.
var macroIdentifierRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var parsableIntegerRegex = regexp.MustCompile(`^(?:0[xX][0-9a-fA-F]+|0[0-7]*|[1-9][0-9]*)(?:[uU](?:ll?|LL?)?|ll?[uU]?|LL?[uU]?)?$`)

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

// returns the next token but does not consume the input, optionally filtering out EOL markers. The bool flag identicates if data was available
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
