// Copyright 2025 EngFlow, Inc. All rights reserved.
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

package cpp

import (
	"path/filepath"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const languageName = "c++"

type cppLanguage struct{}

type cppInclude struct {
	// Include path extracted from brackets or double quotes
	rawPath string
	// Repository root directory relative rawPath for quoted include, rawPath otherwise
	normalizedPath string
	// True when include defined using brackets
	isSystemInclude bool
}

type cppImports struct {
	includes []cppInclude
	// TODO: module imports / exports
}

func NewLanguage() language.Language {
	return &cppLanguage{}
}

// language.Language methods
func (c *cppLanguage) Kinds() map[string]rule.KindInfo {
	kinds := make(map[string]rule.KindInfo)
	mergeMaps := func(m1, m2 map[string]bool) map[string]bool {
		result := make(map[string]bool, len(m1)+len(m2))
		for k, v := range m1 {
			result[k] = v
		}
		for k, v := range m2 {
			result[k] = v
		}
		return result
	}

	for _, commonDef := range ccRuleDefs {
		// Attributes common to all rules
		kindInfo := rule.KindInfo{
			NonEmptyAttrs:  map[string]bool{"srcs": true},
			MergeableAttrs: map[string]bool{"srcs": true, "deps": true},
			ResolveAttrs:   map[string]bool{"deps": true},
		}
		switch commonDef {
		case "cc_library":
			kindInfo.NonEmptyAttrs = mergeMaps(kindInfo.NonEmptyAttrs, map[string]bool{
				"hdrs": true,
			})
			kindInfo.MergeableAttrs = mergeMaps(kindInfo.MergeableAttrs, map[string]bool{
				"hdrs": true,
			})
		}
		kinds[commonDef] = kindInfo
	}

	return kinds
}

var ccRuleDefs = []string{
	"cc_library", "cc_shared_libary", "cc_static_library",
	"cc_import",
	"cc_binary",
	"cc_test",
}

func (c *cppLanguage) Loads() []rule.LoadInfo {
	return []rule.LoadInfo{
		{
			Name:    "@rules_cc//cc:defs.bzl",
			Symbols: ccRuleDefs,
		},
	}
}
func (*cppLanguage) Fix(c *config.Config, f *rule.File) {}

var sourceExtensions = []string{".c", ".cc", ".cpp", ".cxx", ".c++", ".S"}
var headerExtensions = []string{".h", ".hh", ".hpp", ".hxx"}
var cExtensions = append(sourceExtensions, headerExtensions...)

func hasMatchingExtension(filename string, extensions []string) bool {
	ext := filepath.Ext(filename)
	for _, validExt := range extensions {
		if strings.EqualFold(ext, validExt) { // Case-insensitive comparison
			return true
		}
	}
	return false
}
