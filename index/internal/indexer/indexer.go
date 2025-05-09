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

package indexer

import (
	"encoding/json"
	"log"
	"maps"
	"os"
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
		Targets []*ModuleTarget
	}
	// Defines information about structure of rule that might be indexed, typically based on cc_library
	ModuleTarget struct {
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
	Ambiguous map[string]collections.Set[label.Label]
}

// Process list of modules to create an unfiorm index mapping header to exactly one rule that provides their definition.
// In case if multiple modules define same headers might try to select one that behaves as clousers over remaining ambigious rules.
func CreateHeaderIndex(modules []Module) IndexingResult {
	// headersMapping will store header paths to a collections.Set of Labels.
	headersMapping := map[string]*collections.Set[label.Label]{}
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
				normalizedPath := normalizeHeaderPath(hdr.Name, *target)
				if shouldExcludeHeader(normalizedPath) {
					continue
				}
				if _, exists := headersMapping[normalizedPath]; !exists {
					headersMapping[normalizedPath] = &collections.Set[label.Label]{}
				}
				headersMapping[normalizedPath].Add(targetLabel)
			}
		}
	}

	// Partition the headers into non-conflicting (exactly one label) and ambiguous (multiple labels).
	headerToRule := make(map[string]label.Label)
	ambiguous := make(map[string]collections.Set[label.Label])
	for path, labels := range headersMapping {
		switch len(*labels) {
		case 1:
			// Extract the only label in the collections.Set.
			for l := range *labels {
				headerToRule[path] = l
				break
			}
		default:
			ambiguous[path] = *labels
		}
	}

	return IndexingResult{
		HeaderToRule: headerToRule,
		Ambiguous:    ambiguous,
	}
}

// Writes the mapping of IndexingResult.HeaderToRule to disk in JSON format.
// Labels are stored as renered strings
func (result IndexingResult) WriteToFile(outputFile string) {
	mappings := make(map[string]string, len(result.HeaderToRule))
	for hdr, lbl := range result.HeaderToRule {
		mappings[hdr] = lbl.String()
	}

	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		log.Fatalf("Failed to serialize header index to JSON: %v", err.Error())
	}

	os.MkdirAll(filepath.Dir(outputFile), 0644)
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		log.Fatalf("Failed to write index file: %v", err.Error())
	}
}

// Prints to stdout detailed information about headers with resolved mappings and the ambigious header definitions.
func (result IndexingResult) Show() {
	log.Printf("Indexing result:")
	log.Printf("Headers with mapping: %v", len(result.HeaderToRule))
	for _, hdr := range slices.Sorted(maps.Keys(result.HeaderToRule)) {
		log.Printf("%-80s: %v", hdr, result.HeaderToRule[hdr])
	}

	log.Printf("Ambigious headers: %v", len(result.Ambiguous))
	for _, hdr := range slices.Sorted(maps.Keys(result.Ambiguous)) {
		log.Printf("%-80s: %v", hdr, result.Ambiguous[hdr])
	}
}

// Groups targets into disjoint groups based on the their defined headers.
// Allows to find targets that contain at least 1 common header defined in their definition.
func (module Module) GroupTargetsByHeaders() []collections.Set[*ModuleTarget] {
	targets := module.Targets
	var groups []collections.Set[*ModuleTarget]

	// Build adjacency list: map each target index to its neighbors
	adj := make(map[int][]int)
	n := len(targets)
	for i := range n {
		for j := i + 1; j < n; j++ {
			if (targets[i]).Hdrs.Intersects(targets[j].Hdrs) {
				adj[i] = append(adj[i], j)
				adj[j] = append(adj[j], i)
			}
		}
	}

	// DFS to find connected components
	visited := make([]bool, n)
	for i := range n {
		if visited[i] {
			continue
		}
		stack := []int{i}
		component := make(collections.Set[*ModuleTarget])
		visited[i] = true

		for len(stack) > 0 {
			curr := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			component[targets[curr]] = true
			for _, neighbor := range adj[curr] {
				if !visited[neighbor] {
					stack = append(stack, neighbor)
					visited[neighbor] = true
				}
			}
		}
		groups = append(groups, component)
	}
	return groups
}

// Given set of targets that define the same headers try to select ones that contain other targets as their direct or transitive dependencies
func SelectRootTargets(targets collections.Set[*ModuleTarget]) []*ModuleTarget {
	allTargets := make(map[label.Label]*ModuleTarget)
	dependentTargets := make(collections.Set[label.Label])

	// Collect all target names
	for target := range targets {
		allTargets[target.Name] = target
	}

	// Mark all targets that are listed as dependencies
	for target := range targets {
		for dep := range target.Deps {
			dependentTargets[dep] = true
		}
	}

	// Any target not in the dependency map is a root
	roots := make(collections.Set[*ModuleTarget])
	for name, target := range allTargets {
		if !dependentTargets[name] {
			roots.Add(target)
		}
	}

	return roots.Values()
}

func shouldExcludeHeader(path string) bool {
	// Exclude blank paths.
	if strings.TrimSpace(path) == "" {
		return true
	}

	// Exlucde possily hidden files
	segments := filepath.SplitList(path)
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

// normalizeHeaderPath applies several normalization steps to make the header path
// conform to a format that a C compiler or cc_rules can correctly resolve.
func normalizeHeaderPath(hdrPath string, target ModuleTarget) string {
	path := hdrPath

	// Step 1: If a stripIncludePrefix exists and hdrPath starts with it, remove it.
	if target.StripIncludePrefix != "" {
		if relativized, err := filepath.Rel(target.StripIncludePrefix, path); err == nil {
			path = relativized
		}
	}

	// Step 2: If an includePrefix exists, prepend it (i.e. join it with path).
	if target.IncludePrefix != "" {
		path = filepath.Join(target.IncludePrefix, path)
	}

	// Step 3: From target.Includes, find the longest include path that is a prefix of 'path'
	// and make 'path' relative to it.
	matchingIncludes := collections.Filter(target.Includes.Values(), func(include string) bool {
		return strings.HasPrefix(string(path), include)
	})
	switch len(matchingIncludes) {
	case 0: // no-op
	default:
		longestInclude := slices.MaxFunc(matchingIncludes, func(l, r string) int {
			return len(l) - len(r)
		})
		if relativize, err := filepath.Rel(longestInclude, path); err == nil {
			path = relativize
		}
	}

	// Step 4: If no normalization was applied (i.e. path remains equal to hdrPath)
	// and the target's name defines a package relative path, then prepend it.
	// (This step’s condition is taken to mean that if target.Includes was empty and hdrPath
	// was not modified, we use target.Name.Pkg)
	if target.Name.Pkg != "" && path == hdrPath && len(target.Includes) == 0 {
		path = filepath.Join(target.Name.Pkg, path)
	}

	return path
}
