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
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSourceGroups(t *testing.T) {
	testCases := []struct {
		desc               string
		rel                string
		stripIncludePrefix string
		includePrefix      string
		ccSearch           []ccSearch
		input              []fileInfo
		expected           []sourceGroupSummary
	}{
		{
			desc:  "A source file with no includes should be unassigned",
			input: []fileInfo{fileInfoForTest("orphan.cc")},
			expected: []sourceGroupSummary{
				{id: "orphan", sources: []string{"orphan.cc"}},
			},
		},
		{
			desc: "Each header should form its own group even if it includes another",
			input: []fileInfo{
				fileInfoForTest("a.h"),
				fileInfoForTest("b.h", "a.h"),
				fileInfoForTest("c.h", "b.h"),
			},
			expected: []sourceGroupSummary{
				{id: "a", sources: []string{"a.h"}},
				{id: "b", sources: []string{"b.h"}},
				{id: "c", sources: []string{"c.h"}},
			},
		},
		{
			desc: "Group source with header even when not included",
			input: []fileInfo{
				fileInfoForTest("a.h"),
				fileInfoForTest("a.c"),
				fileInfoForTest("b.cc"),
				fileInfoForTest("b.h"),
			},
			expected: []sourceGroupSummary{
				{id: "a", sources: []string{"a.c", "a.h"}},
				{id: "b", sources: []string{"b.cc", "b.h"}},
			},
		},
		{
			desc: "Merge cyclic dependency sources",
			input: []fileInfo{
				fileInfoForTest("a.h", "b.h"),
				fileInfoForTest("a.c", "a.h"),
				fileInfoForTest("b.h", "a.h"),
				fileInfoForTest("b.cc", "b.h"),
				fileInfoForTest("c.h", "a.h"),
			},
			expected: []sourceGroupSummary{
				{id: "a", sources: []string{"a.c", "a.h", "b.cc", "b.h"}},
				{id: "c", sources: []string{"c.h"}},
			},
		},
		{
			desc: "Detect implementation based cycle",
			input: []fileInfo{
				fileInfoForTest("a.h"),
				fileInfoForTest("a.c", "b.h"),
				fileInfoForTest("b.h"),
				fileInfoForTest("b.cc", "a.h"),
			},
			expected: []sourceGroupSummary{
				{id: "a", sources: []string{"a.c", "a.h", "b.cc", "b.h"}},
			},
		},
		{
			desc: "Handle cyclic dependencies among headers correctly",
			input: []fileInfo{
				fileInfoForTest("p.h", "q.h"),
				fileInfoForTest("q.h", "r.h"),
				fileInfoForTest("r.h", "p.h"),
			},
			expected: []sourceGroupSummary{
				{id: "p", sources: []string{"p.h", "q.h", "r.h"}},
			},
		},
		{
			desc: "A source file that includes multiple unrelated headers should assigned to it's own group",
			input: []fileInfo{
				fileInfoForTest("m.h"),
				fileInfoForTest("n.h"),
				fileInfoForTest("o.h"),
				fileInfoForTest("file.cpp", "m.h", "n.h", "o.h"),
			},
			expected: []sourceGroupSummary{
				{id: "file", sources: []string{"file.cpp"}},
				{id: "m", sources: []string{"m.h"}},
				{id: "n", sources: []string{"n.h"}},
				{id: "o", sources: []string{"o.h"}},
			},
		},
		{
			desc: "Correctly group mixed dependencies",
			input: []fileInfo{
				fileInfoForTest("a.h"),
				fileInfoForTest("b.h", "a.h"),
				fileInfoForTest("c.h"),
				fileInfoForTest("d.h", "c.h"),
				fileInfoForTest("e.h", "d.h", "f1.h", "f2.h"),
				fileInfoForTest("f1.h", "e.h"),
				fileInfoForTest("f2.h", "e.h"),
				fileInfoForTest("g.h", "b.h", "d.h"),
				fileInfoForTest("h.h", "g.h"),
				fileInfoForTest("i.h", "g.h"),
				fileInfoForTest("j.h", "h.h", "i.h"),
			},
			expected: []sourceGroupSummary{
				{id: "a", sources: []string{"a.h"}},
				{id: "b", sources: []string{"b.h"}},
				{id: "c", sources: []string{"c.h"}},
				{id: "d", sources: []string{"d.h"}},
				{id: "e", sources: []string{"e.h", "f1.h", "f2.h"}},
				{id: "g", sources: []string{"g.h"}},
				{id: "h", sources: []string{"h.h"}},
				{id: "i", sources: []string{"i.h"}},
				{id: "j", sources: []string{"j.h"}},
			},
		},
		{
			desc: "Header including an external include file should still form a group",
			input: []fileInfo{
				{
					name:     "lib.h",
					kind:     libSrcKind,
					includes: []ccInclude{{path: "system.h", isSystemInclude: true}},
				},
				fileInfoForTest("lib.cc", "lib.h"),
				{
					name:     "app.cpp",
					kind:     libSrcKind,
					includes: []ccInclude{{path: "system.h", isSystemInclude: true}},
				},
			},
			expected: []sourceGroupSummary{
				{id: "app", sources: []string{"app.cpp"}},
				{id: "lib", sources: []string{"lib.cc", "lib.h"}},
			},
		},
		{
			desc: "Implementation of header should merge groups even if header does not",
			input: []fileInfo{
				fileInfoForTest("a.h"),
				fileInfoForTest("b.h"),
				fileInfoForTest("a.cc", "b.h"),
				fileInfoForTest("b.cc", "a.h"),
			},
			expected: []sourceGroupSummary{
				{id: "a", sources: []string{"a.cc", "a.h", "b.cc", "b.h"}},
			},
		},
		{
			desc: "Implementation of header does not merge if can define dependency",
			input: []fileInfo{
				fileInfoForTest("a.h"),
				fileInfoForTest("a.cc"),
				fileInfoForTest("b.h"),
				fileInfoForTest("b.cc", "a.h"),
			},
			expected: []sourceGroupSummary{
				{id: "a", sources: []string{"a.cc", "a.h"}},
				{id: "b", sources: []string{"b.cc", "b.h"}},
			},
		},
		{
			desc:               "Include prefix applies to full include paths",
			rel:                "src",
			stripIncludePrefix: "/src",
			includePrefix:      "foo",
			input: []fileInfo{
				fileInfoForTest("a.h"),
				fileInfoForTest("a.cc", "foo/b.h"),
				fileInfoForTest("b.h"),
				fileInfoForTest("b.cc", "src/a.h"), // full untransformed path still valid
			},
			expected: []sourceGroupSummary{
				{id: "a", sources: []string{"a.cc", "a.h", "b.cc", "b.h"}},
			},
		},
		{
			desc: "cc_search applies to full include paths",
			rel:  "src",
			ccSearch: []ccSearch{
				{stripIncludePrefix: "/src/include", includePrefix: ""},
			},
			input: []fileInfo{
				fileInfoForTest("a.h"),
				fileInfoForTest("a.cc", "foo/b.h", "src/include/a.h"),
				fileInfoForTest("b.h"),
				fileInfoForTest("b.cc", "src/include/a.h", "src/include/b.h"),
			},
			expected: []sourceGroupSummary{
				{id: "a", sources: []string{"a.cc", "a.h"}},
				{id: "b", sources: []string{"b.cc", "b.h"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := summarizeSourceGroups(groupSourcesByUnits(tc.rel, tc.stripIncludePrefix, tc.includePrefix, tc.ccSearch, tc.input))
			assert.Equal(t, tc.expected, actual)
		})
	}
}

type sourceGroupSummary struct {
	id      groupId
	sources []string
}

func summarizeSourceGroups(groups sourceGroups) []sourceGroupSummary {
	summaries := make([]sourceGroupSummary, 0, len(groups))
	for id, group := range groups {
		summaries = append(summaries, sourceGroupSummary{
			id:      id,
			sources: toRelativePaths(group.sources),
		})
	}
	slices.SortFunc(summaries, func(a, b sourceGroupSummary) int {
		return strings.Compare(string(a.id), string(b.id))
	})
	return summaries
}

func fileInfoForTest(name string, includePaths ...string) fileInfo {
	includes := make([]ccInclude, len(includePaths))
	for i, path := range includePaths {
		includes[i] = ccInclude{path: path}
	}
	return fileInfo{
		name:     name,
		kind:     libSrcKind,
		includes: includes,
	}
}
