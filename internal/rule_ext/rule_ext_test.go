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

package rule_ext

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttrBool(t *testing.T) {
	testCases := []struct {
		description   string
		input         []byte
		attr          string
		defaultValue  bool
		expectedValue bool
	}{
		{
			description:   "missing attribute returns default (true)",
			input:         []byte(`my_rule(name = "foo")`),
			attr:          "missing",
			defaultValue:  true,
			expectedValue: true,
		},
		{
			description:   "missing attribute returns default (false)",
			input:         []byte(`my_rule(name = "foo")`),
			attr:          "missing",
			defaultValue:  false,
			expectedValue: false,
		},
		{
			description:   "True literal returns true with default false",
			input:         []byte(`my_rule(name = "foo", flag = True)`),
			attr:          "flag",
			defaultValue:  false,
			expectedValue: true,
		},
		{
			description:   "True literal returns true with default true",
			input:         []byte(`my_rule(name = "foo", flag = True)`),
			attr:          "flag",
			defaultValue:  true,
			expectedValue: true,
		},
		{
			description:   "False literal returns false with default true",
			input:         []byte(`my_rule(name = "foo", flag = False)`),
			attr:          "flag",
			defaultValue:  true,
			expectedValue: false,
		},
		{
			description:   "False literal returns false with default false",
			input:         []byte(`my_rule(name = "foo", flag = False)`),
			attr:          "flag",
			defaultValue:  false,
			expectedValue: false,
		},
		{
			description:   "non-bool attribute returns default (true)",
			input:         []byte(`my_rule(name = "bar")`),
			attr:          "name",
			defaultValue:  true,
			expectedValue: true,
		},
		{
			description:   "non-bool attribute returns default (false)",
			input:         []byte(`my_rule(name = "bar")`),
			attr:          "name",
			defaultValue:  false,
			expectedValue: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			f, err := rule.LoadData("", "", tc.input)
			require.NoError(t, err)
			require.Len(t, f.Rules, 1)
			got := AttrBool(f.Rules[0], tc.attr, tc.defaultValue)
			assert.Equal(t, tc.expectedValue, got)
		})
	}
}
