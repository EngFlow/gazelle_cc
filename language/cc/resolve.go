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

package cc

import (
	"log"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cc/index/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/pathtools"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// resolve.Resolver methods
func (c *ccLanguage) Name() string                                        { return languageName }
func (c *ccLanguage) Embeds(r *rule.Rule, from label.Label) []label.Label { return nil }

func (*ccLanguage) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	var imports []resolve.ImportSpec
	switch r.Kind() {
	case "cc_proto_library":
		if !slices.Contains(r.PrivateAttrKeys(), ccProtoLibraryFilesKey) {
			break
		}

		// For each .proto in the target, index the compiler-generated header (foo.proto -> foo.pb.h).
		// This lets other rules resolve #include "pkg/foo.pb.h" even though the header does not appear in hdrs/outs.
		protos := r.PrivateAttr(ccProtoLibraryFilesKey).([]string)
		imports = make([]resolve.ImportSpec, len(protos))
		for i, protoFile := range protos {
			if baseFileName, isProto := strings.CutSuffix(protoFile, ".proto"); isProto {
				generatedHeaderName := baseFileName + ".pb.h"
				imports[i] = resolve.ImportSpec{Lang: languageName, Imp: path.Join(f.Pkg, generatedHeaderName)}
			}
		}
	default:
		hdrs := r.AttrStrings("hdrs")
		stripIncludePrefix := r.AttrString("strip_include_prefix")
		if stripIncludePrefix != "" {
			stripIncludePrefix = path.Clean(stripIncludePrefix)
		}
		includePrefix := r.AttrString("include_prefix")
		if includePrefix != "" {
			includePrefix = path.Clean(includePrefix)
		}
		includes := r.AttrStrings("includes")
		for i, includeDir := range includes {
			includes[i] = path.Clean(includeDir)
		}

		// Maximum possible slice: each header is indexed once for its fully-qualified path and at most once for every matching declared -I include directory.
		imports = make([]resolve.ImportSpec, 0, len(hdrs)*(1+len(includes)))
		for _, hdr := range hdrs {
			// Index the canonicalPath form exactly as it will appear in source
			// Transform the path based on the rule attributes
			canonicalPath := transformIncludePath(f.Pkg, stripIncludePrefix, includePrefix, path.Join(f.Pkg, hdr))
			imports = append(imports, resolve.ImportSpec{Lang: languageName, Imp: canonicalPath})

			// Index shorter includes paths made valid by each -I <includeDir>
			// Bazel adds every entry in the `includes` attribute to the compiler’s search path.
			// With `includes=[include, include/ext]` header `include/ext/foo.h` can be referenced in 3 different ways:
			// - include/ext/foo.h - the fully qualified (canonical) form
			// - ext/foo.h - relative to the `include/` directory (1st 'includes' entry)
			// - foo.h - relative to the `include/ext/` directory (2nd 'includes' entry)
			// We index the an alterantive variants here if they are matching the includes directory.
			for _, includeDir := range includes {
				relativeTo := path.Join(f.Pkg, includeDir)
				if includeDir == "." {
					// Include '.' is special: it makes the path resolvable based from directory defining BUILD file instead of repository root
					relativeTo = f.Pkg
				}
				// Ensure the prefix ends with path separator to distinguish include=foo hdrs=[foo.h, foo/bar.h]
				// It was already cleaned so there won't be duplicate path seperators here
				relativeTo = relativeTo + string(filepath.Separator)
				relativePath, matching := strings.CutPrefix(canonicalPath, relativeTo)
				if !matching {
					// If the include directory is not relative to canonical form it's would be simply ignored.
					continue
				}
				imports = append(imports, resolve.ImportSpec{Lang: languageName, Imp: relativePath})
			}
		}
	}

	return imports
}

// transformIncludePath converts a path to a header file into a string by which the
// header file may be included, accounting for the library's
// strip_include_prefix and include_prefix attributes.
//
// libRel is the slash-separated, repo-root-relative path to the directory
// containing the target.
//
// stripIncludePrefix is the value of the target's strip_include_prefix
// attribute. If it's "", this has no effect. If it's a relative path (including
// "."), both libRel and stripIncludePrefix are stripped from rel. If it's an
// absolute path, the leading '/' is removed, and only stripIncludePrefix is
// removed from hdrRel.
//
// includePrefix is the value of the target's include_prefix attribute.
// It's prepended to hdrRel after stripIncludePrefix is applied.
//
// Both includePrefix and stripIncludePrefix must be clean (with path.Clean)
// if they are non-empty.
//
// hdrRel is the slash-separated, repo-root-relative path to the header file.
func transformIncludePath(libRel, stripIncludePrefix, includePrefix, hdrRel string) string {
	// Strip the prefix.
	var effectiveStripIncludePrefix string
	if path.IsAbs(stripIncludePrefix) {
		effectiveStripIncludePrefix = stripIncludePrefix[len("/"):]
	} else if stripIncludePrefix != "" {
		effectiveStripIncludePrefix = path.Join(libRel, stripIncludePrefix)
	}
	cleanRel := pathtools.TrimPrefix(hdrRel, effectiveStripIncludePrefix)

	// Apply the new prefix.
	cleanRel = path.Join(includePrefix, cleanRel)

	return cleanRel
}

