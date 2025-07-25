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
type Environment map[string]int

func (e Environment) Clone() Environment {
	return maps.Clone(e)
}

type macroDefinition struct {
	Name  string
	Value int
}

// The function returns an Environment map where each macro name is a key and its value is an integer.
// Validates that each value is an integer literal understood by the conditional-expression evaluator
// Each definition is expected to be in the form of "NAME=VALUE" or just "NAME".
// If a definition does not have an explicit value, it defaults to 1 (e.g., "FOO" is equivalent to "FOO=1").
// If any definition fails to parse, the function returns an error.
// The macro names must be valid identifiers, and the values must be integer literals.
// The function does not support string or float macro definitions, and it will return an error if it encounters such definitions.
// Returns error if at least one definition failed to parse
func ParseMacros(definitions []string) (Environment, error) {
	out := Environment{}
	var parsingErrors []error
	for _, d := range definitions {
		defn, err := parseMacro(d)
		if err != nil {
			parsingErrors = append(parsingErrors, fmt.Errorf("failed to parse: %v: %v", d, err))
			continue
		}
		out[defn.Name] = defn.Value
	}
	return out, errors.Join(parsingErrors...)
}

func parseMacro(definition string) (macroDefinition, error) {
	name, stringValue, _ := strings.Cut(definition, "=")
	if !macroIdentifierRegex.MatchString(name) {
		return macroDefinition{}, fmt.Errorf("invalid macro name %q", name)
	}

	var value int
	switch stringValue {
	case "": // FOO -> FOO=1
		value = 1
	default:
		intValue, err := parseIntLiteral(stringValue)
		if err != nil {
			return macroDefinition{}, fmt.Errorf("failed to parse macro value %s: %v", definition, err)
		}
		value = intValue
	}
	return macroDefinition{Name: name, Value: value}, nil
}
