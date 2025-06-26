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

	"github.com/stretchr/testify/assert"
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
		result, err := ParseSource(tc.input)
		if err != nil {
			t.Errorf("Failed to parse %q, reason: %v", tc.input, err)
		}
		includes := result.Includes
		if fmt.Sprintf("%v", includes) != fmt.Sprintf("%v", tc.expected) {
			t.Errorf("For input: %q, expected %+v, but got %+v", tc.input, tc.expected, includes)
		}
	}
}

func TestParseConditionalIncludes(t *testing.T) {
	testCases := []struct {
		input    string
		expected SourceInfo
	}{
		// ifdef syntax
		{
			input: `
#include "common.h"
#ifdef _WIN32
#include <windows.h>
#elifdef \ 
	__APPLE__
#include <unistd.h>
#elifndef __linux__
#include <fcntl.h>
#else
#include "other.h"
#endif
#include "last.h"
`,
			expected: SourceInfo{
				Includes: []Include{
					{Path: "common.h"},
					{Path: "windows.h", IsSystemInclude: true, Condition: Defined{Ident("_WIN32")}},
					{Path: "unistd.h", IsSystemInclude: true, Condition: And{
						Defined{Ident("__APPLE__")},
						Not{Defined{Ident("_WIN32")}},
					}},
					{Path: "fcntl.h", IsSystemInclude: true, Condition: And{
						Not{Defined{Ident("__linux__")}},
						Not{Or{Defined{Ident("_WIN32")}, Defined{Ident("__APPLE__")}}},
					}},
					{Path: "other.h", Condition: Not{
						Or{
							Or{
								Defined{Ident("_WIN32")},
								Defined{Ident("__APPLE__")},
							},
							Not{Defined{Ident("__linux__")}},
						}}},
					{Path: "last.h"},
				},
			},
		},
		// if defined syntax
		{
			input: `
#if defined _WIN32
#include "windows.h"
#elif defined ( __APPLE__ )
#include "unistd.h"
#elif ! \
	defined(\
	__linux__)
#include "fcntl.h"
#else 
#include "other.h"
#endif
`,
			expected: SourceInfo{
				Includes: []Include{
					{Path: "windows.h", Condition: Defined{Ident("_WIN32")}},
					{Path: "unistd.h", Condition: And{
						Defined{Ident("__APPLE__")},
						Not{Defined{Ident("_WIN32")}},
					}},
					{Path: "fcntl.h", Condition: And{
						Not{Defined{Ident("__linux__")}},
						Not{Or{Defined{Ident("_WIN32")}, Defined{Ident("__APPLE__")}}},
					}},
					{Path: "other.h", Condition: Not{
						Or{
							Or{
								Defined{Ident("_WIN32")},
								Defined{Ident("__APPLE__")},
							},
							Not{Defined{Ident("__linux__")}},
						}}},
				},
			},
		},
		{
			// complex boolean expression
			input: `
#if (defined(_WIN32) && defined(ENABLE_GUI)) || defined(__ANDROID__)
#include "ui.h"
#elif defined(_WIN32)
#include "cli.h"
#endif
`,
			expected: SourceInfo{
				Includes: []Include{
					{
						Path: "ui.h",
						Condition: Or{
							And{
								Defined{Name: "_WIN32"},
								Defined{Name: "ENABLE_GUI"},
							},
							Defined{Name: "__ANDROID__"},
						},
					},
					{
						Path: "cli.h",
						Condition: And{
							Defined{Name: "_WIN32"},
							Not{
								Or{
									And{
										Defined{Name: "_WIN32"},
										Defined{Name: "ENABLE_GUI"},
									},
									Defined{Name: "__ANDROID__"},
								},
							},
						},
					},
				},
			},
		},
		{
			// multiline directive with continuations
			input: `
#if defined(_WIN32) && \
    !defined(DISABLE_FEATURE) || \
    (defined(__APPLE__) && defined(ENABLE_COCOA))
#include "feature.h"
#else
#include "nofeature.h"
#endif
`,
			expected: SourceInfo{
				Includes: []Include{
					{
						Path: "feature.h",
						Condition: Or{
							And{
								Defined{Name: "_WIN32"},
								Not{Defined{Name: "DISABLE_FEATURE"}},
							},
							And{
								Defined{Name: "__APPLE__"},
								Defined{Name: "ENABLE_COCOA"},
							},
						},
					},
					{
						Path: "nofeature.h",
						Condition: Not{
							Or{
								And{
									Defined{Name: "_WIN32"},
									Not{Defined{Name: "DISABLE_FEATURE"}},
								},
								And{
									Defined{Name: "__APPLE__"},
									Defined{Name: "ENABLE_COCOA"},
								},
							},
						},
					},
				},
			},
		},
		{
			// #if X as equivalent of X != 0
			input: `
#if TARGET_IOS
  #include "ios_api.h"
#elif !TARGET_WINDOWS 
	#include "unix_api.h"
#else
	#include "windows_api.h"
#endif
`,
			expected: SourceInfo{
				Includes: []Include{
					{
						Path:      "ios_api.h",
						Condition: Compare{Ident("TARGET_IOS"), "!=", Constant(0)},
					},
					{
						Path: "unix_api.h",
						Condition: And{
							Not{Compare{Ident("TARGET_WINDOWS"), "!=", Constant(0)}},
							Not{Compare{Ident("TARGET_IOS"), "!=", Constant(0)}},
						},
					},
					{
						Path: "windows_api.h",
						Condition: Not{
							Or{
								Compare{Ident("TARGET_IOS"), "!=", Constant(0)},
								Not{Compare{Ident("TARGET_WINDOWS"), "!=", Constant(0)}},
							}},
					},
				},
			},
		},
		{
			// simple #if / #else with comparsion operator
			input: `
#if __WINT_WIDTH__ >= 32
#include "wideint.h"
#else
#include "narrowint.h"
#endif
`,
			expected: SourceInfo{
				Includes: []Include{
					{
						Path:      "wideint.h",
						Condition: Compare{Ident("__WINT_WIDTH__"), ">=", Constant(32)},
					},
					{
						Path: "narrowint.h",
						Condition: Not{
							Compare{Ident("__WINT_WIDTH__"), ">=", Constant(32)},
						},
					},
				},
			},
		},
		{
			// simple #if / #else with comparsion operator
			input: `
		#if 1 == __LITTLE_ENDIAN__
		#include "a.h"
		#elif 0 != TARGET_IOS
		#include "b.h"
		#elif 32 > POINTER_SIZE
		#include "c.h"
		#endif
		`,
			expected: SourceInfo{
				Includes: []Include{
					{
						Path:      "a.h",
						Condition: Compare{Constant(1), "==", Ident("__LITTLE_ENDIAN__")},
					},
					{
						Path: "b.h",
						Condition: And{
							Compare{Constant(0), "!=", Ident("TARGET_IOS")},
							Not{Compare{Constant(1), "==", Ident("__LITTLE_ENDIAN__")}},
						},
					},
					{
						Path: "c.h",
						Condition: And{
							Compare{Constant(32), ">", Ident("POINTER_SIZE")},
							Not{Or{
								Compare{Constant(1), "==", Ident("__LITTLE_ENDIAN__")},
								Compare{Constant(0), "!=", Ident("TARGET_IOS")},
							}}},
					},
				},
			},
		},
		{
			// ==, >, and the automatic negations created for #elif / #else
			input: `
#if __ARM_ARCH == 8
#include "armv8.h"
#elif __ARM_ARCH > 8
#include "armv9.h"
#else
#include "armlegacy.h"
#endif
`,
			expected: SourceInfo{
				Includes: []Include{
					{
						Path:      "armv8.h",
						Condition: Compare{Ident("__ARM_ARCH"), "==", Constant(8)},
					},
					{
						Path: "armv9.h",
						// parser rewrites #elif into A && !previous(A)
						Condition: And{
							Compare{Ident("__ARM_ARCH"), ">", Constant(8)},
							Not{Compare{Ident("__ARM_ARCH"), "==", Constant(8)}},
						},
					},
					{
						Path: "armlegacy.h",
						// final #else → !(A || B)
						Condition: Not{
							Or{
								Compare{Ident("__ARM_ARCH"), "==", Constant(8)},
								Compare{Ident("__ARM_ARCH"), ">", Constant(8)},
							},
						},
					},
				},
			},
		},
		{
			// nested #if / #else blocks – 3 levels deep
			input: `
				#if defined FOO
					#include "foo.h"
						#if defined(BAR)
							#include "bar.h"
							#ifdef BAZ
								#include "baz.h"
							#elifdef QUX
								#include "qux.h"
							#else
								#include "nobaz.h"
							#endif
						#else
							#include "nobar.h"
						#endif
				#else
					#include "nofoo.h"
				#endif
				`,
			expected: SourceInfo{
				Includes: []Include{
					{
						Path:      "foo.h",
						Condition: Defined{Ident("FOO")},
					},
					{
						Path: "bar.h",
						Condition: And{
							Defined{Ident("FOO")},
							Defined{Ident("BAR")},
						},
					},
					{
						Path: "baz.h",
						Condition: And{
							And{
								Defined{Ident("FOO")},
								Defined{Ident("BAR")},
							},
							Defined{Ident("BAZ")},
						},
					},
					{
						// QUX branch:  FOO && BAR && QUX && !BAZ
						Path: "qux.h",
						Condition: And{
							And{ // FOO && BAR
								Defined{Ident("FOO")},
								Defined{Ident("BAR")},
							},
							And{ // QUX && !BAZ
								Defined{Ident("QUX")},
								Not{Defined{Ident("BAZ")}},
							},
						},
					},
					{
						// nobaz branch: FOO && BAR && !(BAZ || QUX)
						Path: "nobaz.h",
						Condition: And{
							And{ // FOO && BAR
								Defined{Ident("FOO")},
								Defined{Ident("BAR")},
							},
							Not{ // !(BAZ || QUX)
								Or{
									Defined{Ident("BAZ")},
									Defined{Ident("QUX")},
								},
							},
						},
					},
					{
						Path: "nobar.h",
						Condition: And{
							Defined{Ident("FOO")},
							Not{Defined{Ident("BAR")}},
						},
					},
					{
						Path:      "nofoo.h",
						Condition: Not{Defined{Ident("FOO")}},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		result, err := ParseSource(tc.input)
		if err != nil {
			t.Errorf("Failed to parse %q, reason: %v", tc.input, err)
		}
		assert.Equal(t, tc.expected, result, "Input:%v", tc.input)
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
		result, err := ParseSource(tc.input)
		if err != nil {
			t.Errorf("Failed to parse %q, reason: %v", tc.input, err)
		}
		hasMain := result.HasMain
		if fmt.Sprintf("%v", hasMain) != fmt.Sprintf("%v", tc.expected) {
			t.Errorf("For test case %d input: %q, expected %+v, but got %+v", idx, tc.input, tc.expected, hasMain)
		}
	}
}
