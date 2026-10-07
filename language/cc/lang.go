// Copyright 2026 EngFlow Inc. All rights reserved.
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
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/EngFlow/gazelle_cc/internal/index"
	"github.com/EngFlow/gazelle_cc/language/internal/cc/platform"
	"github.com/bazel-contrib/bazel-gazelle/v2/compat"
	"github.com/bazel-contrib/bazel-gazelle/v2/config"
	"github.com/bazel-contrib/bazel-gazelle/v2/label"
	"github.com/bazel-contrib/bazel-gazelle/v2/language"
	"github.com/bazel-contrib/bazel-gazelle/v2/resolve"
	"github.com/bazel-contrib/bazel-gazelle/v2/rule"
)

var (
	rulesCcDefsBzl    = label.New("rules_cc", "cc", "defs.bzl")
	ccProtoLibraryBzl = label.New("protobuf", "bazel", "cc_proto_library.bzl")
	ccGrpcLibraryBzl  = label.New("grpc", "bazel", "cc_grpc_library.bzl")
)

var (
	_ language.Language     = (*ccLanguage)(nil)
	_ config.Configurer     = (*ccLanguage)(nil)
	_ compat.FlagConfigurer = (*ccLanguage)(nil)
	_ language.Generator    = (*ccLanguage)(nil)
	_ language.Fixer        = (*ccLanguage)(nil)
	_ language.OnFinisher   = (*ccLanguage)(nil)
	_ resolve.Indexer       = (*ccLanguage)(nil)
	_ resolve.Resolver      = (*ccLanguage)(nil)
)

const (
	languageName       = "cc"
	ccTestRunnerDepKey = "_test_runner"
	ccExistingDepsKey  = "_existing_deps"
)

type (
	ccLanguage struct {
		// Index of header includes parsed from Bazel Central Registry
		bzlmodBuiltInIndex ccDependencyIndex
		// Set of missing bazel_dep modules referenced in includes but not defined
		// Used for deduplication of missing modul_dep warnings
		notFoundBzlModDeps collections.Set[string]
		// Set of relative paths to directories that already have build files or
		// will have build files populated by rules from this extension or
		// others that ran earlier. Populated by Configure (called in pre-order)
		// and GenerateRules (called in post-order but maybe not recursively).
		buildFileDirRels collections.Set[string]
		// List of collected errors, reported together at once after the dependency resolution
		collectedErrors []error
		// Paths to directories with severe errors. We avoid generating or updating
		// rules in these directories.
		relsWithErrors collections.Set[string]
	}
	ccInclude struct {
		// File where this include was found
		sourceFile string
		// Line number in sourceFile where this include was found
		lineNumber int
		// Include path extracted from brackets or double quotes
		path string
		// True when include defined using brackets
		isSystemInclude bool
		// Indicates whether include is shared by all platforms or is restricted to specific ones
		isPlatformSpecific bool
		// List of platforms that matched the include #if condition. Empty when shared by all platforms or unreachable by any configured platform
		platforms []platform.Platform
	}
	ccImports struct {
		// #include directives found in header files, including those listed in "srcs" directories
		hdrIncludes []ccInclude
		// #include directives found in non-header files
		srcIncludes []ccInclude
		// TODO: module imports / exports
	}
	ccDependencyIndex map[string]label.Label
)

// Directory from which include is resolved
func (include ccInclude) sourceDirectory() string {
	return filepath.Dir(string(include.sourceFile))
}

func (include ccInclude) String() string {
	if include.isSystemInclude {
		return fmt.Sprintf("'#include <%s>' at %s:%d", include.path, include.sourceFile, include.lineNumber)
	} else {
		return fmt.Sprintf("'#include \"%s\"' at %s:%d", include.path, include.sourceFile, include.lineNumber)
	}
}

func (imports ccImports) allIncludes() []ccInclude {
	return slices.Concat(imports.hdrIncludes, imports.srcIncludes)
}

func NewV2() language.Language {
	return &ccLanguage{
		bzlmodBuiltInIndex: loadBuiltInBzlModDependenciesIndex(),
		notFoundBzlModDeps: make(collections.Set[string]),
		buildFileDirRels:   make(collections.Set[string]),
		relsWithErrors:     make(collections.Set[string]),
	}
}

