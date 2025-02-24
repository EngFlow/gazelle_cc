package cpp

import (
	"fmt"
	"maps"
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
				"orphan": {sources: []sourceFile{"orphan.cc"}},
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
				"a": {sources: []sourceFile{"a.h"}},
				"b": {sources: []sourceFile{"b.h"}, dependsOn: []groupId{"a"}},
				"c": {sources: []sourceFile{"c.h"}, dependsOn: []groupId{"b"}},
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
				"a": {sources: []sourceFile{"a.c", "a.h"}},
				"b": {sources: []sourceFile{"b.cc", "b.h"}},
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
				"a": {sources: []sourceFile{"a.c", "a.h", "b.cc", "b.h"}},
				"c": {sources: []sourceFile{"c.h"}, dependsOn: []groupId{"a"}},
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
				"p": {sources: []sourceFile{"p.h", "q.h", "r.h"}},
			},
		},
		{
			clue: "A source file that includes multiple unrelated headers should assigned to it's own group",
			input: sourceInfos{
				"m.h":      {},
				"n.h":      {},
				"o.h":      {},
				"file.cpp": {Includes: parser.Includes{DoubleQuote: []string{"m.h", "n.h", "o.h"}}},
			},
			expected: sourceGroups{
				"m":    {sources: []sourceFile{"m.h"}},
				"n":    {sources: []sourceFile{"n.h"}},
				"o":    {sources: []sourceFile{"o.h"}},
				"file": {sources: []sourceFile{"file.cpp"}, dependsOn: []groupId{"m", "n", "o"}},
			},
		},

		{
			clue: "Correctly group mixed dependencies",
			input: sourceInfos{
				"a.h":  {},
				"b.h":  {Includes: parser.Includes{DoubleQuote: []string{"a.h"}}},
				"c.h":  {},
				"d.h":  {Includes: parser.Includes{DoubleQuote: []string{"c.h"}}},
				"e.h":  {Includes: parser.Includes{DoubleQuote: []string{"d.h", "f1.h", "f2.h"}}},
				"f1.h": {Includes: parser.Includes{DoubleQuote: []string{"e.h"}}},
				"f2.h": {Includes: parser.Includes{DoubleQuote: []string{"e.h"}}},
				"g.h":  {Includes: parser.Includes{DoubleQuote: []string{"b.h", "d.h"}}},
				"h.h":  {Includes: parser.Includes{DoubleQuote: []string{"g.h"}}},
				"i.h":  {Includes: parser.Includes{DoubleQuote: []string{"g.h"}}},
				"j.h":  {Includes: parser.Includes{DoubleQuote: []string{"h.h", "i.h"}}},
			},
			expected: sourceGroups{
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
		},
		{
			clue: "Header including an external include file should still form a group",
			input: sourceInfos{
				"lib.h":   {Includes: parser.Includes{Bracket: []string{"system.h"}}},
				"lib.cc":  {Includes: parser.Includes{DoubleQuote: []string{"lib.h"}}},
				"app.cpp": {Includes: parser.Includes{Bracket: []string{"system.h"}}},
			},
			expected: sourceGroups{
				"lib": {sources: []sourceFile{"lib.cc", "lib.h"}},
				"app": {sources: []sourceFile{"app.cpp"}},
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
				"a": {sources: []sourceFile{"a.cc", "a.h", "b.cc", "b.h"}},
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
				"a": {sources: []sourceFile{"a.cc", "a.h"}},
				"b": {sources: []sourceFile{"b.cc", "b.h"}, dependsOn: []groupId{"a"}},
			},
		},
	}

	for idx, tc := range testCases {
		result := groupSourcesByHeaders(
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
