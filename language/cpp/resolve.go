// Copyright 2025 EngFlow, Inc. All rights reserved.
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

package cpp

import (
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
func (c *cppLanguage) Name() string                                        { return languageName }
func (c *cppLanguage) Embeds(r *rule.Rule, from label.Label) []label.Label { return nil }

func (*cppLanguage) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	rel := f.Pkg
	prefix := rel
	hdrs := r.AttrStrings("hdrs")
	imports := make([]resolve.ImportSpec, len(hdrs))
	for i, hdr := range hdrs {
		imports[i] = resolve.ImportSpec{Lang: languageName, Imp: path.Join(prefix, hdr)}
	}
	return imports
}

func (*cppLanguage) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, imports interface{}, from label.Label) {
	if imports == nil {
		return
	}

	cppImports := imports.(cppImports)
	deps := make(map[label.Label]bool)

	for _, include := range cppImports.includes {
		resolvedLabel := resolveImportSpec(c, ix, from, resolve.ImportSpec{Lang: languageName, Imp: include.normalizedPath})
		if resolvedLabel != label.NoLabel {
			deps[resolvedLabel] = true
		}

		// Retry to resolve is external dependency was defined using quotes instead of braces
		if !include.isSystemInclude {
			resolvedLabel = resolveImportSpec(c, ix, from, resolve.ImportSpec{Lang: languageName, Imp: include.rawPath})
			if resolvedLabel != label.NoLabel {
				deps[resolvedLabel] = true
			}
		}
	}

	if len(deps) > 0 {
		r.SetAttr("deps", slices.SortedStableFunc(maps.Keys(deps), func(l, r label.Label) int {
			return strings.Compare(l.String(), r.String())
		}))
	}
}

func resolveImportSpec(c *config.Config, ix *resolve.RuleIndex, from label.Label, importSpec resolve.ImportSpec) label.Label {
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
	return label.NoLabel
}
