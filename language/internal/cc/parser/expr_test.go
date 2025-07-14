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

type macrosPreset string

const (
	linuxPreset   macrosPreset = "linux"
	windowsPreset macrosPreset = "windows"
)

var macroPresets = map[macrosPreset]cc.Macros{
	linuxPreset: {
		"LINUX":       1,
		"SHARED_FLAG": 1,
	},
	windowsPreset: {
		"WIN32":       1,
		"SHARED_FLAG": 0,
	},
}

// Test evaluation of expressions by testing against predefined platform macros.
// Checks if given expression evaluates to true using given macros set
func TestExprEvaluation(t *testing.T) {
	cases := []struct {
		name     string
		expr     Expr
		expected []macrosPreset
	}{
		{
			"simple presence",
			Defined{Name: "LINUX"},
			[]macrosPreset{linuxPreset},
		},
		{
			"unknown macro",
			Defined{Name: "OTHER"},
			[]macrosPreset{},
		},
		{
			"negated presence",
			Not{X: Defined{Name: "LINUX"}},
			[]macrosPreset{windowsPreset},
		},
		{
			"negated unknown macro",
			Not{X: Defined{Name: "OTHER"}},
			[]macrosPreset{linuxPreset, windowsPreset},
		},
		{
			"compare != 0", // #if SHARED_FLAG
			Compare{Left: Ident("SHARED_FLAG"), Op: "!=", Right: ConstantInt(0)},
			[]macrosPreset{linuxPreset},
		},
		{
			"compare == 0", // #if ! SHARED_FLAG
			Compare{Left: Ident("SHARED_FLAG"), Op: "==", Right: ConstantInt(0)},
			[]macrosPreset{windowsPreset},
		},
		{
			"compare >= 0",
			Compare{Left: Ident("SHARED_FLAG"), Op: ">=", Right: ConstantInt(0)},
			[]macrosPreset{linuxPreset, windowsPreset},
		},
		{
			"compare > 0",
			Compare{Left: Ident("SHARED_FLAG"), Op: ">", Right: ConstantInt(0)},
			[]macrosPreset{linuxPreset},
		},
		{
			"compare const == const -> true",
			Compare{Left: ConstantInt(0), Op: "==", Right: ConstantInt(0)},
			[]macrosPreset{linuxPreset, windowsPreset},
		},
		{
			"compare const != const -> true",
			Compare{Left: ConstantInt(0), Op: "!=", Right: ConstantInt(0)},
			[]macrosPreset{},
		},
		{
			"compare $ident == $ident -> true",
			Compare{Left: Ident("VER"), Op: "==", Right: Ident("VER")},
			[]macrosPreset{linuxPreset, windowsPreset},
		},
		{
			"compare $unknownIdent == 0 -> true",
			Compare{Left: Ident("OTHER"), Op: "==", Right: ConstantInt(0)},
			[]macrosPreset{linuxPreset, windowsPreset},
		},
		{
			"compare 0 != $unknownIdent -> false",
			Compare{Left: ConstantInt(0), Op: "!=", Right: Ident("OTHER")},
			[]macrosPreset{},
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
			[]macrosPreset{linuxPreset, windowsPreset},
		},
		{
			"eval ident",
			Ident("LINUX"),
			[]macrosPreset{linuxPreset},
		},
		{
			"eval constant zero",
			ConstantInt(0),
			[]macrosPreset{},
		},
		{
			"eval constant non zero",
			ConstantInt(1),
			[]macrosPreset{linuxPreset, windowsPreset},
		},
	}

	for _, tc := range cases {
		availableInPresets := []macrosPreset{}
		for platform, macros := range macroPresets {
			if tc.expr.Eval(macros) {
				availableInPresets = append(availableInPresets, platform)
			}
		}
		assert.ElementsMatch(t, tc.expected, availableInPresets, tc.name)
	}
}
