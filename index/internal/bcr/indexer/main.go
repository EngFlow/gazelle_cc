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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"

	"github.com/EngFlow/gazelle_cc/internal/collections"

	"github.com/EngFlow/gazelle_cc/index/internal/bcr"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
)

// Script responsible for creating index of external header include to the label of extern dependency defining it's rule.
// It does process all modules defined in https://registry.bazel.build/ and extracts information about defined header for public rules using bazel query.
// The mapping keys are always in the normalized form of include paths that should be valid when refering using #include directive in C/C++ sources assuming include paths were not overriden
// The values of the mapping is a string representation on Bazel label where the repository is the name of the module
// Mappings are always based on the last version of module available in the registry. If the latest available version is yanked then whole module would be skipped.
//
// The script needs to checkout (download) sources of each module and execute bazel query using a fresh instance of Bazel server.
// This step can be ignored if .cache/modules/ contains extracted module informations from previous run.
//
// When processing the results of the query script might exclude targets or headers that are assumed to be internal, the excluded files would be written in textual file on the disk.
// Mapping contains only headers that are assigned to exactly 1 rule. Header with ambigious rule definitions are also written in textual format for manual inspection.
// It does also use system binaries: git, patch (gpatch is required on MacOs instead to correctly apply patches to Bazel modules) and bazel (bazelisk preferred)
func main() {
	cfg := parseFlags()

	bcrClient, err := bcr.CheckoutBazelRegistry(cfg.bcrConfig)
	if err != nil {
		log.Fatalf("Failed to checkout bazel registry: %v", err)
	}

	modules, err := gatherModuleInfos(bcrClient)
	if err != nil {
		log.Fatalf("failed to resolve modules info: %v", err)
	}

	// if cfg.verbose {
	// 	showModuleInfos(modules)
	// }
	index := indexer.CreateHeaderIndex(modules)
	fmt.Printf("Direct mapping created for %d headers\n", len(index.HeaderToRule))
	fmt.Printf("Ambigious header assignment for %d entries\n", len(index.Ambiguous))
	// var exclCount int
	// for _, v := range mapping.Excluded {
	// 	exclCount += len(v)
	// }
	// fmt.Printf("Excluded %d headers in %d targets\n", exclCount, len(mapping.Excluded))
	index.WriteToFile(cfg.outputPath)
	if cfg.verbose {
		log.Println(index.String())
	}
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
	fmt.Printf("Scanning %d modules for cc_rules\n", len(entries))

	moduleNames := make(chan string)
	moduleInfosResults := make(chan bcr.ResolveModuleInfoResult)

	workers := runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for moduleName := range moduleNames {
			rr := bcrClient.ResolveModuleInfo(moduleName, "") // implicitlly latest version
			if bcrClient.Config.Verbose {
				if rr.Info != nil {
					fmt.Fprintf(os.Stderr, "%-50s: resolved - cc_libraries: %d\n", rr.Info.Module.String(), len(rr.Info.Targets))
				} else {
					fmt.Fprintf(os.Stderr, "%-50s: failed   - %s\n", rr.Unresolved.Module.String(), rr.Unresolved.Reason)
				}
			}
			moduleInfosResults <- rr
		}
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}

	go func() {
		for _, e := range entries {
			if e.IsDir() {
				moduleNames <- e.Name()
			}
		}
		close(moduleNames)
		wg.Wait()
		close(moduleInfosResults)
	}()

	var infos []bcr.ModuleInfo
	var failed int
	for r := range moduleInfosResults {
		if r.IsResolved() && len(r.Info.Targets) > 0 {
			infos = append(infos, *r.Info)
		} else if r.IsUnresolved() {
			failed++
		}
	}
	fmt.Printf("Found %d modules with non-empty cc_library defs\n", len(infos))
	fmt.Printf("Failed to gather module information in %d modules\n", failed)
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Module.Name == infos[j].Module.Name {
			return infos[i].Module.Version < infos[j].Module.Version
		}
		return infos[i].Module.Name < infos[j].Module.Name
	})
	modules := collections.Map(infos, func(m bcr.ModuleInfo) indexer.Module {
		return m.ToIndexerModule().WithAmbigiousTargetsResolved()
	})
	return modules, nil
}
