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
//   - Conditional compilation guards formed with `#if[*]`, `#ifdef`, `#ifndef` and friends, and converts the boolean logic into an Expr AST declared in the same package.
//   - The presence of a `main()` function – useful for distinguishing executables from libraries.
//
// The parser is not a complete C/C++ pre-processor – it only understands enough of the grammar to serve the purposes of gazelle_cc and deliberately ignores tokens that are irrelevant for dependency extraction.
package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/EngFlow/gazelle_cc/language/internal/cc/platform"
)

type SourceInfo struct {
	Includes []Include
	HasMain  bool
}

type Include struct {
	Path string
	// Wheter include is included using '<path>' syntax
	IsSystemInclude bool
	// '#if' condition guarding the expression, used to detect platform specific dependencies
	Condition Expr // nil -> unconditional
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
	tr        *tokenReader
	lastToken string

	// accumulated result
	sourceInfo SourceInfo

	// active #if/#else nesting – conjunction of these is the current guard
	conditionStack []Expr
	// stack of "already‑seen" branch expressions for each #if group;
	// used to build !previous when we see #else / #elif
	exprGroupStack [][]Expr
}

// Reads the content of input and extract CC source informations
func parse(input io.Reader) (SourceInfo, error) {
	p := &parser{tr: newTokenReader(input)}
	for {
		tok, ok := p.tr.next()
		if !ok {
			return p.sourceInfo, p.tr.scanner.Err()
		}
		prev := p.lastToken
		p.lastToken = tok

		if strings.HasPrefix(tok, "#") {
			if err := p.parseDirective(tok); err != nil {
				return p.sourceInfo, err
			}
			continue
		}
		if tok == "main" {
			if next, exists := p.tr.next(); exists && next == "(" {
				if prev == "int" {
					p.sourceInfo.HasMain = true
				}
			}
		}
	}
}

// currentGuard returns the AND‑conjunction of every active #if expression.
func (p *parser) currentGuard() Expr {
	if len(p.conditionStack) == 0 {
		return nil
	}
	acc := p.conditionStack[0]
	for i := 1; i < len(p.conditionStack); i++ {
		acc = And{acc, p.conditionStack[i]}
	}
	return acc
}
func (p *parser) pushCondition(expr Expr) { p.conditionStack = append(p.conditionStack, expr) }
func (p *parser) popCondition() bool {
	if len(p.conditionStack) == 0 {
		return false
	}
	p.conditionStack = p.conditionStack[:len(p.conditionStack)-1]
	return true
}

func (p *parser) currentGroup() []Expr {
	if len(p.exprGroupStack) == 0 {
		return nil
	}
	return p.exprGroupStack[len(p.exprGroupStack)-1]
}
func (p *parser) pushNewGroup(expr Expr) { p.exprGroupStack = append(p.exprGroupStack, []Expr{expr}) }
func (p *parser) appendToCurrentGroup(expr Expr) {
	if len(p.exprGroupStack) == 0 {
		log.Panic("parser invariant violated: no expression group present")
	}
	last := &p.exprGroupStack[len(p.exprGroupStack)-1]
	*last = append(*last, expr)
}
func (p *parser) popGroup() bool {
	if len(p.exprGroupStack) == 0 {
		return false
	}
	p.exprGroupStack = p.exprGroupStack[:len(p.exprGroupStack)-1]
	return true
}

// Returns the next macro definition identifier
func (p *parser) parseIdent() (Ident, error) {
	token, ok := p.tr.next()
	if !ok {
		return "", fmt.Errorf("expected identifier, found EOF")
	}
	if token == "\\" { // line continuation – skip and recurse
		return p.parseIdent()
	}
	return Ident(token), nil
}

func (p *parser) handleInclude() error {
	isBracket := false
	include, ok := p.tr.next()
	if !ok {
		return fmt.Errorf("unexpected EOF after #include")
	}

	// "<foo>" style – we saw the opening '<'
	if include == "<" {
		isBracket = true
		include, ok = p.tr.next()
		if !ok {
			return fmt.Errorf("unexpected EOF in bracketed include")
		}
	} else if !strings.Contains(include, "\"") {
		// Malformed input, e.g. `#include weird>`
		isBracket = true
	}

	p.sourceInfo.Includes = append(p.sourceInfo.Includes, Include{
		Path:            strings.Trim(include, "\""),
		IsSystemInclude: isBracket,
		Condition:       p.currentGuard(),
	})
	return nil
}

