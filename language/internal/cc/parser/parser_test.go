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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseIncludes(t *testing.T) {
	testCases := []struct {
		input    string
		expected []Directive
	}{
		// Parses valid source code
		{
			input: `
#include <stdio.h>
#include "myheader.h"
# include <math.h>
`,
			expected: []Directive{
				IncludeDirective{Path: "stdio.h", IsSystem: true, LineNumber: 2},
				IncludeDirective{Path: "myheader.h", LineNumber: 3},
				IncludeDirective{Path: "math.h", IsSystem: true, LineNumber: 4},
			},
		},
		{
			// Ignore malformed include
			input: `
#include "valid.h"
#include "stdio.h
#include stdlib.h"
#include <math.h
#include exception>
#include "multiple"quotes.h"
#include <other_valid>
# 
# unknown_directive
`,
			expected: []Directive{
				IncludeDirective{Path: "valid.h", LineNumber: 2},
				IncludeDirective{Path: "other_valid", IsSystem: true, LineNumber: 8},
			},
		},
	}

	for _, tc := range testCases {
		result, err := ParseSource(tc.input)
		if err != nil {
			t.Errorf("Failed to parse %q, reason: %v", tc.input, err)
		}
		assert.Equal(t, tc.expected, result.Directives, "Input:%v", tc.input)
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
				Directives: []Directive{
					IncludeDirective{Path: "common.h", LineNumber: 2},
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Defined{Ident("_WIN32")},
							Body: []Directive{
								IncludeDirective{Path: "windows.h", IsSystem: true, LineNumber: 4},
							},
						}, {
							Kind:      ElifBranch,
							Condition: Defined{Ident("__APPLE__")},
							Body:      []Directive{IncludeDirective{Path: "unistd.h", IsSystem: true, LineNumber: 7}},
						}, {
							Kind:      ElifBranch,
							Condition: Not{Defined{Ident("__linux__")}},
							Body: []Directive{
								IncludeDirective{Path: "fcntl.h", IsSystem: true, LineNumber: 9},
							},
						}, {
							Kind: ElseBranch,
							Body: []Directive{
								IncludeDirective{Path: "other.h", LineNumber: 11},
							},
						},
					}},
					IncludeDirective{Path: "last.h", LineNumber: 13},
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
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Defined{Ident("_WIN32")},
							Body: []Directive{
								IncludeDirective{Path: "windows.h", LineNumber: 3},
							},
						}, {
							Kind:      ElifBranch,
							Condition: Defined{Ident("__APPLE__")},
							Body:      []Directive{IncludeDirective{Path: "unistd.h", LineNumber: 5}},
						}, {
							Kind:      ElifBranch,
							Condition: Not{Defined{Ident("__linux__")}},
							Body: []Directive{
								IncludeDirective{Path: "fcntl.h", LineNumber: 9},
							},
						}, {
							Kind: ElseBranch,
							Body: []Directive{
								IncludeDirective{Path: "other.h", LineNumber: 11},
							},
						},
					}},
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
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind: IfBranch,
							Condition: Or{
								And{
									Defined{Ident("_WIN32")},
									Defined{Ident("ENABLE_GUI")},
								},
								Defined{Ident("__ANDROID__")},
							},
							Body: []Directive{
								IncludeDirective{Path: "ui.h", LineNumber: 3},
							},
						},
						{
							Kind:      ElifBranch,
							Condition: Defined{Ident("_WIN32")},
							Body: []Directive{
								IncludeDirective{Path: "cli.h", LineNumber: 5},
							},
						},
					}},
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
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind: IfBranch,
							Condition: Or{
								And{
									Defined{Ident("_WIN32")},
									Not{Defined{Ident("DISABLE_FEATURE")}},
								},
								And{
									Defined{Ident("__APPLE__")},
									Defined{Ident("ENABLE_COCOA")},
								},
							},
							Body: []Directive{
								IncludeDirective{Path: "feature.h", LineNumber: 5},
							},
						},
						{
							Kind: ElseBranch,
							Body: []Directive{
								IncludeDirective{Path: "nofeature.h", LineNumber: 7},
							},
						},
					}},
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
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Ident("TARGET_IOS"),
							Body: []Directive{
								IncludeDirective{Path: "ios_api.h", LineNumber: 3},
							},
						},
						{
							Kind:      ElifBranch,
							Condition: Not{Ident("TARGET_WINDOWS")},
							Body: []Directive{
								IncludeDirective{Path: "unix_api.h", LineNumber: 5},
							},
						},
						{
							Kind: ElseBranch,
							Body: []Directive{
								IncludeDirective{Path: "windows_api.h", LineNumber: 7},
							},
						},
					}},
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
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Compare{Left: Ident("__WINT_WIDTH__"), Op: ">=", Right: ConstantInt(32)},
							Body: []Directive{
								IncludeDirective{Path: "wideint.h", LineNumber: 3},
							},
						},
						{
							Kind: ElseBranch,
							Body: []Directive{
								IncludeDirective{Path: "narrowint.h", LineNumber: 5},
							},
						},
					}},
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
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Compare{Left: ConstantInt(1), Op: "==", Right: Ident("__LITTLE_ENDIAN__")},
							Body: []Directive{
								IncludeDirective{Path: "a.h", LineNumber: 3},
							},
						},
						{
							Kind:      ElifBranch,
							Condition: Compare{ConstantInt(0), "!=", Ident("TARGET_IOS")},
							Body: []Directive{
								IncludeDirective{Path: "b.h", LineNumber: 5},
							},
						},
						{
							Kind:      ElifBranch,
							Condition: Compare{Left: ConstantInt(32), Op: ">", Right: Ident("POINTER_SIZE")},
							Body: []Directive{
								IncludeDirective{Path: "c.h", LineNumber: 7},
							},
						},
					}},
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
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Compare{Left: Ident("__ARM_ARCH"), Op: "==", Right: ConstantInt(8)},
							Body: []Directive{
								IncludeDirective{Path: "armv8.h", LineNumber: 3},
							},
						},
						{
							Kind:      ElifBranch,
							Condition: Compare{Left: Ident("__ARM_ARCH"), Op: ">", Right: ConstantInt(8)},
							Body: []Directive{
								IncludeDirective{Path: "armv9.h", LineNumber: 5},
							},
						},
						{
							Kind: ElseBranch,
							Body: []Directive{
								IncludeDirective{Path: "armlegacy.h", LineNumber: 7},
							},
						},
					}},
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
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Defined{Ident("FOO")},
							Body: []Directive{
								IncludeDirective{Path: "foo.h", LineNumber: 3},
								IfBlock{Branches: []ConditionalBranch{
									{
										Kind:      IfBranch,
										Condition: Defined{Ident("BAR")},
										Body: []Directive{
											IncludeDirective{Path: "bar.h", LineNumber: 5},
											IfBlock{Branches: []ConditionalBranch{
												{
													Kind:      IfBranch,
													Condition: Defined{Ident("BAZ")},
													Body:      []Directive{IncludeDirective{Path: "baz.h", LineNumber: 7}},
												},
												{
													Kind:      ElifBranch,
													Condition: Defined{Ident("QUX")},
													Body:      []Directive{IncludeDirective{Path: "qux.h", LineNumber: 9}},
												},
												{
													Kind: ElseBranch,
													Body: []Directive{IncludeDirective{Path: "nobaz.h", LineNumber: 11}},
												},
											}},
										}}, {
										Kind: ElseBranch,
										Body: []Directive{IncludeDirective{Path: "nobar.h", LineNumber: 14}},
									}}}}},
						{
							Kind: ElseBranch,
							Body: []Directive{IncludeDirective{Path: "nofoo.h", LineNumber: 17}},
						},
					},
					},
				},
			},
		},
		{
			input: `
				#if !A == B
					#include <unistd.h>
				#endif`,
			expected: SourceInfo{
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Compare{Left: Not{Ident("A")}, Op: "==", Right: Ident("B")},
							Body:      []Directive{IncludeDirective{Path: "unistd.h", IsSystem: true, LineNumber: 3}},
						},
					}},
				},
			},
		},
		{
			input: `
				#ifndef FOO_H
					#define FOO_H
					#include "bar.h"
					#undef FOO_H
				#endif`,
			expected: SourceInfo{
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Not{Defined{Ident("FOO_H")}},
							Body: []Directive{
								DefineDirective{Name: "FOO_H", Args: []string{}, Body: []string{}},
								IncludeDirective{Path: "bar.h", LineNumber: 4},
								UndefineDirective{Name: "FOO_H"},
							},
						},
					}},
				},
			},
		},
		{
			// Apply function-like macro
			input: `
			#if defined(__has_builtin)
				#if __has_builtin(__builtin_add_overflow)
				#endif
			#endif
			`,
			expected: SourceInfo{
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Defined{Ident("__has_builtin")},
							Body: []Directive{
								IfBlock{Branches: []ConditionalBranch{
									{
										Kind:      IfBranch,
										Condition: Apply{Name: Ident("__has_builtin"), Args: []Expr{Ident("__builtin_add_overflow")}},
										Body:      []Directive{},
									},
								},
								},
							},
						}},
					},
				},
			},
		},
		{
			// Apply function-like macro
			input: `
			#define IS_EQUAL(a, b) ((a) == (b))
			#if IS_EQUAL(FOO, BAR)
				#include "foo.h"
			#else 
				#include "bar.h"
			#endif
			`,
			expected: SourceInfo{
				Directives: []Directive{
					DefineDirective{Name: "IS_EQUAL", Args: []string{"a", "b"}, Body: []string{"(", "(", "a", ")", "==", "(", "b", ")", ")"}},
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Apply{Name: Ident("IS_EQUAL"), Args: []Expr{Ident("FOO"), Ident("BAR")}},
							Body: []Directive{
								IncludeDirective{Path: "foo.h", LineNumber: 4},
							},
						},
						{
							Kind: ElseBranch,
							Body: []Directive{
								IncludeDirective{Path: "bar.h", LineNumber: 6},
							},
						},
					},
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
		assert.Equal(t, tc.expected, result.HasMain, "Test case %d, Input: %v", idx, tc.input)
	}
}
