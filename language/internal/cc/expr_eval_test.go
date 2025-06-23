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
	"slices"
	"testing"

	"github.com/EngFlow/gazelle_cc/language/internal/cc/parser"
	"github.com/EngFlow/gazelle_cc/language/internal/cc/platform"
)

var (
	linuxAMD64   = platform.Platform{OS: platform.Os("linux"), Arch: platform.Arch("x86_64")}
	windowsAMD64 = platform.Platform{OS: platform.Os("windows"), Arch: platform.Arch("x86_64")}
)

func freshPlatformMacros() map[platform.Platform]platform.Macros {
	return map[platform.Platform]platform.Macros{
		linuxAMD64: {
			"LINUX":       1,
			"SHARED_FLAG": 1,
		},
		windowsAMD64: {
			"WIN32":       1,
			"SHARED_FLAG": 0,
		},
	}
}

func TestPlatformsForMacro(t *testing.T) {
	platform.KnownPlatformMacros = freshPlatformMacros()

	tests := []struct {
		name     string
		macro    string
		expected []platform.Platform
	}{
		{"known macro", "LINUX", []platform.Platform{linuxAMD64}},
		{"common macro", "SHARED_FLAG", []platform.Platform{linuxAMD64, windowsAMD64}},
		{"unknown macro", "NOT_DEFINED", []platform.Platform{}},
	}

	for _, tc := range tests {
		got := platformsForMacro(tc.macro, platform.KnownPlatformMacros).Values()
		slices.SortFunc(got, platform.ComparePlatform)
		if !slices.Equal(got, tc.expected) {
			t.Errorf("%s: platformsForMacro(%q) = %v, want %v", tc.name, tc.macro, got, tc.expected)
		}
	}
}

func TestPlatformsForExpr(t *testing.T) {
	platform.KnownPlatformMacros = freshPlatformMacros()

	cases := []struct {
		name     string
		expr     parser.Expr
		expected []platform.Platform
	}{
		{
			"simple presence",
			parser.Defined{Name: "LINUX"},
			[]platform.Platform{linuxAMD64},
		},
		{
			"unknown macro",
			parser.Defined{Name: "OTHER"},
			[]platform.Platform{},
		},
		{
			"negated presence",
			parser.Not{X: parser.Defined{Name: "LINUX"}},
			[]platform.Platform{windowsAMD64},
		},
		{
			"negated unknown macro",
			parser.Not{X: parser.Defined{Name: "OTHER"}},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
		{
			"compare != 0", // #if SHARED_FLAG
			parser.Compare{Left: parser.Ident("SHARED_FLAG"), Op: "!=", Right: parser.Constant(0)},
			[]platform.Platform{linuxAMD64},
		},
		{
			"compare == 0", // #if ! SHARED_FLAG
			parser.Compare{Left: parser.Ident("SHARED_FLAG"), Op: "==", Right: parser.Constant(0)},
			[]platform.Platform{windowsAMD64},
		},
		{
			"compare >= 0",
			parser.Compare{Left: parser.Ident("SHARED_FLAG"), Op: ">=", Right: parser.Constant(0)},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
		{
			"compare > 0",
			parser.Compare{Left: parser.Ident("SHARED_FLAG"), Op: ">", Right: parser.Constant(0)},
			[]platform.Platform{linuxAMD64},
		},
		{
			"compare const == const -> true",
			parser.Compare{Left: parser.Constant(0), Op: "==", Right: parser.Constant(0)},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
		{
			"compare const != const -> true",
			parser.Compare{Left: parser.Constant(0), Op: "!=", Right: parser.Constant(0)},
			[]platform.Platform{},
		},
		{
			"compare $ident == $ident -> true",
			parser.Compare{Left: parser.Ident("VER"), Op: "==", Right: parser.Ident("VER")},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
		{
			"compare $unknownIdent == 0 -> true",
			parser.Compare{Left: parser.Ident("OTHER"), Op: "==", Right: parser.Constant(0)},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
		{
			"compare 0 != $unknownIdent -> false",
			parser.Compare{Left: parser.Constant(0), Op: "!=", Right: parser.Ident("OTHER")},
			[]platform.Platform{},
		},
		{
			"AND / OR combo", // #if (defined(LINUX) && SHARED_FLAG) || defined(WIN32)
			parser.Or{
				L: parser.And{
					L: parser.Defined{Name: "LINUX"},
					R: parser.Compare{Left: parser.Ident("SHARED_FLAG"), Op: "!=", Right: parser.Constant(0)},
				},
				R: parser.Defined{Name: "WIN32"},
			},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
	}

	for _, tc := range cases {
		got := PlatformsForExpr(tc.expr, platform.KnownPlatformMacros)
		if !slices.Equal(got, tc.expected) {
			t.Errorf("%s: PlatformsForExpr(%v) = %v, want %v", tc.name, tc.expr, got, tc.expected)
		}
	}
}