func (*ccLanguage) Name() string { return languageName }

func (c *ccLanguage) Kinds() []rule.KindInfo {
	mergeMaps := func(m1, m2 map[string]bool) map[string]bool {
		result := make(map[string]bool, len(m1)+len(m2))
		maps.Copy(result, m1)
		maps.Copy(result, m2)
		return result
	}

	kinds := make([]rule.KindInfo, 0, len(ccRuleDefs)+2)
	for _, commonDef := range ccRuleDefs {
		kindInfo := rule.KindInfo{
			Name:           commonDef,
			LoadedFrom:     rulesCcDefsBzl,
			NonEmptyAttrs:  map[string]bool{"srcs": true, "deps": true},
			MergeableAttrs: map[string]bool{"srcs": true, "deps": true},
			ResolveAttrs:   map[string]bool{"deps": true},
		}
		if commonDef == "cc_library" {
			kindInfo.NonEmptyAttrs = mergeMaps(kindInfo.NonEmptyAttrs, map[string]bool{
				"hdrs":                true,
				"implementation_deps": true,
			})
			kindInfo.MergeableAttrs = mergeMaps(kindInfo.MergeableAttrs, map[string]bool{
				"hdrs":                 true,
				"implementation_deps":  true,
				"include_prefix":       true,
				"strip_include_prefix": true,
			})
			kindInfo.ResolveAttrs = mergeMaps(kindInfo.ResolveAttrs, map[string]bool{
				"implementation_deps": true,
			})
		}
		kinds = append(kinds, kindInfo)
	}
	kinds = append(kinds,
		rule.KindInfo{
			Name:           "cc_proto_library",
			LoadedFrom:     ccProtoLibraryBzl,
			MatchAttrs:     []string{"deps"},
			NonEmptyAttrs:  map[string]bool{"deps": true},
			MergeableAttrs: map[string]bool{"deps": true},
			ResolveAttrs:   map[string]bool{"deps": true},
		},
		rule.KindInfo{
			Name:       "cc_grpc_library",
			LoadedFrom: ccGrpcLibraryBzl,
			NonEmptyAttrs: map[string]bool{
				"srcs": true,
				"deps": true,
			},
			MergeableAttrs: map[string]bool{
				"srcs":              true,
				"deps":              true,
				"proto_only":        true,
				"grpc_only":         true,
				"well_known_protos": true,
			},
			ResolveAttrs: map[string]bool{
				"srcs": true,
				"deps": true,
			},
		},
	)

	return kinds
}

var ccRuleDefs = []string{
	"cc_library", "cc_shared_library", "cc_static_library",
	"cc_import",
	"cc_binary",
	"cc_test",
}

func (*ccLanguage) Fix(_ context.Context, _ language.FixArgs) error { return nil }

func (lang *ccLanguage) handleReportedError(rel string, mode errorReportingMode, err error) {
	switch mode {
	case errorReportingMode_warn:
		log.Print(err)
	case errorReportingMode_error:
		lang.collectedErrors = append(lang.collectedErrors, err)
		lang.relsWithErrors.Add(rel)
	}
}

var sourceExtensions = []string{".c", ".cc", ".cpp", ".cxx", ".c++", ".S"}
var headerExtensions = []string{".h", ".hh", ".hpp", ".hxx"}
var ccExtensions = append(sourceExtensions, headerExtensions...)

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

func loadUserProvidedDependencyIndex(file string) (index.DependencyIndex, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return index.DependencyIndex{}, err
	}

	var result index.DependencyIndex
	err = json.Unmarshal(data, &result)
	return result, err
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

// DO NOT SUBMIT: drop os.Exit once Gazelle v2 fails the run on OnFinish errors
// (or cc error directives return severity through Generate/Resolve) without -strict.
func (c *ccLanguage) OnFinish(_ context.Context) error {
	if len(c.collectedErrors) == 0 {
		return nil
	}
	log.Printf("Found %d error(s):", len(c.collectedErrors))
	for _, err := range c.collectedErrors {
		log.Printf("  %v", err)
	}
	os.Exit(1)
	return nil
}