func (p *parser) handleIfdef(kind string) error {
	ident, err := p.parseIdent()
	if err != nil {
		return err
	}
	var expr Expr = Defined{Name: ident}
	if kind == "#ifndef" {
		expr = Not{expr}
	}
	p.pushCondition(expr)
	p.pushNewGroup(expr)
	return nil
}

func (p *parser) handleIf() error {
	expr, err := p.parseExpr()
	if err != nil {
		return err
	}
	p.pushCondition(expr)
	p.pushNewGroup(expr)
	return nil
}

func (p *parser) handleElse() {
	cur := p.currentGroup()
	if !p.popCondition() || cur == nil {
		return // malformed – silently ignore
	}
	neg := Not{orAll(cur...)}
	p.pushCondition(neg)
	p.appendToCurrentGroup(neg)
}

func (p *parser) handleElif(kind string) error {
	cur := p.currentGroup()
	if !p.popCondition() || cur == nil {
		return nil // malformed – silently ignore
	}

	var expr Expr
	switch kind {
	case "#elif":
		var err error
		expr, err = p.parseExpr()
		if err != nil {
			return err
		}
	case "#elifdef", "#elifndef":
		ident, err := p.parseIdent()
		if err != nil {
			return err
		}
		expr = Defined{Name: ident}
		if kind == "#elifndef" {
			expr = Not{expr}
		}
	}

	notPrev := Not{orAll(cur...)}
	branch := And{expr, notPrev}
	p.pushCondition(branch)
	p.appendToCurrentGroup(expr) // add only the raw expr for future !prev
	return nil
}

// Dispatcher for directive handlers
func (p *parser) parseDirective(tok string) error {
	switch tok {
	case "#include":
		return p.handleInclude()
	case "#ifdef", "#ifndef":
		return p.handleIfdef(tok)
	case "#if":
		return p.handleIf()
	case "#else":
		p.handleElse()
	case "#elif", "#elifdef", "#elifndef":
		return p.handleElif(tok)
	case "#endif":
		p.popCondition()
		p.popGroup()
	}
	return nil
}

// Reads the input until end of line or until end of multi-line macro expression and parses it into Expr
func (p *parser) parseExpr() (Expr, error) {
	// Collect all tokens until end of line for easier processing of directive
	// Can collect more then 1 line if ending with '\' character
	ts := tokensStream{}
	tr := p.tr
collect:
	for {
		token, ok := p.tr.nextInternal(true)
		if !ok {
			return nil, fmt.Errorf("expected more tokens: %v", tr.scanner.Err())
		}
		switch token {
		case "\\":
			// Multiline expression, continue parsing next line
			if next, ok := tr.peek(); ok && next == EOL {
				_, _ = tr.next() // consume EOL
				continue
			}
		case EOL:
			// End of single line expression
			break collect
		default:
			ts.tokens = append(ts.tokens, token)
		}
	}
	parser := exprParser{ts: &ts}
	return parser.parseOr()
}

// Parser for expressions working on already loaded and cleaned up list of tokens collected until end of possibly multine macro expression
// Used to parse the #if <expr> conditions, handles binary (&&, ||) and unary negation (!) operators
type exprParser struct {
	ts *tokensStream
}

func (ep *exprParser) parseOr() (Expr, error) {
	ts := ep.ts
	left, err := ep.parseAnd()
	if err != nil {
		return nil, err
	}
	for ts.peek("||") {
		_ = ts.consume("||")
		right, err := ep.parseAnd()
		if err != nil {
			return nil, err
		}
		left = Or{left, right}
	}
	return left, nil
}

func (ep *exprParser) parseAnd() (Expr, error) {
	ts := ep.ts
	left, err := ep.parseUnary()
	if err != nil {
		return nil, err
	}
	for ts.peek("&&") {
		_ = ts.consume("&&")
		right, err := ep.parseUnary()
		if err != nil {
			return nil, err
		}
		left = And{left, right}
	}
	return left, nil
}

