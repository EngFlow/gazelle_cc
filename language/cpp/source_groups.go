package cpp

import (
	"log"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cpp/language/internal/cpp/parser"
)

type groupId string
type sourceGroup struct {
	sources   []sourceFile
	dependsOn []groupId
}
type sourceGroups struct {
	groups     map[groupId]*sourceGroup
	unassigned []sourceFile
}

func (g *sourceGroups) groupIds() []groupId {
	ids := slices.Collect(maps.Keys(g.groups))
	slices.Sort(ids)
	return ids
}

func groupSourcesByHeaders(sourceInfos map[sourceFile]parser.SourceInfo) sourceGroups {
	// First phase: Track dependencies using headers
	graph := buildDependencyGraph(sourceInfos)
	sccs := graph.findStronglyConnectedComponents()

	groups := splitIntoSourceGroups(sccs, graph)
	groups.resolveGroupDependencies(graph)

	// Second phase: Assign remaining sources to their respective header groups
	groups.mergeUnassignedSourcesToGroups(sourceInfos)

	// Sort groups for deterministic output
	groups.sort()
	return groups
}

func (groups *sourceGroups) sort() {
	slices.Sort(groups.unassigned)
	for _, group := range groups.groups {
		slices.Sort(group.sources)
		slices.Sort(group.dependsOn)
	}
}

type sourceFileSet map[sourceFile]bool
type sourceGroupNode struct {
	sources   sourceFileSet
	adjacency sourceFileSet
}
type sourceDependencyGraph map[groupId]sourceGroupNode

func buildDependencyGraph(sourceInfos sourceInfos) sourceDependencyGraph {
	graph := make(sourceDependencyGraph)

	// Initialize the nodes of a graph using hdrs
	for src := range sourceInfos {
		groupId := src.toGroupId()
		graph[groupId] = sourceGroupNode{
			sources:   make(sourceFileSet),
			adjacency: make(sourceFileSet)}
	}

	// Create the edges of the graph based on includes of the file
	for file, info := range sourceInfos {
		// When tracking dependencies we use header files as nodes,
		// but we also include direct dependencies of the corresponding file containing implementation.
		// We need to track dependencies introduced by both of these, otherwise a cyclic dependency can be formed
		node := file.toGroupId()
		graph[node].sources[file] = true

		for _, include := range info.Includes.DoubleQuote {
			dep := sourceFile(include)
			// Exclude non local headers, these are handled independently as target dependency
			if _, exists := graph[dep.toGroupId()]; exists {
				graph[node].adjacency[dep] = true
			}
		}
	}
	return graph
}

// Split dependency graph groups using Tarjan’s algorithm to detect SCCs.
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

func splitIntoSourceGroups(fileGroups [][]groupId, graph sourceDependencyGraph) sourceGroups {
	groups := map[groupId]*sourceGroup{}
	var unassigned []sourceFile

	for _, sourcesGroup := range fileGroups {
		var groupSources []sourceFile
		for _, groupId := range sourcesGroup {
			for src := range graph[groupId].sources {
				groupSources = append(groupSources, src)
			}
		}
		if slices.ContainsFunc(groupSources, func(file sourceFile) bool { return file.isHeader() }) {
			groupName := selectGroupName(groupSources, graph)
			groups[groupName] = &sourceGroup{sources: groupSources}
		} else {
			unassigned = slices.Concat(unassigned, groupSources)
		}
	}
	return sourceGroups{
		groups:     groups,
		unassigned: unassigned,
	}
}

