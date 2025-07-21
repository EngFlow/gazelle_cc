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
)

type (
	// Expr represents an abstract syntax tree (AST) node for a C/C++ preprocessor #if condition.
	// Each Expr node implements fmt.Stringer for debugging and round-tripping.
	Expr interface {
		fmt.Stringer
		// Eval returns the result of evaluations the expression in given environemt (macro set). It may return 0 if the expression is not depends on unknown definitions.
		Eval(env Environment) (int, error)
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

func Evaluate(expr Expr, env Environment) (bool, error) {
	intValue, err := expr.Eval(env)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate expression %s: %w", expr, err)
	}
	return intValue != 0, nil
}

func (expr Defined) Eval(env Environment) (int, error) {
	_, exists := env[string(expr.Name)]
	return booleanToInt(exists), nil
}
func (expr Compare) Eval(env Environment) (int, error) {
	lv, err := expr.Left.Eval(env)
	if err != nil {
		return 0, err
	}
	rv, err := expr.Right.Eval(env)
	if err != nil {
		return 0, err
	}
	switch expr.Op {
	case "==":
		return booleanToInt(lv == rv), nil
	case "!=":
		return booleanToInt(lv != rv), nil
	case "<":
		return booleanToInt(lv < rv), nil
	case "<=":
		return booleanToInt(lv <= rv), nil
	case ">":
		return booleanToInt(lv > rv), nil
	case ">=":
		return booleanToInt(lv >= rv), nil
	default:
		log.Panicf("Unknown compare operation type: %v", expr)
		return 0, nil
	}
}
func (expr Apply) Eval(env Environment) (int, error) {
	// We do not support evaluating env with arguments in #if expressions
	// Assume that the macro is defined and return true
	return 1, nil
}
func (expr Not) Eval(env Environment) (int, error) {
	result, err := expr.X.Eval(env)
	if err != nil {
		return 0, err
	}
	if result == 0 {
		result = 1
	} else {
		result = 0
	}
	return result, nil
}
func (expr And) Eval(env Environment) (int, error) {
	lValue, err := expr.L.Eval(env)
	if err != nil || lValue == 0 {
		return 0, err
	}
	rValue, err := expr.R.Eval(env)
	if err != nil || rValue == 0 {
		return 0, err
	}
	return 1, nil
}
func (expr Or) Eval(env Environment) (int, error) {
	lValue, err := expr.L.Eval(env)
	if err != nil {
		return lValue, err
	}
	if lValue != 0 {
		return 1, nil
	}

	rValue, err := expr.R.Eval(env)
	if err != nil {
		return rValue, err
	}
	if rValue != 0 {
		return 1, nil
	}
	return 0, nil
}
func (expr Ident) Eval(env Environment) (int, error) {
	v, defined := env[string(expr)]
	if !defined {
		return 0, nil
	}
	return v, nil
}
func (expr ConstantInt) Eval(env Environment) (int, error) { return int(expr), nil }

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

func booleanToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
