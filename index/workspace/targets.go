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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/EngFlow/gazelle_cc/index/internal/bazel"
	"github.com/EngFlow/gazelle_cc/index/internal/bazel/proto"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/label"
)

// QueryTargets runs a `bazel query` yielding all indexable targets in `repos`.
func QueryTargets(workingDir string, repos Repositories) (*proto.QueryResult, error) {
	if len(repos.Apparent) == 0 {
		return nil, errors.New("no repositories to index")
	}

	// Only public targets are indexable, except for `proto_library` rules,
	// which are needed to find the sources of a public `cc_proto_library`.
	query := fmt.Sprintf(
		// Keep `kind()`s in sync with `splitTargets()`.
		`let universe = @%s//... in `+
			`(kind("cc_.*library|alias", $universe) intersect attr(visibility, "//visibility:public", $universe)) `+
			`union kind("filegroup|proto_library", $universe)`,
		strings.Join(repos.Apparent, "//... + @"),
	)

	// Keep going, so if a repository fails to load we may still compute the
	// index.
	result, err := bazel.ConfiguredQuery(workingDir, query, bazel.QueryConfig{KeepGoing: true})
	if err != nil {
		return nil, fmt.Errorf("failed to query workspace: %w", err)
	}
	return &result, nil
}

// GroupByRepository groups the queried targets by the repository defining them.
func (r Repositories) GroupByRepository(result *proto.QueryResult) map[string][]*proto.Target {
	grouped := make(map[string][]*proto.Target, len(r.Apparent))
	for _, target := range result.GetTarget() {
		rule := target.GetRule()
		if rule == nil {
			continue
		}
		name, ok := r.ParseLabel(rule.GetName())
		if !ok || name.Repo == "" {
			continue
		}
		grouped[name.Repo] = append(grouped[name.Repo], target)
	}
	return grouped
}

// BuildModule turns the targets of a single repository into an indexer.Module.
func (r Repositories) BuildModule(repository string, targets []*proto.Target) indexer.Module {
	aliases, filegroups, protoLibraries, ccLibraries := r.splitTargets(targets)

	indexed := make([]indexer.Target, 0, len(ccLibraries))
	for _, ccLib := range ccLibraries {
		target := ccLib.target
		name := ccLib.name

		deps := collections.SetOf[label.Label]()
		for _, dep := range r.labelListAttr(target, "deps") {
			deps.Add(dep.Rel(name.Repo, name.Pkg))
		}

		// A `cc_proto_library` has no `hdrs`, so we must handle it manually
		// using its `cc_library`.
		if ccLib.ruleClass == "cc_proto_library" {
			// `indexer.IndexableIncludePaths()` joins headers after their
			// target's package, so we add "." to `Includes`.
			indexed = append(indexed, indexer.Target{
				Name:     preferAlias(name, repository, aliases),
				Hdrs:     protoHeaders(name, r.labelListAttr(target, "deps"), protoLibraries),
				Includes: collections.SetOf("."),
				Deps:     deps,
			})
			continue
		}

		hdrs := collections.SetOf[label.Label]()
		for _, hdr := range r.labelListAttr(target, "hdrs") {
			for _, resolved := range resolveSource(hdr, name, filegroups) {
				hdrs.Add(resolved)
			}
		}

		name = preferAlias(name, repository, aliases)

		includes := collections.ToSet(stringListAttr(target, "includes"))
		stripIncludePrefix, rootRelativeStrip := stripPrefixOf(target, name)
		if rootRelativeStrip != "" {
			includes.Add(rootRelativeStrip)
		}

		var includePrefix string
		if value, ok := stringAttr(target, "include_prefix"); ok {
			includePrefix = value
		}

		indexed = append(indexed, indexer.Target{
			Name:               name,
			Hdrs:               hdrs,
			Includes:           includes,
			StripIncludePrefix: stripIncludePrefix,
			IncludePrefix:      includePrefix,
			Deps:               deps,
		})
	}

	return indexer.Module{Repository: repository, Targets: indexed}
}

// stripPrefixOf reads `strip_include_prefix`, mapping it into either a
// package-relative path (understood by `indexer.IndexableIncludePaths()`) or,
// if the path is actually absolute, into a root-relative path.
//
// A leading slash for this attribute means "relative to the repository root"
// rather than to the package. Since the indexer only understands relative paths,
// we transform absolute paths into includes with relative paths.
func stripPrefixOf(target *proto.Target, name label.Label) (packageRelative, rootRelative string) {
	value, ok := stringAttr(target, "strip_include_prefix")
	if !ok {
		return "", ""
	}
	if !strings.HasPrefix(value, "/") {
		return value, ""
	}

	root := value[1:]
	if root == "" {
		root = "."
	}
	pkg := name.Pkg
	if pkg == "" {
		pkg = "."
	}
	relative, err := filepath.Rel(pkg, root)
	if err != nil {
		return "", ""
	}
	return "", filepath.ToSlash(relative)
}

// preferAlias returns the label to map a header to, preferring aliases
// either when they are the repository's main target (e.g. `@fmt//:fmt`) or when
// they are a shorter way to spell the same thing.
//
// Only within one package, though: `indexer.IndexableIncludePaths()` derives
// include paths from the package of the label it is given, so swapping in an
// alias from elsewhere would rewrite the paths too: taking `@protobuf//:json`
// for `@protobuf//src/google/protobuf/json` indexes its header as "json.h"
// rather than "google/protobuf/json/json.h".
//
// That restriction is not strictly needed for a `cc_proto_library`, whose
// header paths come from the `proto_library` it wraps (so e.g.
// `@protobuf//:timestamp_cc_proto` would work). We use it anyway to make
// it easier to reason about.
//
// An alias can also help transition off a deprecated rule, so something we
// _shouldn't_ use, but alas, there is no good way to know, so we prefer it
// when possible.
func preferAlias(name label.Label, repository string, aliases map[label.Label]label.Label) label.Label {
	alias, aliased := aliases[name]
	if !aliased || alias.Pkg != name.Pkg {
		return name
	}
	if alias.Name == repository || len(alias.String()) < len(name.String()) {
		return alias
	}
	return name
}

