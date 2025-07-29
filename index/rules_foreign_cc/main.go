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
	"strings"

	"github.com/EngFlow/gazelle_cc/index/internal/bazel"
	"github.com/EngFlow/gazelle_cc/index/internal/bazel/proto"
	"github.com/EngFlow/gazelle_cc/index/internal/collections"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer/cli"
	"github.com/bazelbuild/bazel-gazelle/label"
)

// Creates an index defining mapping between header and the Bazel rule that defines it, based on the `rules_foreign_cc` definitions found in the project.
// The created index can be used as input for gazelle_cc allowing to resolve external dependenices.
func main() {
	// Flags registered implicitlly by import of indexer/cli
	flag.Parse()
	workdir, err := cli.ResolveWorkingDir()
	if err != nil {
		log.Fatalf("Failed to resolve working directory, %w", err)
	}
	outputFile := cli.ResolveOutputFile()

	defsQuery, err := bazel.Query(workdir, "kind('cmake|configure_make|make|ninja', //...)")
	if err != nil {
		log.Fatal("Bazel query failed, unable to index foreign_cc rules")
	}
	modules := []indexer.Module{}
	for _, foreignDefn := range defsQuery.GetTarget() {
		if module := collectModuleInfo(workdir, foreignDefn); module != nil {
			modules = append(modules, *module)
		}
	}

	indexingResult := indexer.CreateHeaderIndex(modules)
	indexingResult.WriteToFile(outputFile)

	if *cli.Verbose {
		log.Println(indexingResult.String())
	}
}

func collectModuleInfo(workdir string, foreignDefn *proto.Target) *indexer.Module {
	targets := []*indexer.Target{}
	libSource := bazel.GetNamedAttribute(foreignDefn, "lib_source").GetStringValue()
	includeDir := bazel.GetNamedAttribute(foreignDefn, "out_include_dir").GetStringValue()
	if *cli.Verbose {
		log.Printf("Processing foreign_cc rule %v: %v", foreignDefn.GetRule().GetRuleClass(), foreignDefn.GetRule().GetName())
	}
	if libSource == "" {
		log.Printf("Cannot resolve 'lib_source' attr in %v: %v, target would be skipped", foreignDefn.GetRule().GetRuleClass(), foreignDefn.GetRule().GetName())
		return nil
	}

	tryParseLabel := func(labelString string) (label.Label, bool) {
		if parsed, err := label.Parse(labelString); err != nil {
			return label.NoLabel, false
		} else {
			return parsed, true
		}
	}

	hdrs := collections.Set[label.Label]{}
	if sourcesQuery, err := bazel.Query(workdir, libSource); err != nil {
		log.Printf("Failed to query for details for lib_source %v: %w", libSource, err)
	} else {
		for _, sourcesTarget := range sourcesQuery.GetTarget() {
			switch sourcesTarget.GetRule().GetRuleClass() {
			case "filegroup":
				for _, src := range collections.FilterMap(bazel.GetNamedAttribute(sourcesTarget, "srcs").GetStringListValue(), tryParseLabel) {
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
	}

	if depsQuery, err := bazel.ConfiguredQuery(workdir,
		fmt.Sprintf("kind(cc_library, rdeps(//..., %s, 1))", foreignDefn.GetRule().GetName()),
		bazel.QueryConfig{KeepGoing: true},
	); err != nil {
		log.Printf("Failed to found direct dependanant of %v:%v", foreignDefn.GetRule().GetRuleClass(), foreignDefn.GetRule().GetName())
		return nil
	} else {
		for _, ccLib := range depsQuery.GetTarget() {
			libName, err := label.Parse(ccLib.GetRule().GetName())
			if err != nil {
				continue
			}
			targets = append(targets, &indexer.Target{
				Name: libName,
				Hdrs: *hdrs.Join(
					collections.ToSet(collections.FilterMap(
						bazel.GetNamedAttribute(ccLib, "hdrs").GetStringListValue(),
						tryParseLabel))),
				Includes: collections.SetOf(includeDir),
				Deps: collections.ToSet(collections.FilterMap(
					bazel.GetNamedAttribute(ccLib, "deps").StringListValue,
					tryParseLabel)),
			})
		}
	}
	return &indexer.Module{
		Repository: "",
		Targets:    targets,
	}
}
