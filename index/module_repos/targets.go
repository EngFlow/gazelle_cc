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
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cc/index/internal/bazel"
	"github.com/EngFlow/gazelle_cc/index/internal/bazel/proto"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/label"
)

// queryTargets runs a `bazel query` yielding all indexable targets in `repos`.
func queryTargets(workingDir string, repos repositories) (*proto.QueryResult, error) {
	if len(repos.apparent) == 0 {
		return nil, errors.New("no repositories to index")
	}

	// Only public targets are indexable, except for `proto_library` rules,
	// which are needed to find the sources of a public `cc_proto_library`.
	query := fmt.Sprintf(
		// Keep `kind()`s in sync with `splitTargets()`. We need:
		//
		// - `cc_library()` (and user-defined variants with compatible attribute
		//   names), to find exposed C++ libraries (and notably their `hdrs`,
		//   `includes`, `include_prefix`, and `strip_include_prefix`).
		//
		// - `cc_proto_library()` (matched by the `cc_.*library` regex), to
		//   synthesize `.pb.h` headers for their `deps`'s source files (see
		//   below).
		//
		// - `proto_library()`, to find the `.proto` files in each
		//   `proto_library` that `cc_proto_library` depends on. As an example,
		//   given `cc_proto_library(name = "foo", deps = [":bar", ":baz"])`,
		//   we need to scan the `proto_library` targets `:bar` and `:baz` to
		//   find their `.proto` files so that we may synthesize `.pb.h` headers
		//   for `:foo`.
		//
		// - `alias()`, in case a `cc_library` is re-exported with a preferred
		//   alias. We don't actually know whether an alias is preferred, so
		//   we use some heuristics to pick an alias over its target's label;
		//   see `exposedLabel()`.
		//
		// - `filegroup()`, to expand `cc_library` `hdrs` that refer to
		//   `filegroup`s rather than source files.
		//
		// `filegroup`s and `proto_library` rules do not need to be public.
		// `cc_library` rules do, we query private ones too as they can be
		// re-exposed by an `alias`.
		`let universe = @%s//... in `+
			`(kind("^alias rule$", $universe) intersect attr(visibility, "//visibility:public", $universe)) `+
			`union kind("^(cc_.*library|filegroup|proto_library) rule$", $universe)`,
		strings.Join(repos.apparent, "//... + @"),
	)

	// Keep going, so if a repository fails to load we may still compute the
	// index.
	result, err := bazel.ConfiguredQuery(workingDir, query, bazel.QueryConfig{KeepGoing: true})
	if err != nil {
		return nil, fmt.Errorf("failed to query workspace: %w", err)
	}
	return &result, nil
}

// groupByRepository groups the queried targets by the repository defining them.
func (r repositories) groupByRepository(result *proto.QueryResult) map[string][]*proto.Target {
	grouped := make(map[string][]*proto.Target, len(r.apparent))
	for _, target := range result.GetTarget() {
		rule := target.GetRule()
		if rule == nil {
			continue
		}
		name, ok := r.parseLabel(rule.GetName())
		if !ok || name.Repo == "" {
			continue
		}
		grouped[name.Repo] = append(grouped[name.Repo], target)
	}
	return grouped
}

