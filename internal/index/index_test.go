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
	"encoding/json"
	"testing"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/stretchr/testify/assert"
)

func TestMarshalJSON(t *testing.T) {
	input := DependencyIndex{
		"header1.h": {
			label.New("repo", "pkg", "target"),
		},
		"header2.h": {
			label.New("repo", "pkg", "target_a"),
			label.New("repo", "pkg", "target_b"),
		},
		"header3.h": {
			label.New("repo_a", "pkg", "target"),
			label.New("repo_b", "pkg", "target"),
		},
	}
	expected := `{
	"header1.h": "@repo//pkg:target",
	"header2.h": [
		"@repo//pkg:target_a",
		"@repo//pkg:target_b"
	],
	"header3.h": [
		"@repo_a//pkg:target",
		"@repo_b//pkg:target"
	]
}`
	result, err := json.MarshalIndent(input, "", "\t")
	assert.Equal(t, expected, string(result))
	assert.NoError(t, err)
}

func TestUnmarshalJSON(t *testing.T) {
	testCases := []struct {
		input         string
		expected      DependencyIndex
		expectedError string
	}{
		{
			input: `{
 				"header1.h": "@repo//pkg:target",
 				"header2.h": ["@repo//pkg:target_a", "@repo//pkg:target_b"],
 				"header3.h": ["@repo_a//pkg:target", "@repo_b//pkg:target"]
			}`,
			expected: DependencyIndex{
				"header1.h": {
					label.New("repo", "pkg", "target"),
				},
				"header2.h": {
					label.New("repo", "pkg", "target_a"),
					label.New("repo", "pkg", "target_b"),
				},
				"header3.h": {
					label.New("repo_a", "pkg", "target"),
					label.New("repo_b", "pkg", "target"),
				},
			},
		},
		{
			input:         `{"header.h": ":invalid:label"}`,
			expectedError: `"header.h": label parse error: name has invalid characters: ":invalid:label"`,
		},
		{
			input:         `{"header.h": "@repo//:missing_brace"`,
			expectedError: "unexpected end of JSON input",
		},
		{
			input:         `{"header.h": 12345}`,
			expectedError: `"header.h": invalid JSON type: float64`,
		},
	}

	for _, tc := range testCases {
		var result DependencyIndex
		var err error
		err = json.Unmarshal([]byte(tc.input), &result)
		if tc.expectedError == "" {
			assert.NoError(t, err, "input: %s", tc.input)
			assert.Equal(t, tc.expected, result, "input: %s", tc.input)
		} else {
			assert.EqualError(t, err, tc.expectedError, "input: %s", tc.input)
		}
	}
}

func TestMarshalUnmarshalJSON(t *testing.T) {
	input := DependencyIndex{
		"header1.h": {
			label.New("repo", "pkg", "target"),
		},
		"header2.h": {
			label.New("repo", "pkg", "target_a"),
			label.New("repo", "pkg", "target_b"),
		},
		"header3.h": {
			label.New("repo_a", "pkg", "target"),
			label.New("repo_b", "pkg", "target"),
		},
	}

	jsonData, err := json.Marshal(input)
	assert.NoError(t, err)

	var output DependencyIndex
	err = json.Unmarshal(jsonData, &output)
	assert.NoError(t, err)

	assert.Equal(t, input, output)
}
