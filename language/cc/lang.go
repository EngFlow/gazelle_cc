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
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"maps"

	"github.com/EngFlow/gazelle_cc/language/internal/cc/platform"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const languageName = "cc"

type (
	ccLanguage struct {
		// Index of header includes parsed from Bazel Central Registry
		bzlmodBuiltInIndex ccDependencyIndex
		// Set of missing bazel_dep modules referenced in includes but not defined
		// Used for deduplication of missing modul_dep warnings
		notFoundBzlModDeps map[string]bool
	}
	ccInclude struct {
		// Include path extracted from brackets or double quotes
		path string
		// Directory from which include is resolved
		fromDirectory string
		// True when include defined using brackets
		isSystemInclude bool
		// List of platforms that matched the include #if condition. nil if include is not guarded by preprocessor definitions
		platforms []platform.Platform
	}
	ccImports struct {
		// #include directives found in header files
		hdrIncludes []ccInclude
		// #include directives found in non-header files
		srcIncludes []ccInclude
		// TODO: module imports / exports
	}
	ccDependencyIndex map[string]label.Label
)

const ccProtoLibraryFilesKey = "_protos"

func NewLanguage() language.Language {
	return &ccLanguage{
		bzlmodBuiltInIndex: loadBuiltInBzlModDependenciesIndex(),
		notFoundBzlModDeps: make(map[string]bool),
	}
}

// language.Language methods
func (c *ccLanguage) Kinds() map[string]rule.KindInfo {
	kinds := make(map[string]rule.KindInfo)
	mergeMaps := func(m1, m2 map[string]bool) map[string]bool {
		result := make(map[string]bool, len(m1)+len(m2))
		maps.Copy(result, m1)
		maps.Copy(result, m2)
		return result
	}

	for _, commonDef := range ccRuleDefs {
		// Attributes common to all rules
		kindInfo := rule.KindInfo{
			NonEmptyAttrs:  map[string]bool{"srcs": true, "deps": true},
			MergeableAttrs: map[string]bool{"srcs": true, "deps": true},
			ResolveAttrs:   map[string]bool{"deps": true},
		}
		switch commonDef {
		case "cc_library":
			kindInfo.NonEmptyAttrs = mergeMaps(kindInfo.NonEmptyAttrs, map[string]bool{
				"hdrs":                true,
				"implementation_deps": true,
			})
			kindInfo.MergeableAttrs = mergeMaps(kindInfo.MergeableAttrs, map[string]bool{
				"hdrs":                true,
				"implementation_deps": true,
			})
			kindInfo.ResolveAttrs = mergeMaps(kindInfo.ResolveAttrs, map[string]bool{
				"implementation_deps": true,
			})
		}
		kinds[commonDef] = kindInfo
	}
	kinds["cc_proto_library"] = rule.KindInfo{
		MatchAttrs:     []string{"deps"},
		NonEmptyAttrs:  map[string]bool{"deps": true},
		MergeableAttrs: map[string]bool{"deps": true},
		ResolveAttrs:   map[string]bool{"deps": true},
	}

	return kinds
}

var ccRuleDefs = []string{
	"cc_library", "cc_shared_libary", "cc_static_library",
	"cc_import",
	"cc_binary",
	"cc_test",
}
var knownRuleKinds = append(ccRuleDefs, "cc_proto_library")

func (c *ccLanguage) Loads() []rule.LoadInfo {
	panic("ApparentLoads should be called instead")
}

func (*ccLanguage) ApparentLoads(moduleToApparentName func(string) string) []rule.LoadInfo {
	apparentOfDefaultName := func(moduleName, defaultName string) string {
		if module := moduleToApparentName(moduleName); module != "" {
			return module
		} else {
			return defaultName
		}
	}

	return []rule.LoadInfo{
		{
			Name:    fmt.Sprintf("@%s//cc:defs.bzl", apparentOfDefaultName("rules_cc", "rules_cc")),
			Symbols: ccRuleDefs,
		},
		{
			Name:    fmt.Sprintf("@%s//bazel:cc_proto_library.bzl", apparentOfDefaultName("protobuf", "com_google_protobuf")),
			Symbols: []string{"cc_proto_library"},
		},
	}
}
func (*ccLanguage) Fix(c *config.Config, f *rule.File) {}

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

//go:embed bzldep-index.json
var bzlDepHeadersIndex string

func loadBuiltInBzlModDependenciesIndex() ccDependencyIndex {
	index, err := unmarshalDependencyIndex([]byte(bzlDepHeadersIndex))
	if err != nil {
		index = make(ccDependencyIndex)
	}
	return index
}

func loadDependencyIndex(file string) (ccDependencyIndex, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return unmarshalDependencyIndex(data)
}

func unmarshalDependencyIndex(data []byte) (ccDependencyIndex, error) {
	var rawLabels map[string]string
	if err := json.Unmarshal(data, &rawLabels); err != nil {
		return nil, err
	}

	index := make(ccDependencyIndex, len(rawLabels))
	for hdr, target := range rawLabels {
		if decoded, err := label.Parse(target); err == nil {
			index[hdr] = decoded
		}
	}
	return index, nil
}
