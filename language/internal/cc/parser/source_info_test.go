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

	"github.com/EngFlow/gazelle_cc/language/internal/cc"
	"github.com/stretchr/testify/assert"
)

func TestCollectIncludes(t *testing.T) {
	type macrosCase struct {
		name   string
		macros cc.Macros
		want   []IncludeDirective
	}
	tests := []struct {
		name    string
		input   string
		wantAll []IncludeDirective
	}{
		{
			name: "flat includes",
			input: `
				#include <stdio.h>
				#include "foo.h"
				#include <bar.h>
			`,
			wantAll: []IncludeDirective{
				{Path: "stdio.h", IsSystem: true},
				{Path: "foo.h"},
				{Path: "bar.h", IsSystem: true},
			},
		},
		{
			name: "ifdef disables include",
			input: `
				#ifdef FOO
					#include "foo.h"
				#endif
				#include "always.h"
			`,
			wantAll: []IncludeDirective{
				{Path: "foo.h"},
				{Path: "always.h"},
			},
		},
		{
			name: "if/elif/else",
			input: `
				#if defined(A)
					#include "a.h"
				#elif defined(B)
					#include "b.h"
				#else
					#include "c.h"
				#endif
			`,
			wantAll: []IncludeDirective{
				{Path: "a.h"},
				{Path: "b.h"},
				{Path: "c.h"},
			},
		},
		{
			name: "define/undef",
			input: `
				#define FOO 1
				#ifdef FOO
					#include "foo.h"
					#undef FOO
				#endif
				#ifdef FOO
					#include "should_not_appear.h"
				#endif
			`,
			wantAll: []IncludeDirective{
				{Path: "foo.h"},
				{Path: "should_not_appear.h"},
			},
		},
		{
			name: "nested if",
			input: `
				#if defined(OUTER)
					#include "outer.h"
					#if defined(INNER)
						#include "inner.h"
					#endif
				#endif
				#include "always.h"
			`,
			wantAll: []IncludeDirective{
				{Path: "outer.h"},
				{Path: "inner.h"},
				{Path: "always.h"},
			},
		},
		{
			name: "define value and compare",
			input: `
				#define X 2
				#if X == 2
				#include "two.h"
				#elif X == 3
				#include "three.h"
				#endif
			`,
			wantAll: []IncludeDirective{
				{Path: "two.h"},
				{Path: "three.h"},
			},
		},
		{
			name: "undef macro disables include",
			input: `
				#define FOO 1
				#undef FOO
				#ifdef FOO
				#include "foo.h"
				#endif
			`,
			wantAll: []IncludeDirective{
				{Path: "foo.h"},
			},
		},
	}

	for _, tc := range tests {
		result, err := ParseSource(tc.input)
		if err != nil {
			t.Errorf("ParseSource failed for %q: %v", tc.name, err)
			continue
		}
		gotAll := result.CollectIncludes()
		assert.ElementsMatch(t, tc.wantAll, gotAll, "CollectIncludes failed for %q", tc.name)
	}
}