// isHiddenPackage returns whether a target lives under a package segment that
// is conventionally hidden, i.e. one starting with ".".
//
// This was added to ignore the `.tmp_git_root` directory that
// `git_repository()` creates in the presence of a `strip_prefix`. In general,
// it's unlikely that any target in a `.`-prefixed package would be
// intended to be included by an external module, so all such targets are
// excluded.
//
// Note that this does not filter on the semantic name of the package (e.g.
// `third_party`), unlike the BCR indexer.
func isHiddenPackage(target label.Label) bool {
	return strings.HasPrefix(target.Pkg, ".") || strings.Contains(target.Pkg, "/.")
}

// ccLibrary stores information about a `cc_library` yielded by
// `splitTargets()`.
type ccLibrary struct {
	name      label.Label
	ruleClass string
	target    *proto.Target
}

// protoLibrary holds the parts of a `proto_library` needed to work out the
// import path of the headers in the corresponding `cc_proto_library`.
type protoLibrary struct {
	// Package of the `proto_library` itself, which a relative
	// `strip_import_prefix` is taken to be relative to.
	pkg               string
	srcs              []label.Label
	stripImportPrefix string
	importPrefix      string
}

// splitTargets splits targets into helper/generator rules, and cc_library
// rules.
func (r Repositories) splitTargets(targets []*proto.Target) (
	aliases map[label.Label]label.Label,
	filegroups map[label.Label][]label.Label,
	protoLibraries map[label.Label]protoLibrary,
	ccLibraries []ccLibrary,
) {
	aliases = map[label.Label]label.Label{}
	filegroups = map[label.Label][]label.Label{}
	protoLibraries = map[label.Label]protoLibrary{}

	for _, target := range targets {
		rule := target.GetRule()
		if rule == nil {
			continue
		}
		name, ok := r.ParseLabel(rule.GetName())
		if !ok {
			continue
		}
		// Keep `switch` in sync with `QueryTargets()`.
		switch rule.GetRuleClass() {
		case "alias":
			if actual, ok := r.labelAttr(target, "actual"); ok {
				aliases[actual] = name
			}
		case "filegroup":
			if srcs := r.labelListAttr(target, "srcs"); len(srcs) > 0 {
				filegroups[name] = srcs
			}
		case "proto_library":
			stripImportPrefix, _ := stringAttr(target, "strip_import_prefix")
			importPrefix, _ := stringAttr(target, "import_prefix")
			protoLibraries[name] = protoLibrary{
				pkg:               name.Pkg,
				srcs:              r.labelListAttr(target, "srcs"),
				stripImportPrefix: stripImportPrefix,
				importPrefix:      importPrefix,
			}
		default:
			if name, ok := r.ParseLabel(rule.GetName()); ok && !isHiddenPackage(name) {
				ccLibraries = append(ccLibraries, ccLibrary{name, rule.GetRuleClass(), target})
			}
		}
	}

	return aliases, filegroups, protoLibraries, ccLibraries
}

// resolveSource expands one entry of hdrs into the header files it stands for,
// relative to the package of the rule listing it, expanding filegroups.
func resolveSource(
	source, rule label.Label,
	filegroups map[label.Label][]label.Label,
) []label.Label {
	if srcs, ok := filegroups[source]; ok {
		resolved := make([]label.Label, 0, len(srcs))
		for _, src := range srcs {
			resolved = append(resolved, src.Rel(rule.Repo, rule.Pkg))
		}
		return resolved
	}
	return []label.Label{source.Rel(rule.Repo, rule.Pkg)}
}

// protoHeaders returns the import paths of the C++ headers generated for the
// `proto_library` rules in deps.
//
// Roughly, this converts `<path>.proto` into `<path>.ph.h`, handling
// `strip_import_prefix` and `include_prefix`.
func protoHeaders(
	name label.Label,
	deps []label.Label,
	protoLibraries map[label.Label]protoLibrary,
) collections.Set[label.Label] {
	headers := collections.SetOf[label.Label]()

	for _, dep := range deps {
		lib, ok := protoLibraries[dep]
		if !ok {
			continue
		}
		// `strip_import_prefix` defaults to "/".
		strip := lib.stripImportPrefix
		if rooted, isRooted := strings.CutPrefix(strip, "/"); isRooted {
			strip = rooted
		} else if strip != "" {
			strip = filepath.ToSlash(filepath.Join(lib.pkg, strip))
		}
		for _, src := range lib.srcs {
			path := strings.TrimSuffix(filepath.ToSlash(filepath.Join(src.Pkg, src.Name)), ".proto")
			if path == "" {
				continue
			}
			if strip != "" {
				relative, err := filepath.Rel(strip, path)
				if err != nil || strings.HasPrefix(relative, "..") {
					continue
				}
				path = filepath.ToSlash(relative)
			}
			if lib.importPrefix != "" {
				path = filepath.ToSlash(filepath.Join(lib.importPrefix, path))
			}
			// `path` is the finished import path, so it is returned whole as
			// the `Name` of a package-relative label rather than as a real file
			// label, to make sure `indexer.IndexableIncludePaths()` outputs the
			// right thing. Note that this requires `Includes: {"."}` above.
			headers.Add(label.Label{Repo: name.Repo, Pkg: name.Pkg, Name: path + ".pb.h"}.Rel(name.Repo, name.Pkg))
		}
	}

	return headers
}
