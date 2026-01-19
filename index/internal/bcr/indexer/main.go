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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"golang.org/x/sync/errgroup"

	"github.com/EngFlow/gazelle_cc/index/internal/bcr"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
	"github.com/EngFlow/gazelle_cc/internal/collections"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// run is the main entry point for the BCR indexer.
//
// It creates an index of external header includes to the label of the external dependency defining them.
// It processes all modules defined in https://registry.bazel.build/ and extracts information about
// defined headers for public rules using bazel query.
//
// The mapping keys are always in the normalized form of include paths that should be valid when
// referring using #include directive in C/C++ sources assuming include paths were not overridden.
// The values of the mapping is a string representation of Bazel label where the repository is the name of the module.
// Mappings are always based on the last version of module available in the registry.
// If the latest available version is yanked then whole module would be skipped.
//
// The script needs to checkout (download) sources of each module and execute bazel query using a fresh instance of Bazel server.
// This step can be ignored if .cache/modules/ contains extracted module information from previous run.
//
// When processing the results of the query script might exclude targets or headers that are assumed to be internal,
// the excluded files would be written in textual file on the disk.
// Mapping contains only headers that are assigned to exactly 1 rule.
// Headers with ambiguous rule definitions are also written in textual format for manual inspection.
// It does also use system binaries: git, patch (gpatch is required on MacOs instead to correctly apply patches to Bazel modules) and bazel (bazelisk preferred)
func run() error {
	cfg := parseFlags()

	bcrClient, err := bcr.CheckoutBazelRegistry(cfg.bcrConfig)
	if err != nil {
		return fmt.Errorf("failed to checkout bazel registry: %w", err)
	}

	modules, err := gatherModuleInfos(bcrClient)
	if err != nil {
		return fmt.Errorf("failed to resolve modules info: %w", err)
	}

	index := indexer.CreateHeaderIndex(modules)
	if err := index.WriteJSONFile(cfg.outputPath); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}
	if cfg.verbose {
		log.Println(index.Summary())
	}
	return nil
}

type Config struct {
	outputPath string
	verbose    bool
	bcrConfig  bcr.BazelRegistryConfig
}

func parseFlags() Config {
	var cfg Config
	pwd, _ := os.Getwd()
	defaultCache := filepath.Join(pwd, ".cache")
	flag.StringVar(&cfg.outputPath, "output-mappings", filepath.Join(defaultCache, "header-mappings.json"), "Output path for header mappings")
	flag.StringVar(&cfg.bcrConfig.CacheDir, "cache-dir", defaultCache, "Path to cache directory")
	flag.BoolVar(&cfg.verbose, "v", false, "Verbose")
	flag.BoolVar(&cfg.bcrConfig.KeepSources, "keep-sources", false, "Keep fetched sources (default false)")
	flag.BoolVar(&cfg.bcrConfig.RecomputeBad, "recompute-unresolved", false, "Recompute previously unresolved modules (default false)")
	flag.BoolVar(&cfg.bcrConfig.CacheBad, "cache-unresolved", true, "Cache unresolved module results (default true)")
	flag.Parse()
	cfg.bcrConfig.Verbose = cfg.verbose
	return cfg
}

func gatherModuleInfos(bcrClient bcr.BazelRegistry) ([]indexer.Module, error) {
	modulesDir := filepath.Join(bcrClient.RepositoryPath, "modules")
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return nil, err
	}

	// Filter to only directories
	var moduleNames []string
	for _, e := range entries {
		if e.IsDir() {
			moduleNames = append(moduleNames, e.Name())
		}
	}
	fmt.Fprintf(os.Stderr, "Scanning %d modules for cc_rules\n", len(moduleNames))

	// Use semaphore pattern for bounded concurrency
	workerCount := runtime.GOMAXPROCS(0)
	sem := make(chan struct{}, workerCount)
	results := make([]bcr.ResolveModuleInfoResult, len(moduleNames))

	var eg errgroup.Group
	for i, moduleName := range moduleNames {
		sem <- struct{}{} // acquire semaphore
		eg.Go(func() error {
			defer func() { <-sem }() // release semaphore

			rr := bcrClient.ResolveModuleInfo(moduleName, "") // implicitly latest version
			results[i] = rr

			if bcrClient.Config.Verbose {
				if rr.Info != nil {
					fmt.Fprintf(os.Stderr, "%-50s: resolved - cc_libraries: %d\n", rr.Info.Module.String(), len(rr.Info.Targets))
				} else {
					fmt.Fprintf(os.Stderr, "%-50s: failed   - %s\n", rr.Unresolved.Module.String(), rr.Unresolved.Reason)
				}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// Collect successful results
	var infos []bcr.ModuleInfo
	var failed int
	for _, r := range results {
		if r.IsResolved() && len(r.Info.Targets) > 0 {
			infos = append(infos, *r.Info)
		} else if r.IsUnresolved() {
			failed++
		}
	}

	fmt.Fprintf(os.Stderr, "Found %d modules with non-empty cc_library defs\n", len(infos))
	fmt.Fprintf(os.Stderr, "Failed to gather module information in %d modules\n", failed)
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Module.Name == infos[j].Module.Name {
			return infos[i].Module.Version < infos[j].Module.Version
		}
		return infos[i].Module.Name < infos[j].Module.Name
	})
	modules := collections.MapSlice(infos, func(m bcr.ModuleInfo) indexer.Module {
		return m.ToIndexerModule().WithAmbiguousTargetsResolved()
	})
	return modules, nil
}
