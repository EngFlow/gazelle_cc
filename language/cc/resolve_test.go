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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransformIncludePath(t *testing.T) {
	const libRel = "libs/my_lib"
	const hdrRel = "libs/my_lib/foo/bar.h"

	testCases := []struct {
		stripIncludePrefix string
		includePrefix      string
		expectedResult     string
	}{
		{
			stripIncludePrefix: "",
			includePrefix:      "",
			expectedResult:     "libs/my_lib/foo/bar.h",
		},
		{
			stripIncludePrefix: "",
			includePrefix:      "extra",
			expectedResult:     "extra/foo/bar.h",
		},
		{
			stripIncludePrefix: "/libs",
			includePrefix:      "",
			expectedResult:     "my_lib/foo/bar.h",
		},
		{
			stripIncludePrefix: "/libs",
			includePrefix:      "extra",
			expectedResult:     "extra/my_lib/foo/bar.h",
		},
		{
			stripIncludePrefix: "foo",
			includePrefix:      "",
			expectedResult:     "bar.h",
		},
		{
			stripIncludePrefix: "foo",
			includePrefix:      "extra",
			expectedResult:     "extra/bar.h",
		},
	}

	for _, tc := range testCases {
		result := transformIncludePath(libRel, tc.stripIncludePrefix, tc.includePrefix, hdrRel)
		assert.Equal(t, tc.expectedResult, result, "stripIncludePrefix=%q, includePrefix=%q", tc.stripIncludePrefix, tc.includePrefix)
	}
}
