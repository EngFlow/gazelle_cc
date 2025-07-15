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

package parser

import (
	"fmt"
	"log"
	"strings"

	"github.com/EngFlow/gazelle_cc/language/internal/cc"
)

type (
	// Expr represents an abstract syntax tree (AST) node for a C/C++ preprocessor #if condition.
	// Each Expr node implements fmt.Stringer for debugging and round-tripping.
	Expr interface {
		fmt.Stringer
		// Eval reports whether the expression evaluates to true for a given macro set
		Eval(macros cc.Macros) bool
	}

	// Defined represents the defined(X) operator in #if expressions,
	// checking if a macro identifier is defined.
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
		Left  Expr   // Left-hand side of the comparison
		Op    string // Comparison operator: "==", "!=", "<", "<=", ">", ">="
		Right Expr   // Right-hand side of the comparison
	}
	Apply struct {
		// Name or macro being applied.
		Name Ident
		// Arguments to the function or macro,
		Args []Expr
	}
)

type (
	// Value is a sub-interface of Expr, representing a literal value in a #if expression.
	Value interface {
		Expr
		// Evaluates given Value to integer value. The bool flag identifies if given macro is defined an can was successfully evaluated
		// Result of resolving a macro that is not defined in `macros` is implicitlly 0
		Resolve(macros cc.Macros) (int, bool) // bool==false -> “undefined”
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

func (expr Defined) Eval(macros cc.Macros) bool {
	_, exists := macros[string(expr.Name)]
	return exists
}
func (expr Compare) Eval(macros cc.Macros) bool {
	// Evaluate expression and convert boolean to int value or resolve values based on provided macros set environment.
	resolveExpr := func(expr Expr) int {
		switch v := expr.(type) {
		case Value:
			if intValue, defined := v.Resolve(macros); defined {
				return intValue
			}
		default:
			if v.Eval(macros) {
				return 1
			}
		}
		return 0
	}
	lv := resolveExpr(expr.Left)
	rv := resolveExpr(expr.Right)
	switch expr.Op {
	case "==":
		return lv == rv
	case "!=":
		return lv != rv
	case "<":
		return lv < rv
	case "<=":
		return lv <= rv
	case ">":
		return lv > rv
	case ">=":
		return lv >= rv
	default:
		log.Panicf("Unknown compare operation type: %v", expr)
		return false
	}
}
func (expr Apply) Eval(macros cc.Macros) bool {
	// We do not support evaluating macros with arguments in #if expressions
	// Assume that the macro is defined and return true
	return true
}
func (expr Not) Eval(macros cc.Macros) bool { return !expr.X.Eval(macros) }
func (expr And) Eval(macros cc.Macros) bool { return expr.L.Eval(macros) && expr.R.Eval(macros) }
func (expr Or) Eval(macros cc.Macros) bool  { return expr.L.Eval(macros) || expr.R.Eval(macros) }
func (expr Ident) Eval(macros cc.Macros) bool {
	value, _ := expr.Resolve(macros)
	return value != 0
}
func (expr ConstantInt) Eval(macros cc.Macros) bool { return expr != 0 }

func (expr Ident) Resolve(macros cc.Macros) (int, bool) {
	v, defined := macros[string(expr)]
	return v, defined
}
func (value ConstantInt) Resolve(macros cc.Macros) (int, bool) { return int(value), true }

// Negate returns a new Compare expression with the comparison operator logically negated.
// For example, == becomes !=, < becomes >=, and so on. Panics on unknown operator.
func (expr Compare) Negate() Compare {
	var newOperator string
	switch expr.Op {
	case "==":
		newOperator = "!="
	case "!=":
		newOperator = "=="
	case "<":
		newOperator = ">="
	case "<=":
		newOperator = ">"
	case ">":
		newOperator = "<="
	case ">=":
		newOperator = "<"
	default:
		panic(fmt.Sprintf("unknown compare operation type: %s", expr.Op))
	}
	return Compare{Left: expr.Left, Op: newOperator, Right: expr.Right}
}
