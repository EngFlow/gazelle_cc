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

	"github.com/EngFlow/gazelle_cc/language/internal/cc/lexer"
	"github.com/stretchr/testify/assert"
)

func TestParseIncludes(t *testing.T) {
	testCases := []struct {
		input    string
		expected []Directive
	}{
		{
			// Parses valid source code
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
				IncludeDirective{Path: "multiple", IsSystem: false, LineNumber: 7},
				IncludeDirective{Path: "other_valid", IsSystem: true, LineNumber: 8},
			},
		},
		{
			// Handle very long multiline comments
			input: `
/*
 * Lorem ipsum dolor sit amet, consectetur adipiscing elit. Donec egestas bibendum sollicitudin. Sed eros dui, accumsan
 * tempor accumsan et, vehicula ac magna. Morbi interdum ipsum est, at mollis nulla pellentesque id. Aliquam elementum
 * quam blandit faucibus posuere. Curabitur fermentum tellus dolor, in molestie ex consequat vel. Nam iaculis ornare
 * odio. Pellentesque quis felis mauris. Nullam vestibulum consequat malesuada. Pellentesque quis porta urna, eget
 * pellentesque ipsum. Phasellus luctus luctus orci ut convallis. Etiam venenatis lectus id neque pellentesque, eu
 * tincidunt mauris sollicitudin. Vivamus posuere, lectus ut ultrices pharetra, libero sem aliquam mauris, finibus
 * tincidunt felis orci id turpis.
 * 
 * Donec in eleifend odio. Pellentesque sit amet malesuada lacus, sit amet varius nulla. Sed sem sapien, ullamcorper id
 * urna ut, consequat posuere sem. Mauris vitae est nulla. Morbi venenatis metus non elit dictum faucibus et id purus.
 * Nulla euismod posuere dolor. Aliquam vel diam orci.
 * 
 * Nulla porta tortor quis velit iaculis elementum. Morbi fermentum egestas augue eget scelerisque. Donec fermentum arcu
 * a justo congue, eu sagittis risus interdum. Pellentesque interdum cursus ex vitae imperdiet. Nunc at dolor mauris.
 * Suspendisse varius, eros sed luctus eleifend, lorem augue tempus lacus, venenatis blandit sem ante sit amet lacus.
 * Mauris feugiat dolor eget nunc hendrerit cursus. Nunc vestibulum arcu ipsum, sed dapibus ipsum elementum a. Aliquam
 * erat volutpat. Phasellus congue, odio id euismod mollis, dolor lorem euismod enim, quis fermentum odio urna efficitur
 * arcu. Nulla vestibulum dui sit amet nulla lacinia, at tincidunt nisi tincidunt. Sed metus nunc, tempor at aliquam
 * tempor, dictum pulvinar mauris.
 * 
 * Vivamus auctor hendrerit auctor. Duis vehicula faucibus consequat. Nulla sit amet lobortis libero. Vivamus rhoncus
 * lorem sed lectus imperdiet fermentum. Quisque laoreet elit id condimentum congue. Donec scelerisque, augue eget
 * egestas lobortis, nisl ligula ultricies leo, vitae sollicitudin purus nulla at arcu. Maecenas eget massa eget libero
 * venenatis rutrum vel eu nisl. Nunc scelerisque nunc at pharetra finibus. Donec ac ultrices erat, non aliquam nisi.
 * Lorem ipsum dolor sit amet, consectetur adipiscing elit.
 * 
 * Sed ut nibh erat. Cras sed velit at urna porttitor bibendum. Mauris imperdiet, lacus id viverra elementum, orci nulla
 * egestas mauris, quis ultrices ligula sem sit amet urna. Orci varius natoque penatibus et magnis dis parturient
 * montes, nascetur ridiculus mus. Nam dictum iaculis orci a sagittis. Nullam mauris mi, vestibulum quis erat vel,
 * sollicitudin bibendum justo. Pellentesque euismod, nibh et condimentum lobortis, erat magna scelerisque ex, vel
 * condimentum nisl sapien eget magna. Morbi in est blandit, egestas augue sit amet, luctus massa. Proin ultricies
 * rutrum semper. Aenean luctus in arcu nec porttitor. Maecenas a varius ligula.
 * 
 * Nulla id quam iaculis, rutrum nisl id, luctus massa. Duis ultricies odio at sapien porttitor gravida. In enim tellus,
 * pulvinar vel blandit at, luctus vel mauris. Morbi dictum, nisi ut finibus elementum, turpis orci vehicula purus, ut
 * lobortis turpis magna id risus. Phasellus vel purus pulvinar, gravida metus ut, lacinia erat. Integer tempus dictum
 * neque eu dictum. In condimentum at dolor at faucibus. Fusce mattis metus sodales accumsan consectetur. Proin eu
 * aliquam eros. Pellentesque tincidunt vehicula magna, a ornare leo sollicitudin a. Fusce nunc arcu, venenatis commodo
 * metus id, auctor commodo ligula. Praesent facilisis risus id leo ultrices, sed finibus metus tristique. Mauris
 * feugiat vestibulum orci a tristique.
 * 
 * Sed nec commodo dui. Nulla posuere sem erat, in imperdiet lectus efficitur eget. Etiam a enim hendrerit, tincidunt
 * dui quis, ultricies nisi. Integer arcu ipsum, commodo non vehicula eget, sodales et dui. Curabitur luctus magna a leo
 * condimentum auctor eu ac ex. Nulla velit urna, luctus eu vestibulum quis, varius id urna. Integer interdum quam
 * metus, sed maximus leo pharetra id. Donec viverra vulputate velit, vitae faucibus massa tempus sit amet. Cras
 * tristique ullamcorper erat ac mi.
 * \,
 */
#define MACRO
`,
			expected: []Directive{
				DefineDirective{Name: "MACRO", Args: []string{}, Body: []string{}},
			},
		},
		{
			// Malformed input
			input:    "\\,\n",
			expected: nil,
		},
	}

	for _, tc := range testCases {
		result := ParseSource([]byte(tc.input))
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
		// whitespace between '#' and directive keyword
		{
			input: `
		# ifdef _WIN32
		# endif
		`,
			expected: SourceInfo{
				Directives: []Directive{
					IfBlock{Branches: []ConditionalBranch{
						{
							Kind:      IfBranch,
							Condition: Defined{Ident("_WIN32")},
							Body:      nil,
						},
					}},
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
							Condition: Compare{Left: Ident("__WINT_WIDTH__"), Op: lexer.TokenType_OperatorGreaterOrEqual, Right: ConstantInt(32)},
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
							Condition: Compare{Left: ConstantInt(1), Op: lexer.TokenType_OperatorEqual, Right: Ident("__LITTLE_ENDIAN__")},
							Body: []Directive{
								IncludeDirective{Path: "a.h", LineNumber: 3},
							},
						},
						{
							Kind:      ElifBranch,
							Condition: Compare{Left: ConstantInt(0), Op: lexer.TokenType_OperatorNotEqual, Right: Ident("TARGET_IOS")},
							Body: []Directive{
								IncludeDirective{Path: "b.h", LineNumber: 5},
							},
						},
						{
							Kind:      ElifBranch,
							Condition: Compare{Left: ConstantInt(32), Op: lexer.TokenType_OperatorGreater, Right: Ident("POINTER_SIZE")},
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
							Condition: Compare{Left: Ident("__ARM_ARCH"), Op: lexer.TokenType_OperatorEqual, Right: ConstantInt(8)},
							Body: []Directive{
								IncludeDirective{Path: "armv8.h", LineNumber: 3},
							},
						},
						{
							Kind:      ElifBranch,
							Condition: Compare{Left: Ident("__ARM_ARCH"), Op: lexer.TokenType_OperatorGreater, Right: ConstantInt(8)},
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
							Condition: Compare{Left: Not{Ident("A")}, Op: lexer.TokenType_OperatorEqual, Right: Ident("B")},
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
										Body:      nil,
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
		{
			// Unclosed conditional block
			input: `
			#ifdef FOO
				#include "foo.h"
			`,
			expected: SourceInfo{
				Directives: nil,
			},
		},
	}

	for _, tc := range testCases {
		result := ParseSource([]byte(tc.input))
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
		{
			expected: true,
			input:    "int wmain( int argc, wchar_t *argv[ ], wchar_t *envp[ ] ) {return 0;}",
		},
	}

	for idx, tc := range testCases {
		result := ParseSource([]byte(tc.input))
		assert.Equal(t, tc.expected, result.HasMain, "Test case %d, Input: %v", idx, tc.input)
	}
}
