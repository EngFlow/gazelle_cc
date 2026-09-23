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

// Creates an index defining a mapping between a header and the Bazel rule that
// defines it, based on the repositories of the current Bazel module. The
// created index can be used as input for gazelle_cc, allowing it to resolve
// external dependencies.
//
// Unlike the bzlmod indexer, which resolves each bazel_dep against the Bazel
// Central Registry, this indexer lists the dependencies of a specific Bazel
// module (using `bazel mod dump_repo_mapping ""`), then indexes their
// `cc_library` targets using `bazel query`. This allows repositories not in the
// BCR (http_archive, overrides, etc) to be resolved.
//
// Like gazelle_cc, this indexer only supports `cc_library`, and not other rules
// that may yield `CcInfo` . Using `bazel cquery` would allow us to use `CcInfo`
// directly, providing more accurate includes for all "C++-like rules", more
// simply. But:
//
//   - It is much less efficient than `bazel query`.
//
//   - Loading `//...` globs as we do here won't work, as some repositories are
//     not valid when used as 3rd party targets (e.g. `no such package
//     '@@[unknown repo 'google_benchmark' requested from @@abseil-cpp+]//'`).
//
//   - It would evaluate `select()`s, so the query would yield different results
//     on different platforms.
//
// The sweet spot is likely a hybrid strategy that discovers potential targets
// using `bazel query`, then analyzes them with `bazel cquery`, but it would
// depart significantly from what the indexers in this repo already do (and be
// more complex), so this indexer uses `bazel query` only instead.
package main

import (
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/EngFlow/gazelle_cc/index/internal/bazel/proto"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer/cli"
	"github.com/EngFlow/gazelle_cc/internal/index"
	"github.com/bazelbuild/bazel-gazelle/label"
)

var (
	includeRepositories = flag.String("repositories", "", "Regexp of repositories to index; all visible repositories are indexed by default")
	excludeRepositories = flag.String("exclude_repositories", "", "Regexp of repositories to exclude from the index")

	preference = flag.String("prefer", preferNarrowest, "Which target to list first for a header exposed by several targets, deciding what gazelle_cc picks under cc_ambiguous_deps=try_first: \"narrowest\" (fewest headers) or \"widest\" (most)")
)

// Values supported by `preference`.
const (
	preferNarrowest = "narrowest"
	preferWidest    = "widest"
)

func main() {
	flag.Parse()

	if *preference != preferNarrowest && *preference != preferWidest {
		log.Fatalf("Invalid -prefer=%v, expected %q or %q", *preference, preferNarrowest, preferWidest)
	}

	var includeRegexp *regexp.Regexp
	var excludeRegexp *regexp.Regexp

	if *includeRepositories != "" {
		compiled, err := regexp.Compile(*includeRepositories)
		if err != nil {
			log.Fatalf("Failed to compile -repositories: %v", err)
		}
		includeRegexp = compiled
	}
	if *excludeRepositories != "" {
		compiled, err := regexp.Compile(*excludeRepositories)
		if err != nil {
			log.Fatalf("Failed to compile -exclude_repositories: %v", err)
		}
		excludeRegexp = compiled
	}

	workingDir, err := cli.ResolveWorkingDir()
	if err != nil {
		log.Fatalf("Failed to resolve working directory for indexer: %v", err)
	}

	repos, err := resolveRepositories(workingDir, includeRegexp, excludeRegexp)
	if err != nil {
		log.Fatalf("Failed to resolve repositories of %v: %v", workingDir, err)
	}
	if len(repos.apparent) == 0 {
		log.Fatalf("No repositories left to index")
	}
	if *cli.Verbose {
		log.Printf("Indexing %d repositories of %v: %v", len(repos.apparent), workingDir, repos.apparent)
	}

	// Querying a repository fetches it if it is not already present, so this
	// command can take a while.
	result, err := queryTargets(workingDir, repos)
	if err != nil {
		log.Fatalf("Failed to query repositories: %v", err)
	}

	modules := buildModules(repos, repos.groupByRepository(result))
	indexingResult := indexer.CreateHeaderIndex(modules)

	outputFile := cli.ResolveOutputFile()
	if !filepath.IsAbs(outputFile) {
		// cli.ResolveOutputFile only joins the working directory when resolving
		// it *fails*. Under `bazel run` the process starts in the runfiles
		// tree, so a relative path would write the file to a directory far from
		// the (apparent) CWD.
		outputFile = filepath.Join(workingDir, outputFile)
	}
	if err := writeIndex(indexingResult, modules, outputFile); err != nil {
		log.Fatalf("Failed to write index to %v: %v", outputFile, err)
	}

	fmt.Printf("Indexed %d headers from %d repositories into %v\n",
		len(indexingResult.HeaderToRule)+len(indexingResult.Ambiguous), len(modules), outputFile)
	if len(indexingResult.Ambiguous) > 0 {
		fmt.Printf("%d of them are exposed by more than one target\n", len(indexingResult.Ambiguous))
	}
	if *cli.Verbose {
		log.Println(indexingResult.String())
	}
}

