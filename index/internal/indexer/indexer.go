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

// Package provides functionality to index C++ Bazel targets (typically cc_library)
// by the headers they expose. This enables mapping `#include` paths to specific Bazel targets.
//
// This package is intended to be used a common backend for indexing mechanisms for different kinds of external dependencies,
// based on their specific issues and integration requirements.
//
// Key types:
//   - Module: Represents an external Bazel repository and its C++ targets.
//   - Target: Represents an individual cc_library-like rule with its headers and attributes.
//   - IndexingResult: Captures the results of mapping headers to Bazel labels.
package indexer

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/EngFlow/gazelle_cc/internal/index"
	"github.com/bazelbuild/bazel-gazelle/label"
)

type (
	// Represents information about structure of possibly external dependency
	Module struct {
		// Name of external repository, or empty if targets are defined in the same Bazel repository
		Repository string
		// List of targets defined in given module, typically a single cc_library
		Targets []Target
	}
	// Defines information about structure of rule that might be indexed, typically based on cc_library
	Target struct {
		Name               label.Label
		Hdrs               collections.Set[label.Label] // header files (each header is represented as a Label)
		Includes           collections.Set[string]      // list of include paths
		StripIncludePrefix string                       // optional prefix to remove
		IncludePrefix      string                       // optional prefix to add
		Deps               collections.Set[label.Label] // dependencies on other targets
	}
)

// IndexingResult contains the results of indexing headers across multiple modules.
type IndexingResult struct {
	// HeaderToRule maps header include paths to exactly one Bazel rule.
	// Only unambiguous mappings are included here.
	HeaderToRule map[string]label.Label

	// Ambiguous contains headers that are defined by multiple rules across different modules.
	// This captures CROSS-MODULE ambiguity (e.g., both @moduleA//:lib and @moduleB//:lib
	// expose "common/header.h"). This is distinct from intra-module ambiguity which is
	// resolved by WithAmbiguousTargetsResolved() before indexing.
	//
	// These headers cannot be automatically resolved and may require manual configuration
	// or explicit dependency declarations in user BUILD files.
	Ambiguous map[string][]label.Label
}

// CreateHeaderIndex processes a list of modules to create a uniform index mapping headers
// to exactly one rule that provides their definition.
//
// Note: This function expects that intra-module ambiguity has already been resolved
// (via WithAmbiguousTargetsResolved) before calling. Cross-module ambiguity (the same
// header exposed by targets in different modules) is captured in IndexingResult.Ambiguous.
func CreateHeaderIndex(modules []Module) IndexingResult {
	// headersMapping will store header paths to a collections.Set of Labels.
	headersMapping := make(map[string][]label.Label)
	for _, module := range modules {
		for _, target := range module.Targets {
			// Create a targetLabel for the target using the module repository.
			// It's required to correctly map external module to sources found possibly in other rules
			targetLabel := label.New(module.Repository, target.Name.Pkg, target.Name.Name)
			// Normalize headers and add to mapping
			for hdr := range target.Hdrs {
				for _, normalizedPath := range IndexableIncludePaths(hdr, target) {
					if shouldExcludeHeader(normalizedPath) {
						continue
					}
					headersMapping[normalizedPath] = append(headersMapping[normalizedPath], targetLabel)
				}
			}
		}
	}

	// Partition the headers into non-conflicting (exactly one label) and ambiguous (multiple labels).
	headerToRule := make(map[string]label.Label)
	ambiguous := make(map[string][]label.Label)
	for path, labels := range headersMapping {
		if len(labels) == 1 {
			// Extract the only label in the collections.Set.
			for _, l := range labels {
				headerToRule[path] = l
				break
			}
		} else {
			// If there are multiple labels, mark as ambiguous
			ambiguous[path] = labels
		}
	}

	return IndexingResult{
		HeaderToRule: headerToRule,
		Ambiguous:    ambiguous,
	}
}

// Writes the mapping of IndexingResult.HeaderToRule to disk in JSON format.
// Labels are stored as renered strings
func (result IndexingResult) WriteToFile(outputFile string) error {
	// TODO: Temporary conversion to the new index.DependencyIndex format, so
	// "//index:integration_tests" can pass for PR #182. The real migration to
	// index.DependencyIndex will be done in another PR.
	mappings := make(index.DependencyIndex, len(result.HeaderToRule))
	for hdr, dep := range result.HeaderToRule {
		mappings[hdr] = []label.Label{dep}
	}

	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize header index to json: %w", err)
	}

	os.MkdirAll(filepath.Dir(outputFile), 0777)
	if err := os.WriteFile(outputFile, data, 0666); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}
	return nil
}

