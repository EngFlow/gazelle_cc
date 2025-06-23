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
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/EngFlow/gazelle_cc/language/internal/cc/platform"
)

type SourceInfo struct {
	Includes            Includes
	ConditionalIncludes []ConditionalInclude
	HasMain             bool
}

// Includes separates unconditional include paths by their quoting delimiter.
type Includes struct {
	DoubleQuote []string
	Bracket     []string
}

// ConditionalInclude holds a single #include together with the Expr that must be satisfied for it to become active.
// A nil Condition means the include is unconditional and could actually have been stored in Includes – callers may merge the two forms if desired.
type ConditionalInclude struct {
	Path      string
	Condition Expr // nil → unconditional
}

// ParseSource runs the extractor on an in‑memory buffer.
func ParseSource(input string) (SourceInfo, error) {
	reader := strings.NewReader(input)
	return extractSourceInfo(reader)
}

// ParseSourceFile opens `filename“ and feeds its contents to the extractor.
func ParseSourceFile(filename string) (SourceInfo, error) {
	file, err := os.Open(filename)
	if err != nil {
		return SourceInfo{}, err
	}
	defer file.Close()

	return extractSourceInfo(file)
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

func extractSourceInfo(input io.Reader) (SourceInfo, error) {
	tr := newTokenReader(input)

	var (
		sourceInfo  SourceInfo
		condStack   []Expr // active #if/#else nesting
		alreadySeen []Expr // for #elif / #else handling inside current group
		lastToken   string
	)

	push := func(expr Expr) { condStack = append(condStack, expr) }
	pop := func() bool {
		if len(condStack) > 0 {
			condStack = condStack[:len(condStack)-1]
			return true
		}
		return false
	}
	currentGuard := func() Expr {
		if len(condStack) == 0 {
			return nil
		}
		// AND-conjunction of the stack
		acc := condStack[0]
		for i := 1; i < len(condStack); i++ {
			acc = And{acc, condStack[i]}
		}
		return acc
	}

	var nextIdent func() (Ident, error)
	nextIdent = func() (Ident, error) {
		token, exists := tr.Next()
		if !exists {
			return "", fmt.Errorf("not found expected ident")
		}
		if token == "\\" {
			return nextIdent()
		}
		return Ident(token), nil
	}

	parseDirective := func(token string) error {
		switch token {
		case "#include":
			isBracketInclude := false
			include, ok := tr.Next()
			if !ok {
				return fmt.Errorf("unexpected end of file")
			}

			if include == "<" {
				include, ok = tr.Next()
				if !ok {
					return fmt.Errorf("unexpected end of file")
				}
				isBracketInclude = true
			} else if !strings.Contains(include, "\"") {
				// Malformed include, e.g. #include exception>
				isBracketInclude = true
			}

			path := strings.Trim(include, "\"")
			conditionExpr := currentGuard()
			switch {
			case conditionExpr != nil:
				sourceInfo.ConditionalIncludes = append(sourceInfo.ConditionalIncludes,
					ConditionalInclude{Path: path, Condition: conditionExpr})
			case isBracketInclude:
				sourceInfo.Includes.Bracket = append(sourceInfo.Includes.Bracket, path)
			default:
				sourceInfo.Includes.DoubleQuote = append(sourceInfo.Includes.DoubleQuote, path)

			}

		// ifdef conditions
		case "#ifdef", "#ifndef":
			ident, err := nextIdent()
			if err != nil {
				return err
			}
			var definition Expr = Defined{Name: ident}
			if token == "#ifndef" {
				definition = Not{definition}
			}
			push(definition)
			alreadySeen = append(alreadySeen, definition)

		case "#else":
			if !pop() {
				break // malformed source code
			}
			negateAll := Not{orAll(alreadySeen...)}
			push(negateAll)
			alreadySeen = append(alreadySeen, negateAll)

		case "#elifdef", "#elifndef": // C23 extension
			if !pop() {
				break // malformed source code
			}
			ident, err := nextIdent()
			if err != nil {
				return err
			}
			var newExpr Expr = Defined{Name: ident}
			if token == "#elifndef" {
				newExpr = Not{newExpr}
			}
			notPrev := Not{orAll(alreadySeen...)}
			branchExpr := And{newExpr, notPrev}
			push(branchExpr)
			alreadySeen = append(alreadySeen, newExpr)

		case "#endif":
			if !pop() {
				break // malformed source code
			}
			alreadySeen = nil // leave outer group’s memory intact

		case "#if":
			expr, err := parseExpr(tr)
			if err != nil {
				return err
			}
			push(expr)
			alreadySeen = append(alreadySeen, expr)

		case "#elif":
			if !pop() {
				break // malformed source code
			}
			expr, err := parseExpr(tr)
			if err != nil {
				return err
			}
			notPrev := Not{orAll(alreadySeen...)}
			branchExpr := And{expr, notPrev}
			push(branchExpr)
			alreadySeen = append(alreadySeen, expr)
		}
		return nil
	}

	// Main parser loop
	for {
		token, ok := tr.Next()
		if !ok {
			// EOF or error. In case of EOF scanner.Err() return nil
			return sourceInfo, tr.scanner.Err()
		}
		prevToken := lastToken
		lastToken = token

		if strings.HasPrefix(token, "#") {
			parseDirective(token)
			continue
		}

		if token == "main" {
			// TOOD: better detection of main signature
			// We should also check for return type aliases and check if input args
			if tok, exists := tr.Next(); exists && tok == "(" {
				if prevToken == "int" {
					sourceInfo.HasMain = true
				}
				continue
			}
		}
	}
}

func parseExpr(tr *tokenReader) (Expr, error) {
	// Collect all tokens until end of line for easier processing of directive
	// Can collect more then 1 line if ending with '\' character
	ts := tokensStream{}
collect:
	for {
		token, ok := tr.nextInternal(true)
		if !ok {
			return nil, fmt.Errorf("expected more tokens: %v", tr.scanner.Err())
		}
		switch token {
		case "\\":
			// Multiline expression, continue parsing next line
			if next, ok := tr.peekInternal(false); ok && next == EOL {
				_, _ = tr.Next() // consume EOL
				continue
			}
		case EOL:
			// End of single line expression
			break collect
		default:
			ts.tokens = append(ts.tokens, token)
		}
	}
	return parseOr(&ts)
}

func parseOr(ts *tokensStream) (Expr, error) {
	left, err := parseAnd(ts)
	if err != nil {
		return nil, err
	}
	for ts.peek("||") {
		_ = ts.consume("||")
		right, err := parseAnd(ts)
		if err != nil {
			return nil, err
		}
		left = Or{left, right}
	}
	return left, nil
}

func parseAnd(ts *tokensStream) (Expr, error) {
	left, err := parseUnary(ts)
	if err != nil {
		return nil, err
	}
	for ts.peek("&&") {
		_ = ts.consume("&&")
		right, err := parseUnary(ts)
		if err != nil {
			return nil, err
		}
		left = And{left, right}
	}
	return left, nil
}

func parseUnary(ts *tokensStream) (Expr, error) {
	switch {
	case ts.peek("!"):
		_ = ts.consume("!")
		expr, err := parseUnary(ts)
		if err != nil {
			return nil, err
		}
		return Not{expr}, nil

	case ts.peek("("):
		_ = ts.consume("(")
		expr, err := parseOr(ts)
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
			return nil, fmt.Errorf("invalid macro name %q", name)
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
			return nil, fmt.Errorf("macro %s: %v", name, err)
		}
		out[name] = value
	}
	return out, nil
}

func orAll(xs ...Expr) Expr {
	if len(xs) == 0 {
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

// Next returns the next token skipping <EOL> markers.
func (tr *tokenReader) Next() (string, bool) { return tr.nextInternal(false) }
func (tr *tokenReader) Peek() (string, bool) { return tr.peekInternal(false) }

func newTokenReader(r io.Reader) *tokenReader {
	sc := bufio.NewScanner(r)
	sc.Split(tokenizer)
	return &tokenReader{scanner: sc}
}

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

// nextInternal returns the next token, optionally filtering out EOL markers. The bool flag identicates if data was available
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

func (tr *tokenReader) peekInternal(keepEOL bool) (string, bool) {
	if tr.buf != nil {
		if !keepEOL && *tr.buf == EOL {
			return tr.Next() // ensure skip semantics
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
