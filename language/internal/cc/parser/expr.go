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
)

type (
	// Expr represents an abstract syntax tree (AST) node for a C/C++ preprocessor #if condition.
	// Each Expr node implements fmt.Stringer for debugging and round-tripping.
	Expr interface {
		fmt.Stringer
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
)

type (
	// Value is a sub-interface of Expr, representing a literal value in a #if expression.
	Value interface {
		Expr
	}
	// Ident is a macro identifier, such as _WIN32.
	Ident string
	// ConstantInt is an integer constant literal (e.g., 42).
	ConstantInt int
)

func (expr Defined) String() string     { return fmt.Sprintf("defined(%s)", expr.Name) }
func (expr Compare) String() string     { return fmt.Sprintf("%s %s %s", expr.Left, expr.Op, expr.Right) }
func (expr Not) String() string         { return "!(" + expr.X.String() + ")" }
func (expr And) String() string         { return expr.L.String() + " && " + expr.R.String() }
func (expr Or) String() string          { return expr.L.String() + " || " + expr.R.String() }
func (expr Ident) String() string       { return string(expr) }
func (expr ConstantInt) String() string { return fmt.Sprintf("%d", expr) }

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
