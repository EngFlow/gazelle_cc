// Copyright 2026 EngFlow Inc. All rights reserved.
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

package main

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/stretchr/testify/assert"
)

func TestExposedLabel(t *testing.T) {
	tests := []struct {
		name string

		target  string // Target being considered.
		private bool   // If target is private (public by default for tests).
		alias   string // Public alias re-exporting target, if any.

		expectedExposedAs string // Label to index target under, omit for target's own name.
		expectedIndexable bool
	}{
		{
			name:              "public target without an alias keeps its name",
			target:            "@repo//pkg:lib",
			expectedIndexable: true,
		},
		{
			name:              "private target without an alias is not indexed",
			target:            "@repo//pkg:lib",
			private:           true,
			expectedIndexable: false,
		},
		{
			name:              "private target with an alias is indexed under that alias",
			target:            "@repo//pkg:lib",
			private:           true,
			alias:             "@repo//other/much/longer:lib",
			expectedExposedAs: "@repo//other/much/longer:lib",
			expectedIndexable: true,
		},
		{
			name:              "repository-named alias wins over a shorter name",
			target:            "@repo//:a",
			alias:             "@repo//:repo",
			expectedExposedAs: "@repo//:repo",
			expectedIndexable: true,
		},
		{
			name:              "repository-named name wins over a shorter alias",
			target:            "@repo//:repo",
			alias:             "@repo//:a",
			expectedIndexable: true,
		},
		{
			// `@repo//:repo` is the shortest label the repository has, so only
			// the repository-named rule can pick it here: the target is itself
			// named after the repository, which stops the shorter-label rule.
			name:              "repository-named alias wins for a repository-named target",
			target:            "@repo//src:repo",
			alias:             "@repo//:repo",
			expectedExposedAs: "@repo//:repo",
			expectedIndexable: true,
		},
		{
			// Same target, but an alias that is merely shorter does not win.
			name:              "repository-named target is not demoted to a shorter alias",
			target:            "@repo//src:repo",
			alias:             "@repo//:a",
			expectedIndexable: true,
		},
		{
			name:              "shorter alias is preferred, even across packages",
			target:            "@repo//src/google/protobuf/json:json",
			alias:             "@repo//:json",
			expectedExposedAs: "@repo//:json",
			expectedIndexable: true,
		},
		{
			name:              "longer alias is not preferred",
			target:            "@repo//:lib",
			alias:             "@repo//pkg:a", // Short name, but long path.
			expectedIndexable: true,
		},
		{
			name:              "target wins over alias by default",
			target:            "@repo//:b",
			alias:             "@repo//:a", // Before `target` lexicographically but not picked, in contrast to `preferFirstAlias()`.
			expectedIndexable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := mustParseLabel(t, tt.target)
			aliases := map[label.Label]label.Label{}
			if tt.alias != "" {
				aliases[target] = mustParseLabel(t, tt.alias)
			}

			expectedExposedAs := label.NoLabel
			if tt.expectedExposedAs != "" {
				expectedExposedAs = mustParseLabel(t, tt.expectedExposedAs)
			}

			exposedAs, indexable := exposedLabel(target, !tt.private, aliases)
			assert.Equal(t, expectedExposedAs, exposedAs)
			assert.Equal(t, tt.expectedIndexable, indexable)
		})
	}
}

func TestPreferFirstAlias(t *testing.T) {
	tests := []struct {
		name   string
		prefer string
		over   string
	}{
		{
			name:   "repository-named alias beats a shorter one",
			prefer: "@repo//:repo",
			over:   "@repo//:r",
		},
		{
			name:   "shorter alias wins when neither is named after its repository",
			prefer: "@repo//:a",
			over:   "@repo//:bb",
		},
		{
			name:   "shorter alias beats a package-named one",
			prefer: "@repo//:t",
			over:   "@repo//absl/time:time",
		},
		{
			name:   "shorter alias wins between two package-named ones",
			prefer: "@repo//a:a",
			over:   "@repo//deeper/b:b",
		},
		{
			name:   "equally good aliases are ordered lexicographically",
			prefer: "@repo//:aa",
			over:   "@repo//:bb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefer := mustParseLabel(t, tt.prefer)
			over := mustParseLabel(t, tt.over)

			assert.True(t, preferFirstAlias(prefer, over),
				"expected %v to be preferred over %v", prefer, over)

			// Try it the other way around, as the order shouldn't matter.
			assert.False(t, preferFirstAlias(over, prefer),
				"expected %v not to be preferred over %v", over, prefer)
		})
	}
}

// mustParseLabel parses an absolute label, failing the test if it cannot.
func mustParseLabel(t *testing.T, raw string) label.Label {
	t.Helper()
	parsed, err := label.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse %q: %v", raw, err)
	}
	return parsed
}
