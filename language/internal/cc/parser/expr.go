// Copyright 2026 EngFlow Inc. All rights reserved.
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

package parser

import (
	"fmt"
	"log"
	"strings"

	"github.com/EngFlow/gazelle_cc/language/internal/cc/lexer"
)

type (
	// Expr represents an abstract syntax tree (AST) node for a C/C++
	// preprocessor #if condition. Each Expr node implements fmt.Stringer for
	// debugging and round-tripping.
	Expr interface {
		fmt.Stringer
		// Eval returns the result of evaluations the expression in given
		// environemt (macro set). It may return 0 if the expression depends on
		// unknown definitions.
		Eval(env Environment) int
	}

	// Defined represents the defined(X) operator in #if expressions, checking
	// if a macro identifier is defined.
	Defined struct {
		Name Ident
	}

	// Not represents logical negation of a condition: !X
	Not struct {
		X Expr
	}

	// And represents a logical AND (X && Y) in #if expressions.
	And struct {
		L, R Expr
	}

	// Or represents a logical OR (X || Y) in #if expressions.
	Or struct {
		L, R Expr
	}

	// Compare represents a comparison between two values, e.g. A == B, A < B.
	Compare struct {
		// Left-hand side of the comparison
		Left Expr
		// Comparison operator: "==", "!=", "<", "<=", ">", ">="
		Op lexer.TokenType
		// Right-hand side of the comparison
		Right Expr
	}
	Apply struct {
		// Name or macro being applied.
		Name Ident
		// Arguments to the function or macro,
		Args []Expr
	}
)

type (
	// Value is a sub-interface of Expr, representing a literal value in a #if
	// expression.
	Value interface {
		Expr
	}
	// Ident is a macro identifier, such as _WIN32.
	Ident string
	// ConstantInt is an integer constant literal (e.g., 42).
	ConstantInt int
)

func (expr Defined) String() string { return fmt.Sprintf("defined(%s)", expr.Name) }
func (expr Compare) String() string { return fmt.Sprintf("%s %s %s", expr.Left, expr.Op, expr.Right) }
func (expr Apply) String() string {
	argStrings := make([]string, len(expr.Args))
	for i, arg := range expr.Args {
		argStrings[i] = arg.String()
	}
	return fmt.Sprintf("%s(%s)", expr.Name, strings.Join(argStrings, ", "))
}
func (expr Not) String() string         { return "!(" + expr.X.String() + ")" }
func (expr And) String() string         { return expr.L.String() + " && " + expr.R.String() }
func (expr Or) String() string          { return expr.L.String() + " || " + expr.R.String() }
func (expr Ident) String() string       { return string(expr) }
func (expr ConstantInt) String() string { return fmt.Sprintf("%d", expr) }

func Evaluate(expr Expr, env Environment) bool { return expr.Eval(env) != 0 }

func (expr Defined) Eval(env Environment) int {
	_, exists := env[string(expr.Name)]
	return booleanToInt(exists)
}
func (expr Compare) Eval(env Environment) int {
	lv := expr.Left.Eval(env)
	rv := expr.Right.Eval(env)
	switch expr.Op {
	case lexer.TokenType_OperatorEqual:
		return booleanToInt(lv == rv)
	case lexer.TokenType_OperatorNotEqual:
		return booleanToInt(lv != rv)
	case lexer.TokenType_OperatorLess:
		return booleanToInt(lv < rv)
	case lexer.TokenType_OperatorLessOrEqual:
		return booleanToInt(lv <= rv)
	case lexer.TokenType_OperatorGreater:
		return booleanToInt(lv > rv)
	case lexer.TokenType_OperatorGreaterOrEqual:
		return booleanToInt(lv >= rv)
	default:
		log.Panicf("Unknown compare operation type: %v", expr)
		return 0
	}
}
func (expr Apply) Eval(env Environment) int {
	// We do not support evaluating env with arguments in #if expressions.
	// Assume that the macro is defined and return true.
	return 1
}
func (expr Not) Eval(env Environment) int { return booleanToInt(expr.X.Eval(env) == 0) }
func (expr And) Eval(env Environment) int {
	return booleanToInt(expr.L.Eval(env) != 0 && expr.R.Eval(env) != 0)
}
func (expr Or) Eval(env Environment) int {
	return booleanToInt(expr.L.Eval(env) != 0 || expr.R.Eval(env) != 0)
}
func (expr Ident) Eval(env Environment) int {
	v, defined := env[string(expr)]
	if !defined {
		return 0
	}
	return v
}
func (expr ConstantInt) Eval(env Environment) int { return int(expr) }

func booleanToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
