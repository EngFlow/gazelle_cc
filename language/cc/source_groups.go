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
	"log"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cc/internal/collections"
)

// groupId represents a unique identifier for a group of source files
// used when detecting (possibly cyclic) dependenices between sources inside directory under `cc_group unit` mode
// it might be constructred based on the repository-relative path to the sources (excluding file extension) or just the directory name
type groupId string

// constructs a rule name based on the groupId, typically it's the last segment of the path (directory or file name without extension)
func (id groupId) toRuleName() string {
	return filepath.Base(string(id))
}

// sourceGroup represents a collection of source files and their dependencies
type sourceGroup struct {
	sources   []fileInfo
	dependsOn []groupId // Direct dependencies of this group (only used internally for testing)
	subGroups []groupId // Sub-groups creating this group
}

// sourceGroups is a mapping of groupIds to their corresponding sourceGroups
type sourceGroups map[groupId]*sourceGroup

// Returns a source group by assigning files based on their filename (excluding extension)
// without analyzing dependencies between sources
func identitySourceGroups(srcs []fileInfo) sourceGroups {
	srcGroups := make(sourceGroups)
	for _, src := range srcs {
		srcGroups[fileNameToGroupId(src.name)] = &sourceGroup{sources: []fileInfo{src}}
	}
	return srcGroups
}

// returns a sorted list of groupIds from the sourceGroups
func (g *sourceGroups) groupIds() []groupId {
	ids := slices.Collect(maps.Keys(*g))
	slices.Sort(ids)
	return ids
}

// sort ensures the sources and dependencies in each sourceGroup are sorted.
func (groups *sourceGroups) sort() {
	for _, group := range *groups {
		slices.SortFunc(group.sources, func(a, b fileInfo) int { return strings.Compare(a.name, b.name) })
		slices.Sort(group.subGroups)
		slices.Sort(group.dependsOn)
	}
}

// Modify the sourceGroups entry refered by current groupId and rename it as replacement.
// If the sourceGrops contians entry with replacement groupId their content would be merged
// Returns false if sourceGroups does not define entry with current groupId or true otherwise
func (g *sourceGroups) renameOrMergeWith(current groupId, replacement groupId) bool {
	if current == replacement {
		return false
	}
	group, exists := (*g)[current]
	if !exists {
		return false
	}
	node := group
	if targetGroup, exists := (*g)[replacement]; exists {
		node = &sourceGroup{
			sources:   slices.Concat(targetGroup.sources, group.sources),
			dependsOn: concatUnique(targetGroup.dependsOn, group.dependsOn),
			subGroups: slices.Concat(targetGroup.subGroups, group.subGroups),
		}
	}
	(*g)[replacement] = node
	delete(*g, current)
	return true
}

// Groups source files based on headers and their dependencies
// Splits input sources into non-recursive groups based on dependencies tracked using include directives.
// The function panics if any of input sources is not defined sourceInfos map.
// Header (.h) and it's corresponding implemention (.cc) are always grouped together.
// Source files without corresponding headers are assigned to single-element groups and can never become dependency of any other group.
// Each source file is guaranteed to be assigned to exactly 1 group.
func groupSourcesByUnits(rel, stripIncludePrefix, includePrefix string, ccSearch []ccSearch, fileInfos []fileInfo) sourceGroups {
	graph := buildDependencyGraph(rel, stripIncludePrefix, includePrefix, ccSearch, fileInfos)
	sccs := graph.findStronglyConnectedComponents()
	groups := splitIntoSourceGroups(fileInfos, sccs, graph)
	groups.resolveGroupDependencies(graph)
	groups.sort()             // Ensure deterministic output
	groups.sourceToGroupIds() // Consistency check

	return groups
}

type sourceFileSet map[string]bool

// represents a node in the dependency graph.
type sourceGroupNode struct {
	sources   []string
	adjacency collections.Set[groupId] // Direct dependencies of this node
}

// sourceDependencyGraph represents a directed graph of source dependencies
type sourceDependencyGraph map[groupId]*sourceGroupNode

