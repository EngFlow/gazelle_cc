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

	"github.com/EngFlow/gazelle_cc/language/internal/cc/platform"
)

type (
	// Represents AST for #if conditions allowing for their analysis and evaluation
	Expr interface {
		// Eval reports whether the expression evaluates to true for a given macro set
		Eval(macros platform.Macros) bool
		String() string
	}
	Defined struct{ Name Ident } // defined(x)
	Not     struct{ X Expr }
	And     struct{ L, R Expr } //  a && b
	Or      struct{ L, R Expr } //  a || b
	Compare struct {            // A 'op' B
		Left  Value
		Op    string // "==", "!=", "<", "<=", ">", ">="
		Right Value
	}
)

type (
	// Represents a values that can be part of #if expressions
	Value interface {
		// Evaluates given Value to integer value. The bool flag identifies if given macro is defined an can was successfully evaluated
		// Result of resolving a macro that is not defined in `macros` is implicitlly 0
		Resolve(macros platform.Macros) (int, bool) // bool==false -> “undefined”
		String() string
	}
	// Macro definition literal, e.g. _WIN32
	Ident string
	// Integer value literal, e.g. 42
	Constant int
)

func (expr Defined) String() string   { return fmt.Sprintf("defined(%s)", expr.Name) }
func (expr Compare) String() string   { return fmt.Sprintf("%s %s %d", expr.Left, expr.Op, expr.Right) }
func (expr Not) String() string       { return "!(" + expr.X.String() + ")" }
func (expr And) String() string       { return expr.L.String() + " && " + expr.R.String() }
func (expr Or) String() string        { return expr.L.String() + " || " + expr.R.String() }
func (value Ident) String() string    { return string(value) }
func (value Constant) String() string { return fmt.Sprintf("%d", value) }

func (expr Defined) Eval(macros platform.Macros) bool {
	_, exists := macros[string(expr.Name)]
	return exists
}
func (expr Compare) Eval(macros platform.Macros) bool {
	lv, _ := expr.Left.Resolve(macros)  // undefined -> 0
	rv, _ := expr.Right.Resolve(macros) // undefined -> 0
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
func (expr Not) Eval(macros platform.Macros) bool { return !expr.X.Eval(macros) }
func (expr And) Eval(macros platform.Macros) bool { return expr.L.Eval(macros) && expr.R.Eval(macros) }
func (expr Or) Eval(macros platform.Macros) bool  { return expr.L.Eval(macros) || expr.R.Eval(macros) }

func (value Ident) Resolve(macros platform.Macros) (int, bool) {
	v, defined := macros[string(value)]
	return v, defined
}
func (value Constant) Resolve(macros platform.Macros) (int, bool) {
	return int(value), true
}

// Negates the comparsion expresson by switching the operation to opposite kind, eg. == -> !=
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
		log.Panicf("Unknown compare operation type: %v", expr)
	}
	return Compare{Left: expr.Left, Op: newOperator, Right: expr.Right}
}
