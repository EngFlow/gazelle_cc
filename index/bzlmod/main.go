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
	"sync"

	"github.com/EngFlow/gazelle_cc/index/internal/bcr"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer/cli"
	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/buildtools/build"
)

// Creates an index defining mapping between header and the Bazel rule that defines it, based on the Conan Bazel integration.
// The created index can be used as input for gazelle_cc allowing to resolve external dependenices.
func main() {
	moduleBzl := flag.String("module_bazel", "./MODULE.bazel", "Path to MODULE.bazel containg bazel_dep directives")
	flag.Parse()

	callerRoot, err := cli.ResolveWorkingDir()
	if err != nil {
		log.Fatalf("Failed to resolve working directory for indexer")
	}
	log.Printf("Would run in %v", callerRoot)

	moduleBazelPath := *moduleBzl
	if !filepath.IsAbs(moduleBazelPath) {
		moduleBazelPath = filepath.Join(callerRoot, moduleBazelPath)
	}

	bcrConfig := bcr.NewBazelRegistryConfig()
	bcrConfig.Verbose = *cli.Verbose
	bcrClient, err := bcr.CheckoutBazelRegistry(bcrConfig)
	if err != nil {
		log.Fatalf("Failed to checkout Bazel central registry: %v", err)
	}

	if bcrConfig.Verbose {
		log.Printf("Parsing %v to find bazel_dep directives", moduleBazelPath)
	}
	modules := resolveBazelDepModules(moduleBazelPath, bcrClient)
	indexingResult := indexer.CreateHeaderIndex(modules)
	indexingResult.WriteToFile(cli.ResolveOutputFile())

	if *cli.Verbose {
		log.Println(indexingResult.String())
	}
}

func resolveBazelDepModules(moduleBzlPath string, bcrClient bcr.BazelRegistry) []indexer.Module {
	bazelDeps := make(chan BazelDependency)
	resolveResults := make(chan bcr.ResolveModuleInfoResult)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for bazelDep := range bazelDeps {
			result := bcrClient.ResolveModuleInfo(bazelDep.Name, bazelDep.Version)
			if *cli.Verbose {
				switch {
				case result.IsResolved():
					fmt.Fprintf(os.Stderr, "%-50s: resolved - cc_libraries: %d\n", result.Info.Module.String(), len(result.Info.Targets))
				default:
					fmt.Fprintf(os.Stderr, "%-50s: failed   - %s\n", result.Unresolved.Module.String(), result.Unresolved.Reason)
				}
			}
			resolveResults <- result
		}
	}

	workers := runtime.GOMAXPROCS(0)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}

	go func() {
		content, err := os.ReadFile(moduleBzlPath)
		if err != nil {
			log.Fatalf("Failed t read file: %v - %v", moduleBzlPath, err)
		}
		file, err := build.ParseModule(filepath.Base(moduleBzlPath), content)
		if err != nil {
			log.Fatalf("Failed to parse: %v - %v", moduleBzlPath, err)
		}
		for _, bazelDep := range ExtractBazelDependencies(*file) {
			bazelDeps <- bazelDep
		}
		close(bazelDeps)
		wg.Wait()
		close(resolveResults)
	}()

	var resolvedModules []indexer.Module
	var emptyModules []string
	var failed int
	for result := range resolveResults {
		switch {
		case result.IsResolved() && len(result.Info.Targets) > 0:
			if len(result.Info.Targets) == 0 {
				emptyModules = append(emptyModules, result.Info.Module.Name)
				continue
			}
			resolvedModules = append(
				resolvedModules,
				result.Info.ToIndexerModule().WithAmbigiousTargetsResolved(),
			)
		case result.IsUnresolved():
			failed++
		}
	}

	fmt.Printf("Found %d modules with non-empty cc_library defs: %v\n", len(resolvedModules), collections.Map(resolvedModules, func(m indexer.Module) string { return m.Repository }))
	if len(emptyModules) > 0 {
		fmt.Printf("Found %d modules with without cc_library defs: %v\n", emptyModules)

	}
	if failed > 0 {
		fmt.Printf("Failed to gather module information in %d modules\n", failed)
	}

	return resolvedModules
}

type BazelDependency struct {
	Name    string
	Version string
}

func ExtractBazelDependencies(file build.File) []BazelDependency {
	result := []BazelDependency{}
	for _, stmt := range file.Stmt {
		switch tree := stmt.(type) {
		case *build.CallExpr:
			receiver, ok := tree.X.(*build.Ident)
			if !ok {
				continue
			}
			switch receiver.Name {
			case "bazel_dep":
				dep := BazelDependency{}
			parseArg:
				for idx, arg := range tree.List {
					switch arg := arg.(type) {
					case *build.StringExpr:
						switch idx {
						case 0:
							dep.Name = arg.Value
						case 1:
							dep.Version = arg.Value
						}
					case *build.AssignExpr:
						param, ok := arg.LHS.(*build.Ident)
						if !ok {
							continue parseArg
						}
						rhs, ok := arg.RHS.(*build.StringExpr)
						if !ok {
							continue parseArg
						}
						switch param.Name {
						case "name":
							dep.Name = rhs.Value
						case "version":
							dep.Version = rhs.Value
						}
					}
				}
				result = append(result, dep)
			}
		}
	}
	return result
}
