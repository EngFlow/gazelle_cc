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

package index

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/stretchr/testify/assert"
)

func TestParseUniqueDependencyIndex(t *testing.T) {
	testCases := []struct {
		input         string
		expected      UniqueDependencyIndex
		expectedError string
	}{
		{
			input: `{"header1.h": "@repo1//pkg1:target1", "header2.h": "@repo2//pkg2:target2"}`,
			expected: UniqueDependencyIndex{
				"header1.h": Label{label.New("repo1", "pkg1", "target1")},
				"header2.h": Label{label.New("repo2", "pkg2", "target2")},
			},
		},
		{
			input:         `{"header.h": ":invalid:label"}`,
			expectedError: `label parse error: name has invalid characters: ":invalid:label"`,
		},
		{
			input:         `{"header.h": "@repo//:missing_brace"`,
			expectedError: "unexpected end of JSON input",
		},
	}

	for _, tc := range testCases {
		result, err := ParseUniqueDependencyIndex([]byte(tc.input))
		if tc.expectedError == "" {
			assert.NoError(t, err, "input: %s", tc.input)
			assert.Equal(t, tc.expected, result, "input: %s", tc.input)
		} else {
			assert.EqualError(t, err, tc.expectedError, "input: %s", tc.input)
		}
	}
}

func TestParseFullDependencyIndex(t *testing.T) {
	testCases := []struct {
		input         string
		expected      FullDependencyIndex
		expectedError string
	}{
		{
			input: `{
				"unique": {
					"header1.h": "@repo//pkg:target"
				},
				"ambiguous_within_module": {
					"header2.h": ["@repo//pkg:target_a", "@repo//pkg:target_b"]
				},
				"ambiguous_across_modules": {
					"header3.h": ["@repo_a//pkg:target", "@repo_b//pkg:target"]
				}
			}`,
			expected: FullDependencyIndex{
				Unique: UniqueDependencyIndex{
					"header1.h": Label{label.New("repo", "pkg", "target")},
				},
				AmbiguousWithinModule: AmbiguousDependencyIndex{
					"header2.h": AmbiguousTargets{
						Label{label.New("repo", "pkg", "target_a")},
						Label{label.New("repo", "pkg", "target_b")},
					},
				},
				AmbiguousAcrossModules: AmbiguousDependencyIndex{
					"header3.h": AmbiguousTargets{
						Label{label.New("repo_a", "pkg", "target")},
						Label{label.New("repo_b", "pkg", "target")},
					},
				},
			},
		},
		{
			input: `{
				"unique": {
					"header1.h": "@repo1//pkg1:target1",
					"header2.h": "@repo2//pkg2:target2"
				},
				"ambiguous_within_module": {},
				"ambiguous_across_modules": {}
			}`,
			expected: FullDependencyIndex{
				Unique: UniqueDependencyIndex{
					"header1.h": Label{label.New("repo1", "pkg1", "target1")},
					"header2.h": Label{label.New("repo2", "pkg2", "target2")},
				},
				AmbiguousWithinModule:  AmbiguousDependencyIndex{},
				AmbiguousAcrossModules: AmbiguousDependencyIndex{},
			},
		},
		{
			input: `{
				"unique": {},
				"ambiguous_within_module": {},
				"ambiguous_across_modules": {},
				"extra_field": {}
			}`,
			expected: FullDependencyIndex{
				Unique:                 UniqueDependencyIndex{},
				AmbiguousWithinModule:  AmbiguousDependencyIndex{},
				AmbiguousAcrossModules: AmbiguousDependencyIndex{},
			},
		},
		{
			input: "{}",
			expected: FullDependencyIndex{
				Unique:                 nil,
				AmbiguousWithinModule:  nil,
				AmbiguousAcrossModules: nil,
			},
		},
		{
			input: `{
				"unique": {
					"same_header.h": "@repo//pkg:target"
				},
				"ambiguous_within_module": {
					"same_header.h": ["@repo//pkg:target_a", "@repo//pkg:target_b"]
				}
			}`,
			expectedError: "header present in multiple sections: [same_header.h]",
		},
		{
			input: `{
				"unique": {},
				"ambiguous_within_module": {
					"header.h": ["@repo//pkg:target"]
				}
			}`,
			expectedError: "ambiguous targets must contain at least 2 elements, got 1",
		},
		{
			input: `{
				"unique": {},
				"ambiguous_across_modules": {
					"header.h": ["@repo//pkg:target"]
				}
			}`,
			expectedError: "ambiguous targets must contain at least 2 elements, got 1",
		},
		{
			input: `{
				"unique": {},
				"ambiguous_within_module": {
					"header.h": ["@repo//pkg:target", "@repo//pkg:target"]
				}
			}`,
			expectedError: "duplicate targets in list [@repo//pkg:target @repo//pkg:target]: [@repo//pkg:target]",
		},
		{
			input: `{
				"unique": {},
				"ambiguous_across_modules": {
					"header.h": ["@repo//pkg:target", "@repo//pkg:target"]
				}
			}`,
			expectedError: "duplicate targets in list [@repo//pkg:target @repo//pkg:target]: [@repo//pkg:target]",
		},
		{
			input: `{
				"unique": {},
				"ambiguous_within_module": {
					"header.h": ["@repo1//pkg1:target1", "@repo2//pkg2:target2"]
				}
			}`,
			expectedError: "should share same repo in list [@repo1//pkg1:target1 @repo2//pkg2:target2]",
		},
		{
			input: `{
				"unique": {},
				"ambiguous_across_modules": {
					"header.h": ["@repo//pkg1:target1", "@repo//pkg2:target2"]
				}
			}`,
			expectedError: "should span multiple repos in list [@repo//pkg1:target1 @repo//pkg2:target2]",
		},
	}

	for _, tc := range testCases {
		result, err := ParseFullDependencyIndex([]byte(tc.input))
		if tc.expectedError == "" {
			assert.NoError(t, err, "input: %s", tc.input)
			assert.Equal(t, tc.expected, result, "input: %s", tc.input)
		} else {
			assert.EqualError(t, err, tc.expectedError, "input: %s", tc.input)
		}
	}
}