// buildModule turns the targets of a single repository into an indexer.Module.
func (r repositories) buildModule(repository string, targets []*proto.Target) indexer.Module {
	aliases, filegroups, protoLibraries, ccLibraries := r.splitTargets(targets)

	indexed := make([]indexer.Target, 0, len(ccLibraries))
	for _, ccLib := range ccLibraries {
		target := ccLib.target
		name := ccLib.name

		deps := collections.SetOf[label.Label]()
		for _, dep := range r.labelListAttr(target, "deps") {
			deps.Add(dep.Rel(name.Repo, name.Pkg))
		}

		exposedAs, indexable := exposedLabel(name, isPublic(target), aliases)
		if !indexable {
			continue
		}

		// A `cc_proto_library` has no `hdrs`, so we must handle it manually
		// using its `cc_library`.
		if ccLib.ruleClass == "cc_proto_library" {
			// `indexer.IndexableIncludePaths()` joins headers after their
			// target's package, so we add "." to `Includes`.
			indexed = append(indexed, indexer.Target{
				Name:        name,
				ReexposedAs: exposedAs,
				Hdrs:        protoHeaders(name, r.labelListAttr(target, "deps"), protoLibraries),
				Includes:    collections.SetOf("."),
				Deps:        deps,
			})
			continue
		}

		hdrs := collections.SetOf[label.Label]()
		for _, hdr := range r.labelListAttr(target, "hdrs") {
			for _, resolved := range resolveSource(hdr, name, filegroups) {
				hdrs.Add(resolved)
			}
		}

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
			ReexposedAs:        exposedAs,
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

// exposedLabel returns the label a target should be indexed under, and whether
// it should be indexed at all.
//
// aliases may only contain public aliases.
//
// When both name and its alias are usable, the alias is preferred when it looks
// like the intended public label -- either it is the repository's main target
// (e.g. `@fmt//:fmt`) or it is shorter.
//
// An alias can also help transition off a deprecated rule, so something we
// shouldn't use, but alas, there is no good way to know.
//
// label.NoLabel will be returned if the target should be exposed as name.
func exposedLabel(
	name label.Label,
	public bool,
	aliases map[label.Label]label.Label,
) (exposedAs label.Label, indexable bool) {
	alias, aliased := aliases[name]
	if !aliased {
		return label.NoLabel, public
	}
	if !public {
		return alias, true
	}
	// Prefer repository-named aliases, then shorter ones. Unlike
	// `preferFirstAlias()` we do not prefer package-named aliases as they
	// could match deeply nested aliases somewhere unexpected.
	if alias.Name == alias.Repo || (len(alias.String()) < len(name.String()) && name.Name != name.Repo) {
		return alias, true
	}
	return label.NoLabel, true
}

// preferFirstAlias returns whether first is more likely to be intended as the
// public alias for target than second is.
func preferFirstAlias(first, second label.Label) bool {
	// Prefer whichever matches its package's or repository's name.
	firstMatchesParent := matchesParent(first)
	secondMatchesParent := matchesParent(second)
	if firstMatchesParent != secondMatchesParent {
		return firstMatchesParent
	}
	// Prefer a shorter label, like `exposedLabel()`.
	if byLength := len(first.String()) - len(second.String()); byLength != 0 {
		return byLength < 0
	}
	// If both aliases are similar, compare lexicographically to
	// deterministically choose one over the other.
	return first.String() < second.String()
}

// matchesParent returns whether name is named like its parent package or
// repository.
//
// name must have a Repo, i.e. it must not have been relativized.
func matchesParent(name label.Label) bool {
	if name.Pkg != "" {
		return name.Name == path.Base(name.Pkg)
	}
	return name.Name == name.Repo
}

// isPublic reports whether a target can be depended upon from anywhere, i.e.
// whether its visibility includes `//visibility:public`.
func isPublic(target *proto.Target) bool {
	return slices.Contains(stringListAttr(target, "visibility"), "//visibility:public")
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
func (r repositories) splitTargets(targets []*proto.Target) (
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
		name, ok := r.parseLabel(rule.GetName())
		if !ok {
			continue
		}
		// Keep `switch` in sync with `queryTargets()`.
		switch rule.GetRuleClass() {
		case "alias":
			if actual, ok := r.labelAttr(target, "actual"); ok {
				// Several public aliases may re-export one target, so keep the
				// best rather than whichever the query happened to yield last.
				if previous, seen := aliases[actual]; !seen || preferFirstAlias(name, previous) {
					// Note that `targets` are per-repo, so cross-repo aliases,
					// if any, will never be used.
					aliases[actual] = name
				}
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
			if name, ok := r.parseLabel(rule.GetName()); ok && !isHiddenPackage(name) {
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
// Roughly, this converts `<path>.proto` into `<path>.pb.h`, handling
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
