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

package parser

import (
	"fmt"
	"testing"
)

func TestParseIncludes(t *testing.T) {
	testCases := []struct {
		input    string
		expected []Include
	}{
		// Parses valid source code
		{
			input: `
#include <stdio.h>
#include "myheader.h"
#include <math.h>
`,
			expected: []Include{
				{Path: "stdio.h", IsSystemInclude: true},
				{Path: "myheader.h"},
				{Path: "math.h", IsSystemInclude: true},
			},
		},
		{
			// Accept malformed include
			input: `
#include "stdio.h
#include stdlib.h"
#include <math.h
#include exception>
`,
			expected: []Include{
				{Path: "stdio.h"},
				{Path: "stdlib.h"},
				{Path: "math.h", IsSystemInclude: true},
				{Path: "exception", IsSystemInclude: true},
			},
		},
	}

	for _, tc := range testCases {
		result := ParseSource(tc.input).Includes
		if fmt.Sprintf("%v", result) != fmt.Sprintf("%v", tc.expected) {
			t.Errorf("For input: %q, expected %+v, but got %+v", tc.input, tc.expected, result)
		}
	}
}

func TestParseSourceHasMain(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{
			expected: true,
			input:    " int main(){return 0;}"},
		{
			expected: true,
			input:    "int main(int argc, char *argv) { return 0; }",
		},
		{
			expected: true,
			input: `
				void my_function() {  // Not main
						int x = 5;
				}

				int main() {
						return 0;
				}
			}`,
		},
		{
			expected: true,
			input: `
			 int main(void) {
			 		return 0;
			 }
			 `,
		},
		{
			expected: true,
			input: `
			int main(  ) {
					return 0;
			}`,
		},
		{
			expected: true,
			input: ` int main(
			) {
					return 0;
			}
			`,
		},
		{
			expected: true,
			input: `
			int main   (  ) {
					return 0;
			}`,
		},
		{
			expected: true,
			input: `
			int main   (
			) {
					return 0;
			}`,
		},
		{
			expected: false,
			input:    `// int main(int argc, char** argv){return 0;}`,
		},
		{
			expected: false,
			input: `
			/*
			  int main(int argc, char** argv){return 0;}
			*/
			`,
		},
		{
			expected: true,
			input:    `/* that our main */ int main(int argCount, char** values){return 0;}`,
		},
	}

	for idx, tc := range testCases {
		result := ParseSource(tc.input).HasMain
		if fmt.Sprintf("%v", result) != fmt.Sprintf("%v", tc.expected) {
			t.Errorf("For test case %d input: %q, expected %+v, but got %+v", idx, tc.input, tc.expected, result)
		}
	}
}
