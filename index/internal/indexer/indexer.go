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
	Module struct {
		Repository string
		Targets    []*ModuleTarget
	}
	// Informations about Module
	ModuleTarget struct {
		Name               label.Label
		Hdrs               collections.Set[label.Label] // header files (each header is represented as a Label)
		Includes           collections.Set[string]      // list of include paths
		StripIncludePrefix string                       // optional prefix to remove
		IncludePrefix      string                       // optional prefix to add
		Deps               collections.Set[label.Label]
	}
)

// createHeaderIndex processes module infos to create three mappings:
// 1. headerToRule: headers mapping to exactly one Bazel rule (Label).
// 2. ambiguous: headers defined in multiple rules.
type IndexingResult struct {
	HeaderToRule map[string]label.Label
	Ambiguous    map[string]collections.Set[label.Label]
}

func CreateHeaderIndex(infos []Module) IndexingResult {
	// headersMapping will store header paths to a collections.Set of Labels.
	headersMapping := map[string]*collections.Set[label.Label]{}

	// Iterate through every module and every target.
	for _, module := range infos {
		for _, target := range module.Targets {
			// Create a targetLabel for the target using the module repository.
			targetLabel := label.New(module.Repository, target.Name.Pkg, target.Name.Name)
			if shouldExcludeTarget(targetLabel) {
				continue
			}

			for hdr := range target.Hdrs {
				normalizedPath := normalizeHeaderPath(hdr.Name, *target)
				if shouldExcludeHeader(normalizedPath) {
					continue
				}

				// Add the label to the header mapping.
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
		if len(*labels) == 1 {
			// Extract the only label in the collections.Set.
			for l := range *labels {
				headerToRule[path] = l
				break
			}
		} else {
			ambiguous[path] = *labels
		}
	}

	return IndexingResult{
		HeaderToRule: headerToRule,
		Ambiguous:    ambiguous,
	}
}

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

	segments := filepath.SplitList(path)
	// Exlucde possily hidden files
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
