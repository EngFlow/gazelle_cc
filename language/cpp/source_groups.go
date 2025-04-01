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
	"log"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cpp/language/internal/cpp/parser"
)

// groupId represents a unique identifier for a group of source files
type groupId string

// sourceGroup represents a collection of source files and their dependencies
type sourceGroup struct {
	sources   []sourceFile
	dependsOn []groupId // Direct dependencies of this group (only used internally for testing)
	subGroups []groupId // Sub-groups creating this group
}

// sourceGroups is a mapping of groupIds to their corresponding sourceGroups
type sourceGroups map[groupId]*sourceGroup

// Returns a source group by assigning files based on their filename (excluding extension)
// without analyzing dependencies between sources
func identitySourceGroups(srcs []sourceFile) sourceGroups {
	srcGroups := make(sourceGroups)
	for _, src := range srcs {
		srcGroups[src.toGroupId()] = &sourceGroup{sources: []sourceFile{src}}
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
		slices.Sort(group.sources)
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
func groupSourcesByUnits(sources []sourceFile, sourceInfos map[sourceFile]parser.SourceInfo) sourceGroups {
	graph := buildDependencyGraph(sources, sourceInfos)
	sccs := graph.findStronglyConnectedComponents()
	groups := splitIntoSourceGroups(sccs, graph)
	groups.resolveGroupDependencies(graph)
	groups.sort()             // Ensure deterministic output
	groups.sourceToGroupIds() // Consistency check

	return groups
}

type sourceFileSet map[sourceFile]bool

// represents a node in the dependency graph.
type sourceGroupNode struct {
	sources   sourceFileSet
	adjacency sourceFileSet // Direct dependencies of this node
}

// sourceDependencyGraph represents a directed graph of source dependencies
type sourceDependencyGraph map[groupId]sourceGroupNode

// Source file (.cc) and it's corresponsing header are always grouped together and become a node in a dependency graph.
// Nodes of the graph are constructed base on sources having the same name (excluding extension suffix)
// Edges of the dependency graph are constructed based on include directives to local headers defined in sources of the graph node
func buildDependencyGraph(sourceFiles []sourceFile, sourceInfos map[sourceFile]parser.SourceInfo) sourceDependencyGraph {
	graph := make(sourceDependencyGraph)

	// Initialize graph nodes
	for _, src := range sourceFiles {
		groupId := src.toGroupId()
		graph[groupId] = sourceGroupNode{
			sources:   make(sourceFileSet),
			adjacency: make(sourceFileSet)}
	}

	// Create edges based on include dependencies
	for _, file := range sourceFiles {
		info := sourceInfos[file]
		node := file.toGroupId()
		graph[node].sources[file] = true
		for _, include := range info.Includes.DoubleQuote {
			// Exclude non local headers, these are handled independently as target dependency
			// The include can be either workspace relative or source file relative
			for _, baseDir := range []string{"", path.Dir(file.stringValue())} {
				dep := newSourceFile(baseDir, include)
				if _, exists := graph[dep.toGroupId()]; exists {
					graph[node].adjacency[dep] = true
					break
				}
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
		for sourceFile := range nodes[node].adjacency {
			dep := sourceFile.toGroupId()
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
func splitIntoSourceGroups(fileGroups [][]groupId, graph sourceDependencyGraph) sourceGroups {
	groups := make(sourceGroups, len(fileGroups))

	for _, sourcesGroup := range fileGroups {
		var groupSources []sourceFile
		for _, groupId := range sourcesGroup {
			for src := range graph[groupId].sources {
				groupSources = append(groupSources, src)
			}
		}
		groupName := selectGroupName(groupSources)
		groups[groupName] = &sourceGroup{sources: groupSources}
		if len(sourcesGroup) > 1 { // Set subgroups only if multiple groups defined
			groups[groupName].subGroups = sourcesGroup
		}
	}
	return groups
}

// Assigns to each source group a list of its direct dependencies (sourceGroup.dependsOn)
func (groups *sourceGroups) resolveGroupDependencies(graph sourceDependencyGraph) {
	headerToGroupId := make(map[sourceFile]groupId)
	for id, group := range *groups {
		for _, file := range group.sources {
			if file.isHeader() {
				headerToGroupId[file] = id
			}
		}
	}

	for id, group := range *groups {
		dependencies := make(map[groupId]bool)
		for _, file := range group.sources {
			depId := file.toGroupId()
			for dep := range graph[depId].adjacency {
				if depGroup, exists := headerToGroupId[dep]; exists && depGroup != id {
					dependencies[depGroup] = true
				}
			}
		}

		// Convert dependency set to slice
		group.dependsOn = slices.Collect(maps.Keys(dependencies))
	}
}

// Generates a map of sourceFiles and their corresponsing groupId.
// Panics if source file is assigned to multiple groups
func (groups *sourceGroups) sourceToGroupIds() map[sourceFile]groupId {
	sourceToGroupId := map[sourceFile]groupId{}
	for id, group := range *groups {
		for _, file := range group.sources {
			if previous, exists := sourceToGroupId[file]; exists {
				log.Panicf("Inconsistent source groups, file %v assigned to both groups %v and %v", file, previous, id)
			}
			sourceToGroupId[file] = id
		}
	}
	return sourceToGroupId
}

// Selects a name for the group based on its lexographically first source file name, prefers headers over remaining kinds of files
// The constructed id is lower-cased file name without the extension suffix
func selectGroupName(files []sourceFile) groupId {
	var selectedFile sourceFile
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
	groupName := strings.ToLower(selectedFile.baseName())
	return groupId(groupName)
}

// Splits the source files into sources and headers
func partitionCSources(files []sourceFile) (srcs []sourceFile, hdrs []sourceFile) {
	for _, file := range files {
		if file.isHeader() {
			hdrs = append(hdrs, file)
		} else {
			srcs = append(srcs, file)
		}
	}
	return srcs, hdrs
}

func (file *sourceFile) isHeader() bool {
	ext := filepath.Ext(string(*file))
	return slices.Contains(headerExtensions, ext)
}

func (s *sourceFile) baseName() string {
	name := string(*s)
	return strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
}

func (s *sourceFile) stringValue() string {
	return string(*s)
}

func (s *sourceFile) toGroupId() groupId {
	name := string(*s)
	id := strings.TrimSuffix(name, filepath.Ext(name))
	return groupId(id)
}

func toRelativePaths(dir string, files []sourceFile) []string {
	relPaths := make([]string, len(files))
	for idx, value := range files {
		path, err := filepath.Rel(dir, value.stringValue())
		if err != nil {
			log.Panicf("Cannot relativize: %v - %v", dir, value)
		}
		relPaths[idx] = path
	}
	return relPaths
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