// Source file (.cc) and it's corresponsing header are always grouped together and become a node in a dependency graph.
// Nodes of the graph are constructed base on sources having the same name (excluding extension suffix)
// Edges of the dependency graph are constructed based on include directives to local headers defined in sources of the graph node
func buildDependencyGraph(rel, stripIncludePrefix, includePrefix string, ccSearch []ccSearch, fileInfos []fileInfo) sourceDependencyGraph {
	// Initialize graph nodes and build maps from include paths to group IDs.
	// We consider three types of includes:
	// 1. Full include paths, relative to the repository root. These paths are
	//    usable even when strip_include_prefix / include_prefix are set.
	// 2. Transformed include paths with strip_include_prefix / include_prefix
	//    applied.
	// 3. Local include paths, relative to the including file's directory.
	//    But map keys are relative to THIS directory.
	// This list should also be modified by the includes attribute, but we don't
	// generate those in new rules, and at this point, we haven't associated
	// files with existing rules, so we don't have an includes list to apply.
	// 4. Transformed include paths based on cc_search configurations.
	graph := make(sourceDependencyGraph)
	includeToGroup := make(map[string]groupId)
	for _, fi := range fileInfos {
		id := fileNameToGroupId(fi.name)
		graph[id] = &sourceGroupNode{adjacency: make(collections.Set[groupId])}

		fullRel := path.Join(rel, fi.name)
		includeToGroup[fullRel] = id
		transformed := transformIncludePath(rel, stripIncludePrefix, includePrefix, fullRel)
		includeToGroup[transformed] = id
		includeToGroup[fi.name] = id

		// Also consider cc_search configurations for additional include path transformations
		for _, search := range ccSearch {
			stripPrefix := transformIncludePath(rel, search.stripIncludePrefix, search.includePrefix, fullRel)
			includeToGroup[stripPrefix] = id
		}
	}

	// Create edges based on includes between these files, using the graph above.
	// Don't consider system includes or includes of files outside this set.
	// The latter are handled separately during dependency resolution.
	for _, file := range fileInfos {
		id := fileNameToGroupId(file.name)
		node := graph[id]
		node.sources = append(node.sources, file.name)
		for _, include := range file.includes {
			if include.isSystemInclude {
				continue
			}
			if id, ok := includeToGroup[include.path]; ok {
				node.adjacency.Add(id)
				continue
			}
			relInclude := path.Join(path.Dir(file.name), include.path)
			if id, ok := includeToGroup[relInclude]; ok {
				node.adjacency.Add(id)
			}
		}
	}
	return graph
}

// Split dependency graph groups using Tarjan’s algorithm to detect strongly connected components (SCCs).
// Every component []groupId contains a list of groups that depend recursivelly on each other
func (graph *sourceDependencyGraph) findStronglyConnectedComponents() [][]groupId {
	index := 0
	indices := make(map[groupId]int)
	lowLink := make(map[groupId]int)
	onStack := make(map[groupId]bool)
	var stack []groupId
	var sccs [][]groupId

	var strongConnect func(node groupId)
	strongConnect = func(node groupId) {
		indices[node] = index
		lowLink[node] = index
		index++
		stack = append(stack, node)
		onStack[node] = true

		nodes := *graph
		for dep := range nodes[node].adjacency {
			if _, exists := indices[dep]; !exists {
				strongConnect(dep)
				lowLink[node] = min(lowLink[node], lowLink[dep])
			} else if onStack[dep] {
				lowLink[node] = min(lowLink[node], indices[dep])
			}
		}

		if lowLink[node] == indices[node] {
			var scc []groupId
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == node {
					break
				}
			}
			sccs = append(sccs, scc)
		}
	}

	for groupId := range *graph {
		if _, exists := indices[groupId]; !exists {
			strongConnect(groupId)
		}
	}
	return sccs
}

