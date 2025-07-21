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
	"log"
	"maps"

	"github.com/EngFlow/gazelle_cc/language/internal/cc"
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
func (si SourceInfo) CollectReachableIncludes(macros cc.Macros) []IncludeDirective {
	var result []IncludeDirective
	// Start with a copy of the provided macros, might be modified during evaluation
	var env cc.Macros = maps.Clone(macros)
	var walk func([]Directive)
	walk = func(directives []Directive) {
		for _, d := range directives {
			switch v := d.(type) {
			case IncludeDirective:
				result = append(result, v)

			case DefineDirective:
				intValue := 0
				switch {
				case len(v.Args) > 0:
					// Function-like macro definition is always assumed to be defined
					intValue = 1
				case len(v.Body) == 0:
					// Object-like macro definition with no body is defined as 1
					// #define FOO is interpreted as #define FOO 1
					intValue = 1
				default:
					// Object-like macro definition, try to parse the body
					// We only interpret the first token as an integer value
					if value, err := parseIntLiteral(v.Body[0]); err == nil {
						intValue = value
					}
				}
				env[v.Name] = intValue

			case UndefineDirective:
				delete(env, v.Name)

			case IfBlock:
				for _, branch := range v.Branches {
					shouldVisit := true // By default we visit the branch, unless the condition is present and evaluates to false
					if branch.Condition != nil {
						result, err := Evaluate(branch.Condition, env)
						if err != nil {
							if debug {
								log.Printf("Failed to evaluate condition %v: %v", branch.Condition, err)
							}
							continue
						}
						shouldVisit = result
					}
					if shouldVisit {
						walk(branch.Body)
						break
					}
				}
			}
		}
	}
	walk(si.Directives)
	return result
}
