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
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/EngFlow/gazelle_cc/index/conan/internal/targets"
	"github.com/EngFlow/gazelle_cc/index/internal/bazel"
	"github.com/EngFlow/gazelle_cc/index/internal/bazel/proto"
	"github.com/EngFlow/gazelle_cc/index/internal/collections"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer/cli"

	"github.com/bazelbuild/bazel-gazelle/label"
)

// Creates an index defining mapping between header and the Bazel rule that defines it, based on the Conan Bazel integration.
// The created index can be used as input for gazelle_cc allowing to resolve external dependenices.
func main() {
	install := flag.Bool("install", false, "Should conan deps be installed before indexing")
	conanDir := flag.String("conan_dir", "conan", "Path to conan directory created after running `conan install`")
	flag.Parse()

	callerRoot, err := cli.ResolveWorkingDir()
	if err != nil {
		log.Fatalf("Failed to resolve working directory for indexer")
	}

	outputFile := cli.ResolveOutputFile()

	conanDirectory := *conanDir
	if !filepath.IsAbs(conanDirectory) {
		conanDirectory = filepath.Join(callerRoot, conanDirectory)
	}

	if *install {
		type command struct {
			args    []string
			canFail bool
		}
		for _, command := range []command{
			{
				// Profile needs to be detected once to collect info about os.
				// If it was already detected it would fail with error
				args:    []string{"profile", "detect"},
				canFail: true,
			}, {
				args:    []string{"install", ".", "--build=missing"},
				canFail: false,
			},
		} {
			cmd := exec.Command("conan", command.args...)
			cmd.Dir = callerRoot
			var buf bytes.Buffer
			if *cli.Verbose {
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			} else {
				cmd.Stdout = &buf
				cmd.Stderr = &buf
			}
			log.Printf("Exec %v in %v", cmd.Args, cmd.Dir)
			if cmd.Run() != nil {
				log.Println(buf.String())
				if !command.canFail {
					log.Fatalf("Failed to install conan dependenices")
				}
			}
		}
	}

	// After installation the conanDirectory would contain it's specific rules and a declarations of external modules.
	// Each directory has the same name as external Bazel repository defined in ~/conan2, and contains a matching declarations for it's rules in BUILD.bazel file.
	// Rules in ./conan directory have no sources, that's why we need to query on the external repository instead.
	subdirs, err := listSubdirectories(conanDirectory)
	if err != nil {
		log.Fatalf("Failed to list subdirectories in %s: %v", conanDirectory, err)
	}

	modules := []indexer.Module{}
	for _, dir := range subdirs {
		repoName := dir
		// Search for cc_library in external repository
		result, err := bazel.Query(callerRoot, fmt.Sprintf("kind(cc_library, @%s//...)", repoName))
		if err != nil {
			fmt.Errorf("Bazel query failed: %w", err)
		}
		module := extractIndexerModule(result, repoName)

		// If multiple rules refer to the same headers (typicall in Conan integration) then
		// pick to targets that are on top of dependency chain - does not depend on other rules in group
		selectedTargets := []*indexer.Target{}
		// In conan most of cc_libraries defines filegroup using **/* glob pattern.
		// We need to index only top-level target that depend on all other remaining targets
		for _, intersectingTargets := range targets.GroupTargetsByHeaders(module) {
			roots := targets.SelectRootTargets(intersectingTargets)
			if len(roots) != 1 {
				log.Fatal("Incosistient state, should be only 1 root header")
			}
			// Typically there should be exacly 1 root, but just for sanity let's merge other ones if needed
			root := roots[0]
			for target := range intersectingTargets {
				if target != root {
					root.Hdrs.Join(target.Hdrs)
					root.Includes.Join(target.Includes)
				}
			}
			selectedTargets = append(selectedTargets, root)
		}
		module.Targets = selectedTargets
		modules = append(modules, module)
	}

	indexingResult := indexer.CreateHeaderIndex(modules)
	indexingResult.WriteToFile(outputFile)

	if *cli.Verbose {
		log.Println(indexingResult.String())
	}
}

// Processes bazel query result to extrct cc_library targets as a module
func extractIndexerModule(query proto.QueryResult, moduleName string) indexer.Module {
	targets := []*indexer.Target{}
	for _, info := range query.GetTarget() {
		name, err := label.Parse(info.GetRule().GetName())
		if err != nil {
			log.Printf("Failed to parse queried target label: %v", info.GetRule().GetName())
			continue
		}

		tryParseLabel := func(labelString string) (label.Label, bool) {
			if label, err := label.Parse(labelString); err == nil {
				return label, true
			}
			return label.NoLabel, false
		}

		target := &indexer.Target{
			Name: name,
			Hdrs: collections.ToSet(collections.FilterMap(
				bazel.GetNamedAttribute(info, "hdrs").GetStringListValue(),
				tryParseLabel)),
			Includes:           collections.ToSet(bazel.GetNamedAttribute(info, "includes").GetStringListValue()),
			StripIncludePrefix: bazel.GetNamedAttribute(info, "strip_include_prefix").GetStringValue(),
			IncludePrefix:      bazel.GetNamedAttribute(info, "include_prefix").GetStringValue(),
			Deps: collections.ToSet(collections.FilterMap(
				bazel.GetNamedAttribute(info, "deps").GetStringListValue(),
				tryParseLabel)),
		}
		targets = append(targets, target)
	}
	return indexer.Module{
		Repository: moduleName,
		Targets:    targets,
	}
}

func listSubdirectories(root string) ([]string, error) {
	var dirs []string
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}
