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
	"errors"
	"fmt"
	"maps"
	"strings"
)

// List of defined/known macro definition and their corresponding integer values, e.g {"__ANDROID__": 1, "_M_ARM": 1}
// Any defined macro definition that does not have explicit value, is assumed to be equal 1, eg. `_WIN32`: 1
// We don't support string/float macro definitions and using them in comparsion expressions
type Environment map[string]int // e.g.

func (e Environment) Clone() Environment {
	return maps.Clone(e)
}

type MacroDefinition struct {
	Name  string
	Value int
}

// ParseMacros converts a slice of -D style macro definitions into a platform.Macros map,
// validating that each value is an integerliteral understood by the conditional-expression evaluator.
func ParseMacro(definition string) (MacroDefinition, error) {
	definition = strings.TrimPrefix(definition, "-D") // tolerate gcc/clang style
	name, stringValue := definition, ""               // default: bare macro

	if eqIdx := strings.Index(definition, "="); eqIdx >= 0 {
		name, stringValue = definition[:eqIdx], definition[eqIdx+1:]
	}

	if !macroIdentifierRegex.MatchString(name) {
		return MacroDefinition{}, fmt.Errorf("invalid macro name %q", name)
	}

	var value int
	switch stringValue {
	case "": // FOO -> FOO=1
		value = 1
	default:
		intValue, err := parseIntLiteral(stringValue)
		if err != nil {
			return MacroDefinition{}, fmt.Errorf("failed to parse macro value %s: %v", definition, err)
		}
		value = intValue
	}
	return MacroDefinition{Name: name, Value: value}, nil
}

// ParseMacros converts a slice of -D style macro definitions into a cc.Macros map
// Validates that each value is an integer literal understood by the conditional-expression evaluator
// Returns error if at least one definition failed to parse
func ParseMacros(definitions []string) (Environment, error) {
	out := Environment{}
	var parsingErrors []error
	for _, d := range definitions {
		defn, err := ParseMacro(d)
		if err != nil {
			parsingErrors = append(parsingErrors, fmt.Errorf("failed to parse: %v: %v", d, err))
			continue
		}
		out[defn.Name] = defn.Value
	}
	return out, errors.Join(parsingErrors...)
}
