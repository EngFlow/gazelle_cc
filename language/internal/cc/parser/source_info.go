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
