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
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/EngFlow/gazelle_cc/language/internal/cc/parser"
)

func TestSourceGroups(t *testing.T) {
	includes := func(files ...string) parser.SourceInfo {
		result := parser.SourceInfo{}
		for _, file := range files {
			result.Directives = append(result.Directives, parser.IncludeDirective{Path: file})
		}
		return result
	}
	testCases := []struct {
		clue     string
		input    sourceInfos
		expected sourceGroups
	}{
		{
			clue: "A source file with no includes should be unassigned",
			input: sourceInfos{
				"orphan.cc": {},
			},
			expected: sourceGroups{
				"orphan": {sources: []string{"orphan.cc"}},
			},
		},
		{
			clue: "Each header should form its own group even if it includes another",
			input: sourceInfos{
				"a.h": {},
				"b.h": includes("a.h"),
				"c.h": includes("b.h"),
			},
			expected: sourceGroups{
				"a": {sources: []string{"a.h"}},
				"b": {sources: []string{"b.h"}, dependsOn: []groupId{"a"}},
				"c": {sources: []string{"c.h"}, dependsOn: []groupId{"b"}},
			},
		},
		{
			clue: "Group source with header even when not included",
			input: sourceInfos{
				"a.h":  {},
				"a.c":  {},
				"b.cc": {},
				"b.h":  {},
			},
			expected: sourceGroups{
				"a": {sources: []string{"a.c", "a.h"}},
				"b": {sources: []string{"b.cc", "b.h"}},
			},
		},
		{
			clue: "Merge cyclic dependency sources",
			input: sourceInfos{
				"a.h":  includes("b.h"),
				"a.c":  includes("a.h"),
				"b.h":  includes("a.h"),
				"b.cc": includes("b.h"),
				"c.h":  includes("a.h"),
			},
			expected: sourceGroups{
				"a": {sources: []string{"a.c", "a.h", "b.cc", "b.h"}, subGroups: []groupId{"a", "b"}},
				"c": {sources: []string{"c.h"}, dependsOn: []groupId{"a"}},
			},
		},
		{
			clue: "Detect implementation based cycle",
			input: sourceInfos{
				"a.h":  {},
				"a.c":  includes("b.h"),
				"b.h":  {},
				"b.cc": includes("a.h"),
			},
			expected: sourceGroups{
				"a": {sources: []string{"a.c", "a.h", "b.cc", "b.h"}, subGroups: []groupId{"a", "b"}},
			},
		},
		{
			clue: "Handle cyclic dependencies among headers correctly",
			input: sourceInfos{
				"p.h": includes("q.h"),
				"q.h": includes("r.h"),
				"r.h": includes("p.h"),
			},
			expected: sourceGroups{
				"p": {sources: []string{"p.h", "q.h", "r.h"}, subGroups: []groupId{"p", "q", "r"}},
			},
		},
		{
			clue: "A source file that includes multiple unrelated headers should assigned to it's own group",
			input: sourceInfos{
				"m.h":      {},
				"n.h":      {},
				"o.h":      {},
				"file.cpp": includes("m.h", "n.h", "o.h"),
			},
			expected: sourceGroups{
				"m":    {sources: []string{"m.h"}},
				"n":    {sources: []string{"n.h"}},
				"o":    {sources: []string{"o.h"}},
				"file": {sources: []string{"file.cpp"}, dependsOn: []groupId{"m", "n", "o"}},
			},
		},

		{
			clue: "Correctly group mixed dependencies",
			input: sourceInfos{
				"a.h":  {},
				"b.h":  includes("a.h"),
				"c.h":  {},
				"d.h":  includes("c.h"),
				"e.h":  includes("d.h", "f1.h", "f2.h"),
				"f1.h": includes("e.h"),
				"f2.h": includes("e.h"),
				"g.h":  includes("b.h", "d.h"),
				"h.h":  includes("g.h"),
				"i.h":  includes("g.h"),
				"j.h":  includes("h.h", "i.h"),
			},
			expected: sourceGroups{
				"a": {sources: []string{"a.h"}},
				"b": {sources: []string{"b.h"}, dependsOn: []groupId{"a"}},
				"c": {sources: []string{"c.h"}},
				"d": {sources: []string{"d.h"}, dependsOn: []groupId{"c"}},
				"e": {sources: []string{"e.h", "f1.h", "f2.h"}, dependsOn: []groupId{"d"}, subGroups: []groupId{"e", "f1", "f2"}},
				"g": {sources: []string{"g.h"}, dependsOn: []groupId{"b", "d"}},
				"h": {sources: []string{"h.h"}, dependsOn: []groupId{"g"}},
				"i": {sources: []string{"i.h"}, dependsOn: []groupId{"g"}},
				"j": {sources: []string{"j.h"}, dependsOn: []groupId{"h", "i"}},
			},
		},
		{
			clue: "Header including an external include file should still form a group",
			input: sourceInfos{
				"lib.h":   {Directives: []parser.Directive{parser.IncludeDirective{Path: "system.h", IsSystem: true}}},
				"lib.cc":  {Directives: []parser.Directive{parser.IncludeDirective{Path: "lib.h"}}},
				"app.cpp": {Directives: []parser.Directive{parser.IncludeDirective{Path: "system.h", IsSystem: true}}},
			},
			expected: sourceGroups{
				"lib": {sources: []string{"lib.cc", "lib.h"}},
				"app": {sources: []string{"app.cpp"}},
			},
		},
		{
			clue: "Implementation of header should merge groups even if header does not",
			input: sourceInfos{
				"a.h":  {},
				"b.h":  {},
				"a.cc": includes("b.h"),
				"b.cc": includes("a.h"),
			},
			expected: sourceGroups{
				"a": {sources: []string{"a.cc", "a.h", "b.cc", "b.h"}, subGroups: []groupId{"a", "b"}},
			},
		},
		{
			clue: "Implementation of header does not merge if can define dependency",
			input: sourceInfos{
				"a.h":  {},
				"a.cc": {},
				"b.h":  {},
				"b.cc": includes("a.h"),
			},
			expected: sourceGroups{
				"a": {sources: []string{"a.cc", "a.h"}},
				"b": {sources: []string{"b.cc", "b.h"}, dependsOn: []groupId{"a"}},
			},
		},
	}

	for idx, tc := range testCases {
		result := groupSourcesByUnits(
			slices.Collect(maps.Keys(tc.input)),
			tc.input,
		)

		shouldFail := false
		for groupId, expected := range tc.expected {
			actual, exists := result[groupId]
			if !exists {
				t.Logf("In test case %d (%v): missing group: %v", idx, tc.clue, groupId)
				shouldFail = true
				continue
			}
			if fmt.Sprintf("%v", *expected) != fmt.Sprintf("%v", *actual) {
				t.Logf("In test case %d (%v): groups %v does not match\n\t- expected: %+v\n\t- obtained: %+v", idx, tc.clue, groupId, *expected, *actual)
				shouldFail = true
			}
		}
		for groupId, group := range result {
			_, exists := tc.expected[groupId]
			if !exists {
				t.Logf("In test case %d (%v): unexpected group: %v - %v", idx, tc.clue, groupId, group)
				shouldFail = true
			}
		}

		if shouldFail {
			t.Errorf("Test case %d (%v) failed", idx, tc.clue)
		}
	}
}
