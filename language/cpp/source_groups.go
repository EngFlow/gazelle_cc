package cpp

import (
	"log"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cpp/language/internal/cpp/parser"
)

type groupId string
type sourceGroup struct {
	srcs      []sourceFile
	hdrs      []sourceFile
	dependsOn []groupId
}
type sourceGroups struct {
	groups     map[groupId]*sourceGroup
	unassigned []sourceFile
}

func (g *sourceGroups) groupIds() []groupId {
	ids := make([]groupId, 0, len(g.groups))
	for id := range g.groups {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func groupSourcesByHeaders(sourceFiles []sourceFile, sourceInfos map[sourceFile]parser.SourceInfo) sourceGroups {
	srcs, hdrs := partitionCSources(sourceFiles)

	// First phase: Track dependencies using headers
	graph := buildDependencyGraph(hdrs, sourceInfos)
	sccs := graph.findStronglyConnectedComponents()

	groups := splitIntoSourceGroups(sccs, graph)
	groups.resolveGroupDependencies(graph)

	// Second phase: Assign remaining sources to their respective header groups
	groups.assignSourcesToGroups(srcs, sourceInfos)

	// Sort groups for deterministic output
	groups.sort()
	return groups
}

func (groups *sourceGroups) sort() {
	slices.Sort(groups.unassigned)
	for _, group := range groups.groups {
		slices.Sort(group.srcs)
		slices.Sort(group.hdrs)
		slices.Sort(group.dependsOn)
	}
}

type sourceFileSet map[sourceFile]bool
type sourceDependencyGraph map[sourceFile]sourceFileSet

func buildDependencyGraph(hdrs []sourceFile, sourceInfos sourceInfos) sourceDependencyGraph {
	graph := make(sourceDependencyGraph)
	hdrForBaseName := make(map[string]sourceFile, len(hdrs))

	// Initialize the nodes of a graph using hdrs
	for _, hdr := range hdrs {
		graph[hdr] = make(sourceFileSet)
		// Register the base name of header to allow for quick .cc/.h file pairs lookup
		baseName := hdr.baseName()
		hdrForBaseName[baseName] = hdr
	}

	// Create the edges of the graph based on includes of the file
	for file, info := range sourceInfos {
		// When tracking dependencies we use header files as nodes,
		// but we also include direct dependencies of the corresponding file containing implementation.
		// We need to track dependencies introduced by both of these, otherwise a cyclic dependency can be formed
		var node sourceFile
		if file.isHeader() {
			node = file
		} else {
			baseName := file.baseName()
			correspondingHdr, exists := hdrForBaseName[baseName]
			if !exists {
				continue
			}
			// Create a cyclic dependency between matching .cc <-> .h files to ensure they're always defined in the source group
			node = correspondingHdr
			graph[file] = sourceFileSet{node: true}
			graph[node][file] = true
		}

		for _, include := range info.Includes.DoubleQuote {
			dep := sourceFile(include)
			// Exclude non local headers, these are handled independently as target dependency
			if _, exists := graph[dep]; exists {
				graph[node][dep] = true
			}
		}
	}
	return graph
}

type sourceFileGroup []sourceFile

// Split dependency graph groups using Tarjan’s algorithm to detect SCCs.
func (graph *sourceDependencyGraph) findStronglyConnectedComponents() []sourceFileGroup {
	index := 0
	indices := make(map[sourceFile]int)
	lowLink := make(map[sourceFile]int)
	onStack := make(map[sourceFile]bool)
	var stack []sourceFile
	var sccs []sourceFileGroup

	var strongConnect func(node sourceFile)
	strongConnect = func(node sourceFile) {
		indices[node] = index
		lowLink[node] = index
		index++
		stack = append(stack, node)
		onStack[node] = true

		nodes := *graph
		for dep, _ := range nodes[node] {
			if _, exists := indices[dep]; !exists {
				strongConnect(dep)
				lowLink[node] = min(lowLink[node], lowLink[dep])
			} else if onStack[dep] {
				lowLink[node] = min(lowLink[node], indices[dep])
			}
		}

		if lowLink[node] == indices[node] {
			var scc []sourceFile
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

	for node := range *graph {
		if _, exists := indices[node]; !exists {
			strongConnect(node)
		}
	}

	return sccs
}

func splitIntoSourceGroups(fileGroups []sourceFileGroup, graph sourceDependencyGraph) sourceGroups {
	groups := map[groupId]*sourceGroup{}
	var unassigned []sourceFile

	for _, sourcesGroup := range fileGroups {
		srcs, hdrs := partitionCSources(sourcesGroup)
		if len(hdrs) > 0 {
			groupName := selectGroupName(sourcesGroup, graph)
			groups[groupName] = &sourceGroup{hdrs: hdrs, srcs: srcs}
		} else {
			unassigned = slices.Concat(unassigned, sourcesGroup)
		}
	}
	return sourceGroups{
		groups:     groups,
		unassigned: unassigned,
	}
}

func (groups *sourceGroups) resolveGroupDependencies(graph sourceDependencyGraph) {
	headerToGroupId := make(map[sourceFile]groupId)
	for id := range groups.groups {
		for _, hdr := range groups.groups[id].hdrs {
			headerToGroupId[hdr] = id
		}
	}

	for id, group := range groups.groups {
		dependencies := make(map[groupId]bool)
		// Find dependencies from headers
		for _, hdr := range group.hdrs {
			for dep := range graph[hdr] {
				if depGroup, exists := headerToGroupId[dep]; exists && depGroup != id {
					dependencies[depGroup] = true
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

func (groups *sourceGroups) assignSourcesToGroups(srcs []sourceFile, sourceInfos sourceInfos) {
	if len(srcs) == 0 {
		return
	}

	// First, fill initial groups info based on groups info
	sourceToGroupId := map[sourceFile]groupId{}
	for id, group := range groups.groups {
		for _, file := range slices.Concat(group.srcs, group.hdrs) {
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
				if slices.Contains(group.hdrs, dep) {
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
				group.srcs = append(group.srcs, src)
				sourceToGroupId[src] = id
			}
		} else {
			// If the source includes headers from multiple groups, it remains unassigned.
			groups.unassigned = append(groups.unassigned, src)
		}
	}

	// Any source file not assigned goes into unassigned
	for _, src := range srcs {
		if _, exists := sourceToGroupId[src]; !exists {
			if !slices.Contains(groups.unassigned, src) {
				groups.unassigned = append(groups.unassigned, src)
			}
		}
	}
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
	maxOut := len(dependencyGraph[mostDependant])

	for _, file := range files[1:] {
		out := len(dependencyGraph[file])
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
