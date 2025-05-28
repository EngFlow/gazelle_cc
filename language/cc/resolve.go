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
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
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
		imports = make([]resolve.ImportSpec, len(hdrs))
		for i, hdr := range hdrs {
			imports[i] = resolve.ImportSpec{Lang: languageName, Imp: path.Join(f.Pkg, hdr)}
		}
	}

	return imports
}

func (lang *ccLanguage) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, imports any, from label.Label) {
	if imports == nil {
		return
	}
	cppImports := imports.(cppImports)

	type labelsSet map[label.Label]struct{}
	// Resolves given includes to rule labels and assigns them to given attribute.
	// Excludes explicitly provided labels from being assigned
	// Returns a set of successfully assigned labels, allowing to exclude them in following invocations
	resolveIncludes := func(includes []cppInclude, attributeName string, excluded labelsSet) labelsSet {
		deps := make(map[label.Label]struct{})
		for _, include := range includes {
			resolvedLabel := lang.resolveImportSpec(c, ix, from, resolve.ImportSpec{Lang: languageName, Imp: include.normalizedPath})
			if resolvedLabel == label.NoLabel && !include.isSystemInclude {
				// Retry to resolve is external dependency was defined using quotes instead of braces
				resolvedLabel = lang.resolveImportSpec(c, ix, from, resolve.ImportSpec{Lang: languageName, Imp: include.rawPath})
			}
			if resolvedLabel == label.NoLabel {
				// We typically can get here is given file does not exists or if is assigned to the resolved rule
				continue // failed to resolve
			}
			resolvedLabel = resolvedLabel.Rel(from.Repo, from.Pkg)
			if _, isExcluded := excluded[resolvedLabel]; !isExcluded {
				deps[resolvedLabel] = struct{}{}
			}
		}
		if len(deps) > 0 {
			r.SetAttr(attributeName, slices.SortedStableFunc(maps.Keys(deps), func(l, r label.Label) int {
				return strings.Compare(l.String(), r.String())
			}))
		}
		return deps
	}

	switch resolveCCRuleKind(r.Kind(), c) {
	case "cc_library":
		// Only cc_library has 'implementation_deps' attribute
		// If depenedncy is added by header (via 'deps') ensure it would not be duplicated inside 'implementation_deps'
		publicDeps := resolveIncludes(cppImports.hdrIncludes, "deps", make(labelsSet))
		resolveIncludes(cppImports.srcIncludes, "implementation_deps", publicDeps)
	default:
		includes := slices.Concat(cppImports.hdrIncludes, cppImports.srcIncludes)
		resolveIncludes(includes, "deps", make(labelsSet))
	}
}

func (lang *ccLanguage) resolveImportSpec(c *config.Config, ix *resolve.RuleIndex, from label.Label, importSpec resolve.ImportSpec) label.Label {
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

	if label, exists := lang.bzlmodDependenciesIndex[importSpec.Imp]; exists {
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
