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

	"github.com/EngFlow/gazelle_cc/language/internal/cc/platform"
	"github.com/stretchr/testify/assert"
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

// Test evaluation of expressions by testing against predefined platform macros.
// Checks if given expression evaluates to true using given macros set
func TestExprEvaluation(t *testing.T) {
	cases := []struct {
		name     string
		expr     Expr
		expected []platform.Platform
	}{
		{
			"simple presence",
			Defined{Name: "LINUX"},
			[]platform.Platform{linuxAMD64},
		},
		{
			"unknown macro",
			Defined{Name: "OTHER"},
			[]platform.Platform{},
		},
		{
			"negated presence",
			Not{X: Defined{Name: "LINUX"}},
			[]platform.Platform{windowsAMD64},
		},
		{
			"negated unknown macro",
			Not{X: Defined{Name: "OTHER"}},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
		{
			"compare != 0", // #if SHARED_FLAG
			Compare{Left: Ident("SHARED_FLAG"), Op: "!=", Right: ConstantInt(0)},
			[]platform.Platform{linuxAMD64},
		},
		{
			"compare == 0", // #if ! SHARED_FLAG
			Compare{Left: Ident("SHARED_FLAG"), Op: "==", Right: ConstantInt(0)},
			[]platform.Platform{windowsAMD64},
		},
		{
			"compare >= 0",
			Compare{Left: Ident("SHARED_FLAG"), Op: ">=", Right: ConstantInt(0)},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
		{
			"compare > 0",
			Compare{Left: Ident("SHARED_FLAG"), Op: ">", Right: ConstantInt(0)},
			[]platform.Platform{linuxAMD64},
		},
		{
			"compare const == const -> true",
			Compare{Left: ConstantInt(0), Op: "==", Right: ConstantInt(0)},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
		{
			"compare const != const -> true",
			Compare{Left: ConstantInt(0), Op: "!=", Right: ConstantInt(0)},
			[]platform.Platform{},
		},
		{
			"compare $ident == $ident -> true",
			Compare{Left: Ident("VER"), Op: "==", Right: Ident("VER")},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
		{
			"compare $unknownIdent == 0 -> true",
			Compare{Left: Ident("OTHER"), Op: "==", Right: ConstantInt(0)},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
		{
			"compare 0 != $unknownIdent -> false",
			Compare{Left: ConstantInt(0), Op: "!=", Right: Ident("OTHER")},
			[]platform.Platform{},
		},
		{
			"AND / OR combo", // #if (defined(LINUX) && SHARED_FLAG) || defined(WIN32)
			Or{
				L: And{
					L: Defined{Name: "LINUX"},
					R: Compare{Left: Ident("SHARED_FLAG"), Op: "!=", Right: ConstantInt(0)},
				},
				R: Defined{Name: "WIN32"},
			},
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
		{
			"eval ident",
			Ident("LINUX"),
			[]platform.Platform{linuxAMD64},
		},
		{
			"eval constant zero",
			ConstantInt(0),
			[]platform.Platform{},
		},
		{
			"eval constant non zero",
			ConstantInt(1),
			[]platform.Platform{linuxAMD64, windowsAMD64},
		},
	}

	for _, tc := range cases {
		availableOnPlatform := []platform.Platform{}
		for platform, macros := range freshPlatformMacros() {
			if tc.expr.Eval(macros) {
				availableOnPlatform = append(availableOnPlatform, platform)
			}
		}
		assert.ElementsMatch(t, tc.expected, availableOnPlatform, tc.name)
	}
}