func TestEncodeFullDependencyIndex(t *testing.T) {
	testCases := []struct {
		input    FullDependencyIndex
		expected string
	}{
		{
			input: FullDependencyIndex{
				Unique: UniqueDependencyIndex{
					"header1.h": Label{label.New("repo", "pkg", "target")},
				},
				AmbiguousWithinModule: AmbiguousDependencyIndex{
					"header2.h": AmbiguousTargets{
						Label{label.New("repo", "pkg", "target_a")},
						Label{label.New("repo", "pkg", "target_b")},
					},
				},
				AmbiguousAcrossModules: AmbiguousDependencyIndex{
					"header3.h": AmbiguousTargets{
						Label{label.New("repo_a", "pkg", "target")},
						Label{label.New("repo_b", "pkg", "target")},
					},
				},
			},
			expected: `{
  "unique": {
    "header1.h": "@repo//pkg:target"
  },
  "ambiguous_within_module": {
    "header2.h": [
      "@repo//pkg:target_a",
      "@repo//pkg:target_b"
    ]
  },
  "ambiguous_across_modules": {
    "header3.h": [
      "@repo_a//pkg:target",
      "@repo_b//pkg:target"
    ]
  }
}`,
		},
		{
			input: FullDependencyIndex{
				Unique:                 UniqueDependencyIndex{},
				AmbiguousWithinModule:  AmbiguousDependencyIndex{},
				AmbiguousAcrossModules: AmbiguousDependencyIndex{},
			},
			expected: `{
  "unique": {},
  "ambiguous_within_module": {},
  "ambiguous_across_modules": {}
}`,
		},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expected, string(tc.input.Encode()))
	}
}
