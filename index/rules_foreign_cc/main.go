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

// Creates an index defining mapping between header and the Bazel rule that defines it, based on the `rules_foreign_cc` definitions found in the project.
// The created index can be used as input for gazelle_cc allowing to resolve external dependenices.
func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	output := flag.String("output", "rules_foreign.ccindex", "Output file path for index")
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
	defsQuery := bazel.Query(callerRoot, "kind('cmake|configure_make|make|ninja', //...)")
	if defsQuery == nil {
		log.Fatal("Bazel query failed, unable to index foreign_cc rules")
	}
	modules := []indexer.Module{}
	for _, foreignDefn := range defsQuery.GetTarget() {
		if module := collectModuleInfo(callerRoot, foreignDefn); module != nil {
			modules = append(modules, *module)
		}
	}

	indexingResult := indexer.CreateHeaderIndex(modules)
	indexingResult.WriteToFile(outputFile)

	if *verbose {
		indexingResult.Show()
	}
}

func collectModuleInfo(callerRoot string, foreignDefn *bazel.Target) *indexer.Module {
	targets := []*indexer.ModuleTarget{}
	libSource := foreignDefn.GetNamedAttribute("lib_source").GetStringValue()
	includeDir := foreignDefn.GetNamedAttribute("out_include_dir").GetStringValue()

	hdrs := collections.Set[label.Label]{}
	sourcesQuery := bazel.Query(callerRoot, libSource)
	for _, sourcesTarget := range sourcesQuery.GetTarget() {
		switch sourcesTarget.GetRule().GetRuleClass() {
		case "filegroup":
			for _, src := range collections.Collect(sourcesTarget.GetNamedAttribute("srcs").GetStringListValue(), label.Parse) {
				if strings.HasPrefix(src.Name, includeDir) || strings.HasPrefix(src.Pkg, includeDir) {
					hdrs.Add(src)
				}
			}
		default:
			log.Printf("Unsupported kind of lib_source attribute %v:%v referenced in %v:%v, this target would not be indexed",
				sourcesTarget.GetRule().GetRuleClass(), sourcesTarget.GetRule().GetName(),
				foreignDefn.GetRule().GetRuleClass(), foreignDefn.GetRule().GetName())
		}
	}

	depsQuery := bazel.Query(callerRoot, fmt.Sprintf("kind(cc_library, rdeps(//..., %s, 1))", foreignDefn.GetRule().GetName()))
	if depsQuery == nil {
		log.Printf("Failed to found direct dependanant of %v:%v", foreignDefn.GetRule().GetRuleClass(), foreignDefn.GetRule().GetName())
		return nil
	}
	for _, ccLib := range depsQuery.GetTarget() {
		libName, err := label.Parse(ccLib.GetRule().GetName())
		if err != nil {
			continue
		}
		targets = append(targets, &indexer.ModuleTarget{
			Name: libName,
			Hdrs: *hdrs.Join(
				collections.ToSet(collections.Collect(
					ccLib.GetNamedAttribute("hdrs").GetStringListValue(),
					label.Parse))),
			Includes: collections.Set[string]{includeDir: true},
			Deps: collections.ToSet(collections.Collect(
				ccLib.GetNamedAttribute("deps").StringListValue,
				label.Parse)),
		})
	}
	return &indexer.Module{
		Repository: "",
		Targets:    targets,
	}
}
