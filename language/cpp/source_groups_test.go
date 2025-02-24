package cpp

import (
	"fmt"
	"slices"
	"testing"

	"github.com/EngFlow/gazelle_cpp/language/internal/cpp/parser"
)

func TestSourceGroups(t *testing.T) {
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
				groups:     map[groupId]*sourceGroup{},
				unassigned: []sourceFile{"orphan.cc"},
			},
		},
		{
			clue: "Each header should form its own group even if it includes another",
			input: sourceInfos{
				"a.h": {},
				"b.h": {Includes: parser.Includes{DoubleQuote: []string{"a.h"}}},
				"c.h": {Includes: parser.Includes{DoubleQuote: []string{"b.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {sources: []sourceFile{"a.h"}},
					"b": {sources: []sourceFile{"b.h"}, dependsOn: []groupId{"a"}},
					"c": {sources: []sourceFile{"c.h"}, dependsOn: []groupId{"b"}},
				},
				unassigned: nil,
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
				groups: map[groupId]*sourceGroup{
					"a": {sources: []sourceFile{"a.c", "a.h"}},
					"b": {sources: []sourceFile{"b.cc", "b.h"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Sources should be assigned to their directly included headers",
			input: sourceInfos{
				"a.h":    {},
				"a1.c":   {Includes: parser.Includes{DoubleQuote: []string{"a.h"}}},
				"a2.cc":  {Includes: parser.Includes{DoubleQuote: []string{"a.h"}}},
				"b.hpp":  {},
				"b1.cc":  {Includes: parser.Includes{DoubleQuote: []string{"b.hpp"}}},
				"b2.cpp": {Includes: parser.Includes{DoubleQuote: []string{"b.hpp"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {sources: []sourceFile{"a.h", "a1.c", "a2.cc"}},
					"b": {sources: []sourceFile{"b.hpp", "b1.cc", "b2.cpp"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Merge cyclic dependency sources",
			input: sourceInfos{
				"a.h":  {Includes: parser.Includes{DoubleQuote: []string{"b.h"}}},
				"a.c":  {Includes: parser.Includes{DoubleQuote: []string{"a.h"}}},
				"b.h":  {Includes: parser.Includes{DoubleQuote: []string{"a.h"}}},
				"b.cc": {Includes: parser.Includes{DoubleQuote: []string{"b.h"}}},
				"c.h":  {Includes: parser.Includes{DoubleQuote: []string{"a.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {sources: []sourceFile{"a.c", "a.h", "b.cc", "b.h"}},
					"c": {sources: []sourceFile{"c.h"}, dependsOn: []groupId{"a"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Handle cyclic dependencies among headers correctly",
			input: sourceInfos{
				"p.h": {Includes: parser.Includes{DoubleQuote: []string{"q.h"}}},
				"q.h": {Includes: parser.Includes{DoubleQuote: []string{"r.h"}}},
				"r.h": {Includes: parser.Includes{DoubleQuote: []string{"p.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"p": {sources: []sourceFile{"p.h", "q.h", "r.h"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "A source file that includes multiple unrelated headers should be unassigned",
			input: sourceInfos{
				"m.h":      {},
				"n.h":      {},
				"o.h":      {},
				"file.cpp": {Includes: parser.Includes{DoubleQuote: []string{"m.h", "n.h", "o.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"m": {sources: []sourceFile{"m.h"}},
					"n": {sources: []sourceFile{"n.h"}},
					"o": {sources: []sourceFile{"o.h"}},
				},
				unassigned: []sourceFile{"file.cpp"},
			},
		},

		{
			clue: "Ensure transitive dependencies are grouped",
			input: sourceInfos{
				"a.h":  {},
				"b.h":  {Includes: parser.Includes{DoubleQuote: []string{"a.h"}}},
				"c.h":  {},
				"d.h":  {Includes: parser.Includes{DoubleQuote: []string{"c.h"}}},
				"e.h":  {Includes: parser.Includes{DoubleQuote: []string{"d.h", "f1.h", "f2.h"}}},
				"f1.h": {Includes: parser.Includes{DoubleQuote: []string{"e.h"}}},
				"f2.h": {Includes: parser.Includes{DoubleQuote: []string{"e.h"}}},
				"g.h":  {Includes: parser.Includes{DoubleQuote: []string{"b.h", "d.h"}}},

				"h.h": {Includes: parser.Includes{DoubleQuote: []string{"g.h"}}},
				"i.h": {Includes: parser.Includes{DoubleQuote: []string{"g.h"}}},
				"j.h": {Includes: parser.Includes{DoubleQuote: []string{"h.h", "i.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {sources: []sourceFile{"a.h"}},
					"b": {sources: []sourceFile{"b.h"}, dependsOn: []groupId{"a"}},
					"c": {sources: []sourceFile{"c.h"}},
					"d": {sources: []sourceFile{"d.h"}, dependsOn: []groupId{"c"}},
					"e": {sources: []sourceFile{"e.h", "f1.h", "f2.h"}, dependsOn: []groupId{"d"}},
					"g": {sources: []sourceFile{"g.h"}, dependsOn: []groupId{"b", "d"}},
					"h": {sources: []sourceFile{"h.h"}, dependsOn: []groupId{"g"}},
					"i": {sources: []sourceFile{"i.h"}, dependsOn: []groupId{"g"}},
					"j": {sources: []sourceFile{"j.h"}, dependsOn: []groupId{"h", "i"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Ensure transitive dependencies do not merge groups",
			input: sourceInfos{
				"a.h":     {},
				"b.h":     {Includes: parser.Includes{DoubleQuote: []string{"a.h"}}},
				"c.h":     {Includes: parser.Includes{DoubleQuote: []string{"b.h"}}},
				"d.h":     {Includes: parser.Includes{DoubleQuote: []string{"c.h"}}},
				"file1.c": {Includes: parser.Includes{DoubleQuote: []string{"d.h"}}},
				"file2.c": {Includes: parser.Includes{DoubleQuote: []string{"b.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {sources: []sourceFile{"a.h"}},
					"b": {sources: []sourceFile{"b.h", "file2.c"}, dependsOn: []groupId{"a"}},
					"c": {sources: []sourceFile{"c.h"}, dependsOn: []groupId{"b"}},
					"d": {sources: []sourceFile{"d.h", "file1.c"}, dependsOn: []groupId{"c"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Sources should be assigned to the first group that provides all dependencies",
			input: sourceInfos{
				"h1.h":   {},
				"h2.h":   {Includes: parser.Includes{DoubleQuote: []string{"h1.h"}}},
				"h3.h":   {Includes: parser.Includes{DoubleQuote: []string{"h2.h"}}},
				"s1.c":   {Includes: parser.Includes{DoubleQuote: []string{"h1.h"}}},
				"s2.cpp": {Includes: parser.Includes{DoubleQuote: []string{"h1.h", "h2.h"}}},
				"s3.cc":  {Includes: parser.Includes{DoubleQuote: []string{"h1.h", "h2.h", "h3.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"h1": {sources: []sourceFile{"h1.h", "s1.c"}},
					"h2": {sources: []sourceFile{"h2.h", "s2.cpp"}, dependsOn: []groupId{"h1"}},
					"h3": {sources: []sourceFile{"h3.h", "s3.cc"}, dependsOn: []groupId{"h2"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Splitting into groups should ignore non local dependencies",
			input: sourceInfos{
				"h1.h":   {},
				"h2.h":   {Includes: parser.Includes{DoubleQuote: []string{"h1.h", "external/header.h"}}},
				"h3.h":   {Includes: parser.Includes{DoubleQuote: []string{"h2.h"}}},
				"s1.c":   {Includes: parser.Includes{DoubleQuote: []string{"h1.h"}}},
				"s2.cpp": {Includes: parser.Includes{DoubleQuote: []string{"h1.h", "h2.h", "ext/header.h"}}},
				"s3.cc":  {Includes: parser.Includes{DoubleQuote: []string{"h1.h", "h2.h", "h3.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"h1": {sources: []sourceFile{"h1.h", "s1.c"}},
					"h2": {sources: []sourceFile{"h2.h", "s2.cpp"}, dependsOn: []groupId{"h1"}},
					"h3": {sources: []sourceFile{"h3.h", "s3.cc"}, dependsOn: []groupId{"h2"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Header including an external system file should still form a group",
			input: sourceInfos{
				"lib.h":   {Includes: parser.Includes{Bracket: []string{"system.h"}}},
				"lib.cc":  {Includes: parser.Includes{DoubleQuote: []string{"lib.h"}}},
				"app.cpp": {Includes: parser.Includes{Bracket: []string{"system.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"lib": {sources: []sourceFile{"lib.cc", "lib.h"}},
				},
				unassigned: []sourceFile{"app.cpp"},
			},
		},
		{
			clue: "Implementation of header should merge groups even if header does not",
			input: sourceInfos{
				"a.h":  {},
				"b.h":  {},
				"a.cc": {Includes: parser.Includes{DoubleQuote: []string{"b.h"}}},
				"b.cc": {Includes: parser.Includes{DoubleQuote: []string{"a.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {sources: []sourceFile{"a.cc", "a.h", "b.cc", "b.h"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Implementation of header does not merge if can define dependency",
			input: sourceInfos{
				"a.h":  {},
				"a.cc": {},
				"b.h":  {},
				"b.cc": {Includes: parser.Includes{DoubleQuote: []string{"a.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {sources: []sourceFile{"a.cc", "a.h"}},
					"b": {sources: []sourceFile{"b.cc", "b.h"}, dependsOn: []groupId{"a"}},
				},
				unassigned: nil,
			},
		},
	}

	for idx, tc := range testCases {
		result := groupSourcesByHeaders(tc.input)

		shouldFail := false
		if slices.Compare(result.unassigned, tc.expected.unassigned) != 0 {
			t.Logf("In test case %d (%v) unassigned sources does not match:\n\t- expected: %+v\n\t- obtained: %+v", idx, tc.clue, tc.expected.unassigned, result.unassigned)
			shouldFail = true
		}
		for groupId, expected := range tc.expected.groups {
			actual, exists := result.groups[groupId]
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
		for groupId, group := range result.groups {
			_, exists := tc.expected.groups[groupId]
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
