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
)

type (
	// Represents AST for #if conditions allowing for their analysis and evaluation
	Expr interface {
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
