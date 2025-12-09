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

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
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

func TestImports(t *testing.T) {
	lang := &ccLanguage{}

	testCases := []struct {
		name            string
		ruleKind        string
		ruleAttrs       map[string]interface{}
		rulePrivate     map[string]interface{}
		pkg             string
		ccSearch        []ccSearch
		expectedImports []resolve.ImportSpec
	}{
		{
			name:     "cc_proto_library with proto files",
			ruleKind: "cc_proto_library",
			rulePrivate: map[string]interface{}{
				ccProtoLibraryFilesKey: []string{"foo.proto", "bar.proto"},
			},
			pkg: "pkg",
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/foo.pb.h"},
				{Lang: languageName, Imp: "pkg/bar.pb.h"},
			},
		},
		{
			name:            "cc_proto_library without private attr",
			ruleKind:        "cc_proto_library",
			pkg:             "pkg",
			expectedImports: []resolve.ImportSpec{},
		},
		{
			name:     "cc_library with basic hdrs",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs": []string{"foo.h", "bar.h"},
			},
			pkg: "pkg",
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/foo.h"},
				{Lang: languageName, Imp: "pkg/bar.h"},
			},
		},
		{
			name:     "cc_library with strip_include_prefix",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs":                 []string{"include/foo.h"},
				"strip_include_prefix": "pkg/include",
			},
			pkg: "pkg",
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/include/foo.h"},
			},
		},
		{
			name:     "cc_library with include_prefix",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs":           []string{"foo.h"},
				"include_prefix": "extra",
			},
			pkg: "pkg",
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/foo.h"},
				{Lang: languageName, Imp: "extra/foo.h"},
			},
		},
		{
			name:     "cc_library with includes",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs":     []string{"include/foo.h"},
				"includes": []string{"include"},
			},
			pkg: "pkg",
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/include/foo.h"},
				{Lang: languageName, Imp: "foo.h"},
			},
		},
		{
			name:     "cc_library with includes dot",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs":     []string{"foo.h"},
				"includes": []string{"."},
			},
			pkg: "pkg",
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/foo.h"},
				{Lang: languageName, Imp: "foo.h"},
			},
		},
		{
			name:     "cc_library with cc_search",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs": []string{"src/include/foo.h"},
			},
			pkg: "pkg",
			ccSearch: []ccSearch{
				{stripIncludePrefix: "pkg/src/include", includePrefix: ""},
			},
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/src/include/foo.h"},
				{Lang: languageName, Imp: "foo.h"},
			},
		},
		{
			name:     "cc_library with cc_search absolute path",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs": []string{"src/include/foo.h"},
			},
			pkg: "pkg",
			ccSearch: []ccSearch{
				{stripIncludePrefix: "/pkg/src/include", includePrefix: ""},
			},
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/src/include/foo.h"},
				{Lang: languageName, Imp: "foo.h"},
			},
		},
		{
			name:     "cc_library with cc_search and include_prefix",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs": []string{"src/include/foo.h"},
			},
			pkg: "pkg",
			ccSearch: []ccSearch{
				{stripIncludePrefix: "pkg/src/include", includePrefix: "extra"},
			},
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/src/include/foo.h"},
				{Lang: languageName, Imp: "foo.h"},
			},
		},
		{
			name:     "cc_library with includes and cc_search",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs":     []string{"include/foo.h"},
				"includes": []string{"include"},
			},
			pkg: "pkg",
			ccSearch: []ccSearch{
				{stripIncludePrefix: "pkg/include", includePrefix: ""},
			},
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/include/foo.h"},
				{Lang: languageName, Imp: "foo.h"},
			},
		},
		{
			name:     "cc_library with multiple cc_search",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs": []string{"src/include/foo.h"},
			},
			pkg: "pkg",
			ccSearch: []ccSearch{
				{stripIncludePrefix: "pkg/src/include", includePrefix: ""},
				{stripIncludePrefix: "pkg/src", includePrefix: ""},
			},
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/src/include/foo.h"},
				{Lang: languageName, Imp: "foo.h"},
				{Lang: languageName, Imp: "include/foo.h"},
			},
		},
		{
			name:     "cc_library with strip_include_prefix and include_prefix",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs":                 []string{"include/foo.h"},
				"strip_include_prefix": "pkg/include",
				"include_prefix":       "extra",
			},
			pkg: "pkg",
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/include/foo.h"},
				{Lang: languageName, Imp: "extra/pkg/include/foo.h"},
			},
		},
		{
			name:     "cc_library with all attributes",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs":                 []string{"include/foo.h"},
				"strip_include_prefix": "pkg/include",
				"include_prefix":       "extra",
				"includes":             []string{"include"},
			},
			pkg: "pkg",
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/include/foo.h"},
				{Lang: languageName, Imp: "extra/pkg/include/foo.h"},
				{Lang: languageName, Imp: "foo.h"},
			},
		},
		{
			name:     "cc_library with non-matching cc_search",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs": []string{"foo.h"},
			},
			pkg: "pkg",
			ccSearch: []ccSearch{
				{stripIncludePrefix: "other/pkg", includePrefix: ""},
			},
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/foo.h"},
			},
		},
		{
			name:     "cc_library with empty cc_search stripIncludePrefix",
			ruleKind: "cc_library",
			ruleAttrs: map[string]interface{}{
				"hdrs": []string{"foo.h"},
			},
			pkg: "pkg",
			ccSearch: []ccSearch{
				{stripIncludePrefix: "", includePrefix: ""},
			},
			expectedImports: []resolve.ImportSpec{
				{Lang: languageName, Imp: "pkg/foo.h"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create config
			cfg := &config.Config{}
			cfg.Exts = make(map[string]interface{})
			ccConfig := newCcConfig()
			ccConfig.ccSearch = tc.ccSearch
			cfg.Exts[languageName] = ccConfig

			// Create rule
			r := rule.NewRule(tc.ruleKind, "test_rule")
			for k, v := range tc.ruleAttrs {
				r.SetAttr(k, v)
			}
			for k, v := range tc.rulePrivate {
				r.SetPrivateAttr(k, v)
			}

			// Create file
			f := &rule.File{
				Pkg: tc.pkg,
			}

			// Call Imports
			imports := lang.Imports(cfg, r, f)

			// Verify imports
			assert.Equal(t, len(tc.expectedImports), len(imports), "Expected %d imports, got %d", len(tc.expectedImports), len(imports))

			// Create a map for easier comparison
			importMap := make(map[string]bool)
			for _, imp := range imports {
				importMap[imp.Imp] = true
			}

			for _, expected := range tc.expectedImports {
				assert.True(t, importMap[expected.Imp], "Expected import %q not found. Got: %v", expected.Imp, imports)
				assert.Equal(t, expected.Lang, languageName)
			}
		})
	}
}