func (lang *ccLanguage) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, imports any, from label.Label) {
	if imports == nil {
		return
	}
	ccImports := imports.(ccImports)
	ccConfig := getCcConfig(c)

	publicDeps := newPlatformDepsBuilder()
	privateDeps := newPlatformDepsBuilder()

	// Resolves given includes to rule labels and assigns them to given attribute.
	// Excludes explicitly provided labels from being assigned
	resolveIncludes := func(includes []ccInclude, builder platformDepsBuilder, excluded collections.Set[label.Label]) {

		for _, include := range includes {
			var resolvedLabel = label.NoLabel
			// 1. Try resolve using fully qualified path (repository-root relative)
			if !include.isSystemInclude {
				relPath := filepath.Join(include.fromDirectory, include.path)
				resolvedLabel = lang.resolveImportSpec(c, ix, from, resolve.ImportSpec{Lang: languageName, Imp: relPath})
			}
			// 2. Try resolve using exact path - using the exact include directive
			if resolvedLabel == label.NoLabel {
				// Retry to resolve is external dependency was defined using quotes instead of braces
				resolvedLabel = lang.resolveImportSpec(c, ix, from, resolve.ImportSpec{Lang: languageName, Imp: include.path})
			}
			if resolvedLabel == label.NoLabel {
				// We typically can get here is given file does not exists or if is assigned to the resolved rule
				continue // failed to resolve
			}
			resolvedLabel = resolvedLabel.Rel(from.Repo, from.Pkg)
			if _, isExcluded := excluded[resolvedLabel]; !isExcluded {
				switch {
				case include.platforms == nil:
					builder.addGeneric(resolvedLabel)
				case len(include.platforms) == 0:
					builder.addConstrained(label.New("", "conditions", "default"), resolvedLabel)
				default:
					for _, platform := range include.platforms {
						if platformConfig, exists := ccConfig.platforms[platform]; exists {
							builder.addConstrained(platformConfig.constraint, resolvedLabel)
						}
					}
				}
			}
		}
	}

	switch resolveCCRuleKind(r.Kind(), c) {
	case "cc_library":
		// Only cc_library has 'implementation_deps' attribute
		// If depenedncy is added by header (via 'deps') ensure it would not be duplicated inside 'implementation_deps'
		resolveIncludes(ccImports.hdrIncludes, publicDeps, collections.Set[label.Label]{})
		resolveIncludes(ccImports.srcIncludes, privateDeps, publicDeps.all)
	default:
		includes := slices.Concat(ccImports.hdrIncludes, ccImports.srcIncludes)
		resolveIncludes(includes, publicDeps, collections.Set[label.Label]{})
	}

	if len(publicDeps.all) > 0 {
		r.SetAttr("deps", publicDeps.build())
	}
	if len(privateDeps.all) > 0 {
		r.SetAttr("implementation_deps", privateDeps.build())
	}
}

func (lang *ccLanguage) resolveImportSpec(c *config.Config, ix *resolve.RuleIndex, from label.Label, importSpec resolve.ImportSpec) label.Label {
	conf := getCcConfig(c)
	// Resolve the gazele:resolve overrides if defined
	if resolvedLabel, ok := resolve.FindRuleWithOverride(c, importSpec, languageName); ok {
		return resolvedLabel
	}

	// Resolve using imports registered in Imports
	for _, searchResult := range ix.FindRulesByImportWithConfig(c, importSpec, languageName) {
		if !searchResult.IsSelfImport(from) {
			return searchResult.Label
		}
	}

	for _, index := range conf.dependencyIndexes {
		if label, exists := index[importSpec.Imp]; exists {
			return label
		}
	}

	if label, exists := lang.bzlmodBuiltInIndex[importSpec.Imp]; exists {
		apparantName := c.ModuleToApparentName(label.Repo)
		// Empty apparentName means that there is no such a repository added by bazel_dep
		if apparantName != "" {
			label.Repo = apparantName
			return label
		}
		if _, exists := lang.notFoundBzlModDeps[label.Repo]; !exists {
			// Warn only once per missing module_dep
			lang.notFoundBzlModDeps[label.Repo] = true
			log.Printf("%v: Resolved mapping of '#include %v' to %v, but 'bazel_dep(name = \"%v\")' is missing in MODULE.bazel", from, importSpec.Imp, label, label.Repo)
		}
	}

	return label.NoLabel
}

type platformDepsBuilder struct {
	// Tracks all found dependencies
	all collections.Set[label.Label]
	// Dependencies that are shared by all platforms
	generic collections.Set[label.Label]
	// Map of platform specific constraints and dependencies assigned to each of them
	constrainted map[label.Label]collections.Set[label.Label]
}

func newPlatformDepsBuilder() platformDepsBuilder {
	return platformDepsBuilder{
		all:          make(collections.Set[label.Label]),
		generic:      make(collections.Set[label.Label]),
		constrainted: make(map[label.Label]collections.Set[label.Label]),
	}
}
func (b *platformDepsBuilder) addGeneric(dependency label.Label) {
	b.all.Add(dependency)
	b.generic.Add(dependency)
}
func (b *platformDepsBuilder) addConstrained(condition label.Label, dependency label.Label) {
	b.all.Add(dependency)
	deps, exists := b.constrainted[condition]
	if !exists {
		deps = make(collections.Set[label.Label])
		b.constrainted[condition] = deps
	}
	deps.Add(dependency)
}
func (b *platformDepsBuilder) build() CcPlatformStrings {
	platformStrings := CcPlatformStrings{[]string{}, map[string][]string{}}
	toStringsSlice := func(labels collections.Set[label.Label]) []string {
		return collections.Map(labels.Values(), func(label label.Label) string { return label.String() })
	}
	platformStrings.Generic = toStringsSlice(b.generic)
	for constraintLabel, deps := range b.constrainted {
		platformStrings.Constrained[constraintLabel.String()] = toStringsSlice(deps)
	}
	return platformStrings
}