// Merges sources assigned to each componenet ([]groupId) into a sourceGrops
// Panics if any groupId defined in fileGroups is not defined in graph
func splitIntoSourceGroups(fileInfos []fileInfo, fileGroups [][]groupId, graph sourceDependencyGraph) sourceGroups {
	nameToFileInfo := make(map[string]fileInfo, len(fileInfos))
	for _, fi := range fileInfos {
		nameToFileInfo[fi.name] = fi
	}
	groups := make(sourceGroups, len(fileGroups))

	for _, sourcesGroup := range fileGroups {
		var groupSources []string
		for _, groupId := range sourcesGroup {
			for _, src := range graph[groupId].sources {
				groupSources = append(groupSources, src)
			}
		}
		groupName := selectGroupName(groupSources)
		groupFileInfos := collections.MapSlice(groupSources, func(name string) fileInfo {
			return nameToFileInfo[name]
		})
		groups[groupName] = &sourceGroup{sources: groupFileInfos}
		if len(sourcesGroup) > 1 { // Set subgroups only if multiple groups defined
			groups[groupName].subGroups = sourcesGroup
		}
	}
	return groups
}

// Assigns to each source group a list of its direct dependencies (sourceGroup.dependsOn)
func (groups *sourceGroups) resolveGroupDependencies(graph sourceDependencyGraph) {
	headerToGroupId := make(map[string]groupId)
	for id, group := range *groups {
		for _, file := range group.sources {
			if fileNameIsHeader(file.name) {
				headerToGroupId[file.name] = id
			}
		}
	}

	for _, group := range *groups {
		dependencies := make(map[groupId]bool)
		for _, file := range group.sources {
			depId := fileNameToGroupId(file.name)
			for dep := range graph[depId].adjacency {
				if dep != depId {
					dependencies[dep] = true
				}
			}
		}

		// Convert dependency set to slice
		group.dependsOn = slices.Collect(maps.Keys(dependencies))
	}
}

// Generates a map of sourceFiles and their corresponsing groupId.
// Panics if source file is assigned to multiple groups
func (groups *sourceGroups) sourceToGroupIds() map[string]groupId {
	sourceToGroupId := map[string]groupId{}
	for id, group := range *groups {
		for _, file := range group.sources {
			if previous, exists := sourceToGroupId[file.name]; exists {
				log.Panicf("Inconsistent source groups, file %v assigned to both groups %v and %v", file, previous, id)
			}
			sourceToGroupId[file.name] = id
		}
	}
	return sourceToGroupId
}

// Selects a name for the group based on its lexographically first source file name, prefers headers over remaining kinds of files
// The constructed id is lower-cased file name without the extension suffix
func selectGroupName(files []string) groupId {
	var selectedFile string
	_, hdrs := partitionCSources(files)
	switch len(hdrs) {
	case 0:
		slices.Sort(files)
		selectedFile = files[0]
	case 1:
		selectedFile = hdrs[0]
	default:
		slices.Sort(hdrs)
		selectedFile = hdrs[0]
	}
	return fileNameToGroupId(selectedFile)
}

// Splits the source files into sources and headers
func partitionCSources(files []string) (srcs []string, hdrs []string) {
	for _, file := range files {
		if fileNameIsHeader(file) {
			hdrs = append(hdrs, file)
		} else {
			srcs = append(srcs, file)
		}
	}
	return srcs, hdrs
}

func fileNameIsHeader(name string) bool {
	ext := filepath.Ext(name)
	return slices.Contains(headerExtensions, ext)
}

func fileNameToGroupId(name string) groupId {
	id := strings.TrimSuffix(path.Base(name), filepath.Ext(name))
	return groupId(id)
}

func toRelativePaths(fileInfos []fileInfo) []string {
	names := make([]string, len(fileInfos))
	for i := range fileInfos {
		names[i] = fileInfos[i].name
	}
	return names
}

// Concatenate 2 slices, preserving order but without duplicates
func concatUnique[T comparable](arr1, arr2 []T) []T {
	maxSize := len(arr1) + len(arr2)
	uniqueMap := make(map[T]bool, maxSize)
	result := make([]T, 0, maxSize)

	for _, val := range append(arr1, arr2...) {
		if !uniqueMap[val] {
			uniqueMap[val] = true
			result = append(result, val)
		}
	}

	return result
}
