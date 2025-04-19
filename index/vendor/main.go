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
	"path/filepath"
	"strings"

	"github.com/EngFlow/gazelle_cc/index/internal/bazel"
	"github.com/EngFlow/gazelle_cc/index/internal/collections"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
	"github.com/bazelbuild/bazel-gazelle/label"
)

func main() {
	selectors := defaultSelectors
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	output := flag.String("output", "./vendor.ccindex", "Output file path for index")
	flag.Var(&selectors, "select", "Repeated selectors for paths that should be indexed")
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		log.Fatalf("Program requires exactly 1 argument - a path to the caller project directory, typically $PWD. Flags needs to be defined before arguments")
	}
	callerRoot := flag.Arg(0)
	outputFile := *output
	if !filepath.IsAbs(outputFile) {
		outputFile = filepath.Join(callerRoot, outputFile)
	}

	modules := []indexer.Module{}
	for _, selector := range selectors.values {
		query := bazel.Query(callerRoot, fmt.Sprintf("kind('cc_library', %s)", selector))
		if query == nil {
			log.Printf("Bazel query failed for selector: '%s', it would be skipped", selector)
			continue
		}
		modules = append(modules, indexer.Module{
			Repository: "",
			Targets:    collectTargets(query),
		})
	}

	indexingResult := indexer.CreateHeaderIndex(modules)
	indexingResult.WriteToFile(outputFile)

	if *verbose {
		indexingResult.Show()
	}
}

type selectorsList struct {
	values    []string
	isDefault bool
}

var defaultSelectors = selectorsList{
	values:    []string{"//third_party/...", "//external/...", "//vendored/..."},
	isDefault: true,
}

func (s *selectorsList) String() string {
	return strings.Join(s.values, ",")
}

func (s *selectorsList) Set(value string) error {
	if s.isDefault {
		s.values = []string{}
	}
	s.values = append(s.values, value)
	return nil
}

func collectTargets(query *bazel.QueryResult) []*indexer.ModuleTarget {
	targets := []*indexer.ModuleTarget{}
	for _, ccLib := range query.GetTarget() {
		name, err := label.Parse(ccLib.GetRule().GetName())
		if err != nil {
			log.Printf("Failed to parse queried target label: %v", ccLib.GetRule().GetName())
			continue
		}

		target := &indexer.ModuleTarget{
			Name: name,
			Hdrs: collections.ToSet(collections.Collect(
				ccLib.GetNamedAttribute("hdrs").GetStringListValue(),
				label.Parse)),
			Includes:           collections.ToSet(ccLib.GetNamedAttribute("includes").GetStringListValue()),
			StripIncludePrefix: ccLib.GetNamedAttribute("strip_include_prefix").GetStringValue(),
			IncludePrefix:      ccLib.GetNamedAttribute("include_prefix").GetStringValue(),
			Deps: collections.ToSet(collections.Collect(
				ccLib.GetNamedAttribute("deps").GetStringListValue(),
				label.Parse)),
		}
		targets = append(targets, target)
	}
	return targets
}
