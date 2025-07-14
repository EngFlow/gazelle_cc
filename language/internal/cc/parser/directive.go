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
	"strings"
)

type (
	// Directive represents a single preprocessor directive in a C/C++ translation unit.
	// This may be an include, define, undefine, or conditional block (#if/#ifdef).
	Directive interface {
		fmt.Stringer
	}
	// IncludeDirective represents a `#include` or `#include_next` preprocessor directive.
	// If IsSystem is true, angle brackets were used (<...>), otherwise quotes ("...").
	IncludeDirective struct {
		Path     string // Path of the included file
		IsSystem bool   // True if system include (angle brackets), false if user include (quotes)

	}
	// DefineDirective represents a `#define` preprocessor directive, including
	// the macro name and any replacement tokens.
	DefineDirective struct {
		Name string   // Name of the macro
		Args []string // 0 or more tokens representing arguments of the #define directive
		Body []string // 0 or more tokens representing body of the #define directive
	}
	// UndefineDirective represents a `#undef` preprocessor directive i.e., the removal of a macro definition.
	UndefineDirective struct {
		Name string // Name of the macro to undefine
	}
	// IfBlock represents a conditional compilation block such as #if/#ifdef/#ifndef, along with
	// any #elif and #else branches, and their nested directives.
	IfBlock struct {
		Branches []ConditionalBranch // All branches of the conditional, in order
	}
	// ConditionalBranch represents one branch in a conditional preprocessor block.
	// This may be #if, #elif, or #else. The Condition is nil for #else branches.
	ConditionalBranch struct {
		Kind      BranchKind  // The branch type (If, Elif, Else)
		Condition Expr        // Condition to evaluate (nil for #else)
		Body      []Directive // Nested directives inside this branch
	}
	// BranchKind identifies which kind of branch in a conditional preprocessor block.
	BranchKind int
)

const (
	IfBranch   BranchKind = iota // #if, #ifdef, #ifndef, etc.
	ElifBranch                   // #elif, #elifdef, #elifndef
	ElseBranch                   // #else
)

func (d IncludeDirective) String() string {
	if d.IsSystem {
		return fmt.Sprintf("#include <%s>", d.Path)
	}
	return fmt.Sprintf("#include \"%s\"", d.Path)
}
func (d DefineDirective) String() string {
	argsString := ""
	if len(d.Args) >= 0 {
		argsString = strings.Join(d.Args, ", ")
	}
	return fmt.Sprintf("#define %s(%s) %s", d.Name, argsString, strings.Join(d.Body, " "))
}
func (d UndefineDirective) String() string { return fmt.Sprintf("#undef %s", d.Name) }
func (d IfBlock) String() string {
	var out string
	for _, br := range d.Branches {
		out += br.String()
	}
	out += "#endif\n"
	return out
}

func (b ConditionalBranch) String() string {
	var prefix string
	switch b.Kind {
	case IfBranch:
		prefix = "#if"
	case ElifBranch:
		prefix = "#elif"
	case ElseBranch:
		prefix = "#else"
	}
	var cond string
	if b.Condition != nil {
		cond = " " + b.Condition.String()
	}
	var body string
	for _, d := range b.Body {
		body += d.String() + "\n"
	}
	return fmt.Sprintf("%s%s\n%s", prefix, cond, body)
}
