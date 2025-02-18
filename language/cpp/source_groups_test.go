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
				"b.h": {Includes: parser.Includes{DoubleQuote: []sourceFile{"a.h"}}},
				"c.h": {Includes: parser.Includes{DoubleQuote: []sourceFile{"b.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {hdrs: []sourceFile{"a.h"}},
					"b": {hdrs: []sourceFile{"b.h"}, dependsOn: []groupId{"a"}},
					"c": {hdrs: []sourceFile{"c.h"}, dependsOn: []groupId{"b"}},
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
					"a": {srcs: []sourceFile{"a.c"}, hdrs: []sourceFile{"a.h"}},
					"b": {srcs: []sourceFile{"b.cc"}, hdrs: []sourceFile{"b.h"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Sources should be assigned to their directly included headers",
			input: sourceInfos{
				"a.h":    {},
				"a1.c":   {Includes: parser.Includes{DoubleQuote: []sourceFile{"a.h"}}},
				"a2.cc":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"a.h"}}},
				"b.hpp":  {},
				"b1.cc":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"b.hpp"}}},
				"b2.cpp": {Includes: parser.Includes{DoubleQuote: []sourceFile{"b.hpp"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {srcs: []sourceFile{"a1.c", "a2.cc"}, hdrs: []sourceFile{"a.h"}},
					"b": {srcs: []sourceFile{"b1.cc", "b2.cpp"}, hdrs: []sourceFile{"b.hpp"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Merge cyclic dependency sources",
			input: sourceInfos{
				"a.h":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"b.h"}}},
				"a.c":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"a.h"}}},
				"b.h":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"a.h"}}},
				"b.cc": {Includes: parser.Includes{DoubleQuote: []sourceFile{"b.h"}}},
				"c.h":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"a.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {srcs: []sourceFile{"a.c", "b.cc"}, hdrs: []sourceFile{"a.h", "b.h"}},
					"c": {hdrs: []sourceFile{"c.h"}, dependsOn: []groupId{"a"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Handle cyclic dependencies among headers correctly",
			input: sourceInfos{
				"p.h": {Includes: parser.Includes{DoubleQuote: []sourceFile{"q.h"}}},
				"q.h": {Includes: parser.Includes{DoubleQuote: []sourceFile{"r.h"}}},
				"r.h": {Includes: parser.Includes{DoubleQuote: []sourceFile{"p.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"p": {hdrs: []sourceFile{"p.h", "q.h", "r.h"}},
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
				"file.cpp": {Includes: parser.Includes{DoubleQuote: []sourceFile{"m.h", "n.h", "o.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"m": {hdrs: []sourceFile{"m.h"}},
					"n": {hdrs: []sourceFile{"n.h"}},
					"o": {hdrs: []sourceFile{"o.h"}},
				},
				unassigned: []sourceFile{"file.cpp"},
			},
		},

		{
			clue: "Ensure transitive dependencies are grouped",
			input: sourceInfos{
				"a.h":  {},
				"b.h":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"a.h"}}},
				"c.h":  {},
				"d.h":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"c.h"}}},
				"e.h":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"d.h", "f1.h", "f2.h"}}},
				"f1.h": {Includes: parser.Includes{DoubleQuote: []sourceFile{"e.h"}}},
				"f2.h": {Includes: parser.Includes{DoubleQuote: []sourceFile{"e.h"}}},
				"g.h":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"b.h", "d.h"}}},

				"h.h": {Includes: parser.Includes{DoubleQuote: []sourceFile{"g.h"}}},
				"i.h": {Includes: parser.Includes{DoubleQuote: []sourceFile{"g.h"}}},
				"j.h": {Includes: parser.Includes{DoubleQuote: []sourceFile{"h.h", "i.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {hdrs: []sourceFile{"a.h"}},
					"b": {hdrs: []sourceFile{"b.h"}, dependsOn: []groupId{"a"}},
					"c": {hdrs: []sourceFile{"c.h"}},
					"d": {hdrs: []sourceFile{"d.h"}, dependsOn: []groupId{"c"}},
					"e": {hdrs: []sourceFile{"e.h", "f1.h", "f2.h"}, dependsOn: []groupId{"d"}},
					"g": {hdrs: []sourceFile{"g.h"}, dependsOn: []groupId{"b", "d"}},
					"h": {hdrs: []sourceFile{"h.h"}, dependsOn: []groupId{"g"}},
					"i": {hdrs: []sourceFile{"i.h"}, dependsOn: []groupId{"g"}},
					"j": {hdrs: []sourceFile{"j.h"}, dependsOn: []groupId{"h", "i"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Ensure transitive dependencies do not merge groups",
			input: sourceInfos{
				"a.h":     {},
				"b.h":     {Includes: parser.Includes{DoubleQuote: []sourceFile{"a.h"}}},
				"c.h":     {Includes: parser.Includes{DoubleQuote: []sourceFile{"b.h"}}},
				"d.h":     {Includes: parser.Includes{DoubleQuote: []sourceFile{"c.h"}}},
				"file1.c": {Includes: parser.Includes{DoubleQuote: []sourceFile{"d.h"}}},
				"file2.c": {Includes: parser.Includes{DoubleQuote: []sourceFile{"b.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {hdrs: []sourceFile{"a.h"}},
					"b": {hdrs: []sourceFile{"b.h"}, srcs: []sourceFile{"file2.c"}, dependsOn: []groupId{"a"}},
					"c": {hdrs: []sourceFile{"c.h"}, dependsOn: []groupId{"b"}},
					"d": {hdrs: []sourceFile{"d.h"}, srcs: []sourceFile{"file1.c"}, dependsOn: []groupId{"c"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Sources should be assigned to the first group that provides all dependencies",
			input: sourceInfos{
				"h1.h":   {},
				"h2.h":   {Includes: parser.Includes{DoubleQuote: []sourceFile{"h1.h"}}},
				"h3.h":   {Includes: parser.Includes{DoubleQuote: []sourceFile{"h2.h"}}},
				"s1.c":   {Includes: parser.Includes{DoubleQuote: []sourceFile{"h1.h"}}},
				"s2.cpp": {Includes: parser.Includes{DoubleQuote: []sourceFile{"h1.h", "h2.h"}}},
				"s3.cc":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"h1.h", "h2.h", "h3.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"h1": {hdrs: []sourceFile{"h1.h"}, srcs: []sourceFile{"s1.c"}},
					"h2": {hdrs: []sourceFile{"h2.h"}, srcs: []sourceFile{"s2.cpp"}, dependsOn: []groupId{"h1"}},
					"h3": {hdrs: []sourceFile{"h3.h"}, srcs: []sourceFile{"s3.cc"}, dependsOn: []groupId{"h2"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Splitting into groups should ignore non local dependencies",
			input: sourceInfos{
				"h1.h":   {},
				"h2.h":   {Includes: parser.Includes{DoubleQuote: []sourceFile{"h1.h", "external/header.h"}}},
				"h3.h":   {Includes: parser.Includes{DoubleQuote: []sourceFile{"h2.h"}}},
				"s1.c":   {Includes: parser.Includes{DoubleQuote: []sourceFile{"h1.h"}}},
				"s2.cpp": {Includes: parser.Includes{DoubleQuote: []sourceFile{"h1.h", "h2.h", "ext/header.h"}}},
				"s3.cc":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"h1.h", "h2.h", "h3.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"h1": {hdrs: []sourceFile{"h1.h"}, srcs: []sourceFile{"s1.c"}},
					"h2": {hdrs: []sourceFile{"h2.h"}, srcs: []sourceFile{"s2.cpp"}, dependsOn: []groupId{"h1"}},
					"h3": {hdrs: []sourceFile{"h3.h"}, srcs: []sourceFile{"s3.cc"}, dependsOn: []groupId{"h2"}},
				},
				unassigned: nil,
			},
		},
		{
			clue: "Header including an external system file should still form a group",
			input: sourceInfos{
				"lib.h":   {Includes: parser.Includes{Bracket: []sourceFile{"system.h"}}},
				"lib.cc":  {Includes: parser.Includes{DoubleQuote: []sourceFile{"lib.h"}}},
				"app.cpp": {Includes: parser.Includes{Bracket: []sourceFile{"system.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"lib": {hdrs: []sourceFile{"lib.h"}, srcs: []sourceFile{"lib.cc"}},
				},
				unassigned: []sourceFile{"app.cpp"},
			},
		},
		{
			clue: "Implementation of header should merge groups even if header does not",
			input: sourceInfos{
				"a.h":  {},
				"b.h":  {},
				"a.cc": {Includes: parser.Includes{DoubleQuote: []sourceFile{"b.h"}}},
				"b.cc": {Includes: parser.Includes{DoubleQuote: []sourceFile{"a.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {hdrs: []sourceFile{"a.h", "b.h"}, srcs: []sourceFile{"a.cc", "b.cc"}},
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
				"b.cc": {Includes: parser.Includes{DoubleQuote: []sourceFile{"a.h"}}},
			},
			expected: sourceGroups{
				groups: map[groupId]*sourceGroup{
					"a": {hdrs: []sourceFile{"a.h"}, srcs: []sourceFile{"a.cc"}},
					"b": {hdrs: []sourceFile{"b.h"}, srcs: []sourceFile{"b.cc"}, dependsOn: []groupId{"a"}},
				},
				unassigned: nil,
			},
		},
	}

	for idx, tc := range testCases {
		sourceFiles := make([]sourceFile, 0, len(tc.input))
		for k := range tc.input {
			sourceFiles = append(sourceFiles, k)
		}
		result := groupSourcesByHeaders(sourceFiles, tc.input)

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
