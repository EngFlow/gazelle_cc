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

package cc

import (
	"strings"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/buildtools/build"
	"github.com/stretchr/testify/assert"
)

func TestPlatformDepsBuilderBuild(t *testing.T) {
	lib_a := label.New("", "pkg", "lib_a")
	lib_b := label.New("", "pkg", "lib_b")
	linuxConstraint := label.New("", "platforms", "linux")

	testCases := []struct {
		description  string
		builder      func() platformDepsBuilder
		expectedExpr string
	}{
		{
			description: "only_default",
			builder: func() platformDepsBuilder {
				builder := newPlatformDepsBuilder()
				builder.addConstrained(defaultCondition, lib_a)
				return builder
			},
			expectedExpr: `
["//pkg:lib_a"]
			`,
		},
		{
			description: "generic_only",
			builder: func() platformDepsBuilder {
				builder := newPlatformDepsBuilder()
				builder.addGeneric(lib_a)
				return builder
			},
			expectedExpr: `
["//pkg:lib_a"]
			`,
		},
		{
			description: "generic_and_default",
			builder: func() platformDepsBuilder {
				builder := newPlatformDepsBuilder()
				builder.addGeneric(lib_a)
				builder.addConstrained(defaultCondition, lib_b)
				return builder
			},
			expectedExpr: `
[
    "//pkg:lib_a",
    "//pkg:lib_b",
]
			`,
		},
		{
			description: "generic_and_constrained_dedup",
			builder: func() platformDepsBuilder {
				builder := newPlatformDepsBuilder()
				builder.addGeneric(lib_a)
				builder.addConstrained(linuxConstraint, lib_a)
				return builder
			},
			expectedExpr: `
["//pkg:lib_a"]
			`,
		},
		{
			description: "generic_and_constrained_no_overlap",
			builder: func() platformDepsBuilder {
				builder := newPlatformDepsBuilder()
				builder.addGeneric(lib_a)
				builder.addConstrained(linuxConstraint, lib_b)
				return builder
			},
			expectedExpr: `
[
    "//pkg:lib_a",
] + select({
    "//platforms:linux": [
        "//pkg:lib_b",
    ],
    "//conditions:default": [],
})
			`,
		},
		{
			description: "constrained_only",
			builder: func() platformDepsBuilder {
				builder := newPlatformDepsBuilder()
				builder.addConstrained(linuxConstraint, lib_a)
				return builder
			},
			expectedExpr: `
select({
    "//platforms:linux": [
        "//pkg:lib_a",
    ],
    "//conditions:default": [],
})
			`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			builder := tc.builder()
			expr := builder.build().BzlExpr()
			actual := build.FormatString(expr)

			// Remove leading newline for readability
			expected := strings.TrimSpace(tc.expectedExpr)

			assert.Equal(t, expected, actual)
		})
	}
}
