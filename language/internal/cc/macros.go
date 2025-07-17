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

package cc

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// List of defined/known macro definition and their corresponding integer values, e.g {"__ANDROID__": 1, "_M_ARM": 1}
// Any defined macro definition that does not have explicit value, is assumed to be equal 1, eg. `_WIN32`: 1
// We don't support string/float macro definitions and using them in comparsion expressions
type Macros map[string]int // e.g.

type MacroDefinition struct {
	Name     string
	IntValue int
}

// ParseMacros converts a slice of -D style macro definitions into a platform.Macros map,
// validating that each value is an integerliteral understood by the conditional-expression evaluator.
func ParseMacro(definition string) (MacroDefinition, error) {
	definition = strings.TrimPrefix(definition, "-D") // tolerate gcc/clang style
	name, stringValue := definition, ""               // default: bare macro

	if eqIdx := strings.Index(definition, "="); eqIdx >= 0 {
		name, stringValue = definition[:eqIdx], definition[eqIdx+1:]
	}

	if !MacroIdentifierRegex.MatchString(name) {
		return MacroDefinition{}, fmt.Errorf("invalid macro name %q", name)
	}

	var intValue int
	switch stringValue {
	case "": // FOO -> FOO=1
		intValue = 1
	default:
		if !ParsableIntegerRegex.MatchString(stringValue) {
			return MacroDefinition{}, fmt.Errorf("macro %s=%v, only integer literal values are allowed", name, stringValue)
		}
		var err error
		intValue, err = parseIntLiteral(stringValue)
		if err != nil {
			return MacroDefinition{}, fmt.Errorf("failed to parse macro value %s: %v", definition, err)
		}
	}
	return MacroDefinition{Name: name, IntValue: intValue}, nil
}

// ParseMacros converts a slice of -D style macro definitions into a cc.Macros map
// Validates that each value is an integer literal understood by the conditional-expression evaluator
// Returns error if at least one definition failed to parse
func ParseMacros(definitions []string) (Macros, error) {
	out := Macros{}
	var parsingErrors []error
	for _, d := range definitions {
		defn, err := ParseMacro(d)
		if err != nil {
			parsingErrors = append(parsingErrors, fmt.Errorf("failed to parse: %v: %v", d, err))
			continue
		}
		out[defn.Name] = defn.IntValue
	}
	return out, errors.Join(parsingErrors...)
}

// A valid macro identifier must follow these rules:
// * First character must be ‘_’ or a letter.
// * Subsequent characters may be ‘_’, letters, or decimal digits.
var MacroIdentifierRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var ParsableIntegerRegex = regexp.MustCompile(`^(?:0[xX][0-9a-fA-F]+|0[0-7]*|[1-9][0-9]*)(?:[uU](?:ll?|LL?)?|ll?[uU]?|LL?[uU]?)?$`)

// parseIntLiteral parses an integer literal in decimal, octal, or hex form, ignoring C suffixes.
func parseIntLiteral(tok string) (int, error) {
	tok = strings.TrimRightFunc(tok, func(r rune) bool {
		return r == 'u' || r == 'U' || r == 'l' || r == 'L'
	})
	v, err := strconv.ParseInt(tok, 0, 64)
	return int(v), err
}