func (groups *sourceGroups) resolveGroupDependencies(graph sourceDependencyGraph) {
	headerToGroupId := make(map[sourceFile]groupId)
	for id, group := range groups.groups {
		for _, file := range group.sources {
			if file.isHeader() {
				headerToGroupId[file] = id
			}
		}
	}

	for id, group := range groups.groups {
		dependencies := make(map[groupId]bool)
		// Find dependencies from headers
		for _, file := range group.sources {
			if file.isHeader() {
				depId := file.toGroupId()
				for dep := range graph[depId].adjacency {
					if depGroup, exists := headerToGroupId[dep]; exists && depGroup != id {
						dependencies[depGroup] = true
					}
				}
			}
		}

		// Convert dependency set to slice
		groupDependencyIds := make([]groupId, 0, len(dependencies))
		for k := range dependencies {
			groupDependencyIds = append(groupDependencyIds, k)
		}
		group.dependsOn = groupDependencyIds
	}
}

func (groups *sourceGroups) mergeUnassignedSourcesToGroups(sourceInfos sourceInfos) {
	srcs := groups.unassigned
	if len(srcs) == 0 {
		return
	}
	unassigned := make(map[sourceFile]bool)

	// First, fill initial groups info based on groups info
	sourceToGroupId := map[sourceFile]groupId{}
	for id, group := range groups.groups {
		for _, file := range group.sources {
			if previous, exists := sourceToGroupId[file]; exists {
				log.Panicf("Inconsistent source groups, file %v assigned to both groups %v and %v", file, previous, id)
			}
			sourceToGroupId[file] = id
		}
	}

	// Then, assign remaining sources based on direct inclusion
	for _, src := range srcs {
		if _, exists := sourceToGroupId[src]; exists {
			continue // already assigned
		}

		dependsOnGroup := map[groupId]bool{}
		for _, include := range sourceInfos[src].Includes.DoubleQuote {
			dep := sourceFile(include)
			for id, group := range groups.groups {
				if slices.Contains(group.sources, dep) {
					dependsOnGroup[id] = true
				}
			}
		}

		// Exclude transitive dependencies
		for id := range dependsOnGroup {
			for checkedGroupId := range dependsOnGroup {
				if id != checkedGroupId {
					if groups.isTransitiveDependency(checkedGroupId, id) {
						delete(dependsOnGroup, id)
					}
				}
			}
		}

		// If the source is included in exactly one group, assign it to that group.
		if len(dependsOnGroup) == 1 {
			for id := range dependsOnGroup {
				group := groups.groups[id]
				group.sources = append(group.sources, src)
				sourceToGroupId[src] = id
			}
		} else {
			// If the source includes headers from multiple groups, it remains unassigned.
			unassigned[src] = true
		}
	}
	groups.unassigned = slices.Collect(maps.Keys(unassigned))
}

func (groups *sourceGroups) isTransitiveDependency(id groupId, checkedGroupId groupId) bool {
	group, exists := groups.groups[id]
	if !exists {
		return false
	}
	// Check direct dependencies before traversing transitive deps
	if slices.Contains(group.dependsOn, checkedGroupId) {
		return true
	}
	for _, directDependency := range group.dependsOn {
		if groups.isTransitiveDependency(directDependency, checkedGroupId) {
			return true
		}
	}
	return false
}

// selectGroupName picks a base header with the highest out-degree.
func selectGroupName(files []sourceFile, dependencyGraph sourceDependencyGraph) groupId {
	mostDependant := files[0]
	mostDependantAlts := []sourceFile{}
	maxOut := len(dependencyGraph[mostDependant.toGroupId()].adjacency)

	for _, file := range files[1:] {
		out := len(dependencyGraph[file.toGroupId()].adjacency)
		if out > maxOut {
			mostDependant = file
			mostDependantAlts = nil
			maxOut = out
		} else if out == maxOut {
			mostDependantAlts = append(mostDependantAlts, file)
		}
	}

	selectedFiles := append(mostDependantAlts, mostDependant)
	slices.Sort(selectedFiles)
	groupName := strings.ToLower(selectedFiles[0].baseName())
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
	return groupId(s.baseName())
}

func sourceFilesToStrings(files []sourceFile) []string {
	strings := make([]string, len(files))
	for idx, value := range files {
		strings[idx] = value.stringValue()
	}
	return strings
}