// String returns a human-readable string representation of the IndexingResult.
func (result IndexingResult) String() string {
	var sb strings.Builder

	sb.WriteString("Indexing result:\n")

	sb.WriteString(fmt.Sprintf("Headers with mapping: %d\n", len(result.HeaderToRule)))
	for _, hdr := range slices.Sorted(maps.Keys(result.HeaderToRule)) {
		sb.WriteString(fmt.Sprintf("%-80s: %v\n", hdr, result.HeaderToRule[hdr]))
	}

	sb.WriteString(fmt.Sprintf("Ambiguous headers: %d\n", len(result.Ambiguous)))
	for _, hdr := range slices.Sorted(maps.Keys(result.Ambiguous)) {
		sb.WriteString(fmt.Sprintf("%-80s: %v\n", hdr, result.Ambiguous[hdr]))
	}

	return sb.String()
}

func shouldExcludeHeader(path string) bool {
	// Exclude blank paths.
	if strings.TrimSpace(path) == "" {
		return true
	}

	// Exclude possibly hidden files or directories
	segments := strings.SplitSeq(path, string(filepath.Separator))
	for segment := range segments {
		if strings.HasPrefix(segment, ".") || strings.HasPrefix(segment, "_") {
			return true
		}
		segment = strings.ToLower(segment)
		switch segment {
		case "thirdparty", "third-party", "third_party", "3rd_party", "deps", "test", "tests", "internal":
			return true
		}

	}
	return false
}

// Returns all possible `#include` paths under which the given header (hdr)
// may be accessed when compiling a target using Bazel C++ rules.
//
// It considers the effects of the Bazel cc_library attributes:
// - strip_include_prefix: Removes a real path prefix before exposing headers
// - include_prefix: Prepends a virtual path to header includes after stripping
// - includes: Adds paths to the compiler’s -I or -iquote list for locating headers
//
// Returned paths reflect all valid compiler-visible forms for the header within the target’s package.
// They are useful for detecting which targets may expose a given header or for header-to-target indexing.
// It does expose possible include paths introduced as sideffects by other targets
func IndexableIncludePaths(header label.Label, target Target) []string {
	packagePath := target.Name.Pkg
	targetRelHdr := header.Rel(target.Name.Repo, target.Name.Pkg)
	hdr := filepath.ToSlash(filepath.Join(targetRelHdr.Pkg, targetRelHdr.Name))

	// Always include full path relative to workspace root
	headerPath := filepath.ToSlash(filepath.Join(packagePath, hdr))
	possibleIncludes := collections.SetOf(headerPath)

	// 1. Handle strip_include_prefix
	stripped := hdr
	if target.StripIncludePrefix != "" {
		stripPrefix := target.StripIncludePrefix
		if !path.IsAbs(stripPrefix) {
			stripPrefix = path.Join(targetRelHdr.Pkg, stripPrefix)
		}
		fullHdrPath := path.Join(header.Pkg, header.Name)

		if rel, err := filepath.Rel(stripPrefix, fullHdrPath); err == nil && !strings.HasPrefix(rel, "..") {
			stripped = filepath.ToSlash(rel)
			// Only add the stripped path if it’s not prefixed later
			if target.IncludePrefix == "" {
				possibleIncludes.Add(stripped)
			}
		}
	}

	// 2. Apply include_prefix (only valid when include_prefix is set)
	if target.IncludePrefix != "" && stripped != "" {
		withPrefix := filepath.ToSlash(path.Join(target.IncludePrefix, stripped))
		possibleIncludes.Add(withPrefix)
	}

	// 3. Derive paths from `includes`
	for include := range target.Includes {
		includePath := include
		if includePath == "." {
			includePath = ""
		}
		fullIncludePath := path.Join(packagePath, includePath)
		fullHdrPath := path.Join(packagePath, hdr)

		if rel, err := filepath.Rel(fullIncludePath, fullHdrPath); err == nil && !strings.HasPrefix(rel, "..") {
			rel = filepath.ToSlash(rel)
			if rel != "" {
				possibleIncludes.Add(rel)
			}
		}
	}

	// 4. Also add just the filename if includes would allow it
	if target.Includes.Contains(".") && !strings.Contains(hdr, "/") {
		possibleIncludes.Add(hdr)
		possibleIncludes.Add(path.Join(packagePath, hdr))
	}

	// Final collection
	return possibleIncludes.Values()
}