func (ep *exprParser) parseUnary() (Expr, error) {
	ts := ep.ts
	switch {
	case ts.peek("!"):
		_ = ts.consume("!")
		expr, err := ep.parseUnary()
		if err != nil {
			return nil, err
		}
		return Not{expr}, nil

	case ts.peek("("):
		_ = ts.consume("(")
		expr, err := ep.parseOr()
		if err != nil {
			return nil, err
		}
		if err := ts.consume(")"); err != nil {
			return nil, err
		}
		return expr, err

	case ts.peek("defined"):
		_ = ts.consume("defined")
		if ts.peek("(") {
			_ = ts.consume("(")
			name := Ident(ts.next())
			if err := ts.consume(")"); err != nil {
				return nil, err
			}
			return Defined{Name: name}, nil
		}
		return Defined{Name: Ident(ts.next())}, nil
	}

	token := ts.next()
	if ts.idx < len(ts.tokens) && isBinaryCompareOperator(ts.tokens[ts.idx]) {
		op := ts.next() // ==, !=, <, ...
		lValue, err := interpretValue(token)
		if err != nil {
			return nil, err
		}
		rightToken := ts.next()
		rValue, err := interpretValue(rightToken)
		if err != nil {
			return nil, err
		}
		return Compare{Left: lValue, Op: op, Right: rValue}, nil
	}
	return Compare{Left: Ident(token), Op: "!=", Right: Constant(0)}, nil
}

// interpretValue converts a token into either Ident or Constant.
func interpretValue(token string) (Value, error) {
	if macroIdentifierRegex.MatchString(token) {
		return Ident(token), nil
	}
	if value, err := parseIntLiteral(token); err == nil {
		return Constant(value), nil
	}
	return nil, fmt.Errorf("neither a valid identifier of integer constant")
}

func isBinaryCompareOperator(tok string) bool {
	switch tok {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

func parseIntLiteral(tok string) (int, error) {
	// handle decimal, octal, hex (base 0) and ignore U/L suffixes
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

func orAll(xs ...Expr) Expr {
	if len(xs) == 0 {
		log.Panicf("empty orAll")
		return nil
	}
	acc := xs[0]
	for i := 1; i < len(xs); i++ {
		acc = Or{acc, xs[i]}
	}
	return acc
}

// Thin wrapper around bufio.Scanner that provides `peek` and `next“ primitives while automatically skipping the ubiquitous newline marker except when explicitly requested.
// When an algorithm needs to honour line boundaries (e.g. parseExpr) it calls nextInternal/peekInternal instead.
type tokenReader struct {
	scanner *bufio.Scanner
	buf     *string // one‑token look‑ahead; nil when empty
}

func newTokenReader(r io.Reader) *tokenReader {
	sc := bufio.NewScanner(r)
	sc.Split(tokenizer)
	return &tokenReader{scanner: sc}
}

// next returns the next token skipping <EOL> markers.
func (tr *tokenReader) next() (string, bool) { return tr.nextInternal(false) }
func (tr *tokenReader) peek() (string, bool) { return tr.peekInternal(false) }

// internal helper: fetches next raw token from scanner. The bool flag identicates if data was available
func (tr *tokenReader) fetch() (string, bool) {
	if tr.buf != nil {
		tok := *tr.buf
		tr.buf = nil
		return tok, true
	}
	if !tr.scanner.Scan() {
		return "", false
	}
	return tr.scanner.Text(), true
}

// returns the next token, optionally filtering out EOL markers. The bool flag identicates if data was available
func (tr *tokenReader) nextInternal(keepEOL bool) (string, bool) {
	for {
		tok, ok := tr.fetch()
		if !ok {
			return "", false
		}
		if tok == EOL && !keepEOL {
			continue // skip
		}
		return tok, true
	}
}

// returns the next token but does not consume the input, optionally filtering out EOL markers. The bool flag identicates if data was available
func (tr *tokenReader) peekInternal(keepEOL bool) (string, bool) {
	if tr.buf != nil {
		if !keepEOL && *tr.buf == EOL {
			return tr.next() // ensure skip semantics
		}
		return *tr.buf, true
	}
	tok, ok := tr.nextInternal(keepEOL)
	if !ok {
		return "", false
	}
	tr.buf = &tok
	return tok, true
}

// Expression parser on already read list of tokens to simplify the logic
type tokensStream struct {
	tokens []string
	idx    int
}

func (ts *tokensStream) peek(s string) bool {
	return ts.idx < len(ts.tokens) && ts.tokens[ts.idx] == s
}
func (ts *tokensStream) consume(s string) error {
	if !ts.peek(s) {
		var next string
		if ts.idx < len(ts.tokens) {
			next = ts.tokens[ts.idx]
		} else {
			next = "<EOF>"
		}
		return fmt.Errorf("expected %v, got %v", s, next)
	}
	ts.idx++
	return nil
}
func (ts *tokensStream) next() string {
	if ts.idx >= len(ts.tokens) {
		panic("unexpected EOL in expression")
	}
	val := ts.tokens[ts.idx]
	ts.idx++
	return val
}