// buildModules converts each repository's targets into an indexer.Module.
func buildModules(repos repositories, grouped map[string][]*proto.Target) []indexer.Module {
	repositories := slices.Sorted(maps.Keys(grouped))
	modules := make([]indexer.Module, len(grouped))

	// We could wrap this in an `errgroup` computation since each repository can
	// be processed independently, but this whole loop takes a small amount of
	// the overall runtime (dominated by `bazel query`), so no need to bother.
	for i, repository := range repositories {
		modules[i] = repos.buildModule(repository, grouped[repository])
	}

	return slices.DeleteFunc(modules, func(module indexer.Module) bool {
		return len(module.Targets) == 0
	})
}

// writeIndex writes the header index, keeping every candidate for a header that
// several targets expose rather than dropping it the way
// IndexingResult.WriteToFile does.
func writeIndex(result indexer.IndexingResult, modules []indexer.Module, path string) error {
	// Count how many headers each target exposes to resolve ambiguities below.
	targetCount := 0
	for _, module := range modules {
		targetCount += len(module.Targets)
	}
	counts := make(map[label.Label]int, targetCount)
	for _, module := range modules {
		for _, target := range module.Targets {
			// Rebuild the label to match `CreateHeaderIndex()`.
			name := label.New(module.Repository, target.Name.Pkg, target.Name.Name)
			counts[name] += len(target.Hdrs)
		}
	}

	// Generate mapping.
	mappings := make(index.DependencyIndex, len(result.HeaderToRule)+len(result.Ambiguous))
	for header, dep := range result.HeaderToRule {
		mappings[header] = []label.Label{dep}
	}
	for header, candidates := range result.Ambiguous {
		mappings[header] = rankCandidates(candidates, counts)
	}

	// Write as JSON. Note that entries are ordered by key.
	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize header index: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0666)
}

// rankCandidates orders the targets exposing one header, so that the entry
// gazelle_cc picks under `try_first`/`force_first` is both deterministic and
// the likelier of the two to be the intended one.
//
// This is controlled by `preference`:
//
// `narrowest` prefers the smaller target: `absl/strings/string_view.h` is
// exposed by both `@abseil-cpp//absl/strings:string_view` (1 header) and
// `@abseil-cpp//absl/strings` (15), and the former is the one that users
// typically want. `widest` prefers the aggregate instead, for a workspace
// that would rather depend on one umbrella target per library.
//
// Unfortunately, neither answer is always right. `@nlohmann_json` ships the
// same library twice, as many modular headers (`//:json`) and as one amalgamated
// header (`//:singleheader-json`). A user picking "narrowest" typically wants
// smaller dependencies, and thus smaller header includes, but here it would lead
// to the larger header being picked. Returning multiple labels allows the user
// to manually pick the dependency they want in such cases.
//
// Falls back to the shorter label, and then to lexicographic ordering so the
// output is stable.
func rankCandidates(candidates []label.Label, counts map[label.Label]int) []label.Label {
	// Multiplier used to apply the `preference`.
	byCountMult := 1
	if *preference == preferWidest {
		byCountMult = -1
	}

	// Sort and compact.
	ranked := slices.Clone(candidates)
	slices.SortFunc(ranked, func(left, right label.Label) int {
		if byCount := cmp.Compare(counts[left], counts[right]); byCount != 0 {
			return byCount * byCountMult
		}
		leftName, rightName := left.String(), right.String()
		if byLength := cmp.Compare(len(leftName), len(rightName)); byLength != 0 {
			return byLength
		}
		return cmp.Compare(leftName, rightName)
	})
	return slices.Compact(ranked)
}
