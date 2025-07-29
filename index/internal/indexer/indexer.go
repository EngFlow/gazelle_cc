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

	"github.com/EngFlow/gazelle_cc/index/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/label"
)

type (
	// Represents information about structure of possibly external dependency
	Module struct {
		// Name of external repository, or empty if targets are defined in the same Bazel repository
		Repository string
		// List of targets defined in given module, typically a single cc_library
		Targets []*Target
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

type IndexingResult struct {
	// Headers mapping to exactly one Bazel rule
	HeaderToRule map[string]label.Label
	// Headers defined in multiple rules
	Ambiguous map[string][]label.Label
}

// Process list of modules to create an unfiorm index mapping header to exactly one rule that provides their definition.
// In case if multiple modules define same headers might try to select one that behaves as clousers over remaining ambigious rules.
func CreateHeaderIndex(modules []Module) IndexingResult {
	// headersMapping will store header paths to a collections.Set of Labels.
	headersMapping := make(map[string][]label.Label)
	for _, module := range modules {
		for _, target := range module.Targets {
			// Create a targetLabel for the target using the module repository.
			// It's required to correctly map external module to sources found possibly in other rules
			targetLabel := label.New(module.Repository, target.Name.Pkg, target.Name.Name)
			if shouldExcludeTarget(targetLabel) {
				continue
			}

			// Normalize headers and add to mapping
			for hdr := range target.Hdrs {
				for _, normalizedPath := range IndexableIncludePaths(hdr.Name, *target) {
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
	mappings := make(map[string]string, len(result.HeaderToRule))
	for hdr, label := range result.HeaderToRule {
		mappings[hdr] = label.String()
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
	segments := strings.Split(path, string(filepath.Separator))
	for _, segment := range segments {
		if strings.HasPrefix(segment, ".") || strings.HasPrefix(segment, "_") {
			return true
		}
	}
	return false
}

// shouldExcludeTarget determines if the given target (label) is possibly internal.
func shouldExcludeTarget(label label.Label) bool {
	// Check target's path segments: if any segment (split on non-word characters and filtered to letters)
	for _, segment := range filepath.SplitList(label.Pkg) {
		tokens := splitWords(segment)
		for _, token := range tokens {
			switch token {
			case "internal", "impl":
				return true
			}
		}
	}
	return false
}

// splits a string on non-letter characters.
func splitWords(s string) []string {
	isLetter := func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}
	tokens := strings.FieldsFunc(s, func(r rune) bool {
		return !isLetter(r)
	})
	// Filter out any empty tokens.
	var result []string
	for _, t := range tokens {
		if t != "" {
			result = append(result, t)
		}
	}
	return result
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
func IndexableIncludePaths(hdr string, target Target) []string {
	packagePath := target.Name.Pkg
	headerPath := filepath.ToSlash(filepath.Join(packagePath, hdr))

	// Always include full path relative to workspace root
	possibleIncludes := collections.SetOf(headerPath)

	// 1. Handle strip_include_prefix
	stripped := hdr
	if target.StripIncludePrefix != "" {
		stripPrefix := target.StripIncludePrefix
		if !path.IsAbs(stripPrefix) {
			stripPrefix = path.Join(packagePath, stripPrefix)
		}
		fullHdrPath := path.Join(packagePath, hdr)

		if rel, err := filepath.Rel(stripPrefix, fullHdrPath); err == nil && !strings.HasPrefix(rel, "..") {
			stripped = filepath.ToSlash(rel)
			// Only add the stripped path if it’s not prefixed later
			if target.IncludePrefix == "" {
				possibleIncludes.Add(stripped)
			}
		}
	}

	// 2. Include raw hdr as given unless is stripped
	if stripped == hdr {
		possibleIncludes.Add(hdr)
	}
	// 3. Apply include_prefix (only valid when include_prefix is set)
	if target.IncludePrefix != "" && stripped != "" {
		withPrefix := filepath.ToSlash(path.Join(target.IncludePrefix, stripped))
		possibleIncludes.Add(withPrefix)
	}

	// 4. Derive paths from `includes`
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

	// 5. Also add just the filename if includes would allow it
	if target.Includes.Contains(".") && !strings.Contains(hdr, "/") {
		possibleIncludes.Add(hdr)
		possibleIncludes.Add(path.Join(packagePath, hdr))
	}

	// Final collection
	return possibleIncludes.Values()
}
