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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMacros(t *testing.T) {
	type testCase struct {
		defs     []string
		expected Macros
	}

	validTestCases := []testCase{
		{
			defs: []string{"FOO"},
			expected: Macros{
				"FOO": 1,
			},
		},
		{
			defs: []string{"BAR=123", "BAZ=0x2AUL", "QUX=0755"},
			expected: Macros{
				"BAR": 123,
				"BAZ": 42,
				"QUX": 493,
			},
		},
		{
			defs: []string{"-D__ANDROID__", "-D__ARM_ARCH=8"},
			expected: Macros{
				"__ANDROID__": 1,
				"__ARM_ARCH":  8,
			},
		},
	}

	for _, tc := range validTestCases {
		got, err := ParseMacros(tc.defs)
		if err != nil {
			t.Fatalf("ParseMacros(%v) unexpected error: %v", tc.defs, err)
		}
		assert.Equal(t, tc.expected, got)
	}

	unparsableTestCases := []string{
		"FLT=3.14",       // float
		"STR=\"abc\"",    // string literal
		"CHR='A'",        // char literal
		"-DBAD-NAME=1",   // invalid identifier
		"SUFFIX=123XYZ",  // unknown suffix
		"HEXFLT=0x1.8p3", // hex-float
	}

	for _, def := range unparsableTestCases {
		if _, err := ParseMacro(def); err == nil {
			t.Errorf("ParseMacros(%v) expected error, got nil", def)
		}
	}
}
