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

package cc

import (
	"errors"
	"fmt"
	"log"
	"path"
	"path/filepath"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// resolve.Resolver methods
func (c *ccLanguage) Name() string                                        { return languageName }
func (c *ccLanguage) Embeds(r *rule.Rule, from label.Label) []label.Label { return nil }
func (lang *ccLanguage) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, imports any, from label.Label) {
	ccImports, hasImports := imports.(ccImports)
	if !hasImports {
		return
	}

	publicDeps, privateDeps := lang.resolveDeps(c, ix, r, ccImports, from)
	if len(publicDeps.all) > 0 {
		r.SetAttr("deps", publicDeps.build())
	}
	if len(privateDeps.all) > 0 {
		r.SetAttr("implementation_deps", privateDeps.build())
	}
}

func (lang *ccLanguage) resolveDeps(c *config.Config, ix *resolve.RuleIndex, r *rule.Rule, imports ccImports, from label.Label) (publicDeps, privateDeps platformDepsBuilder) {
	switch resolveCCRuleKind(r.Kind(), c) {
	case "cc_library":
		// Only cc_library has 'implementation_deps' attribute
		// If depenedncy is added by header (via 'deps') ensure it would not be duplicated inside 'implementation_deps'
		publicDeps = lang.resolveIncludes(c, ix, r, from, imports.hdrIncludes, collections.Set[label.Label]{})
		privateDeps = lang.resolveIncludes(c, ix, r, from, imports.srcIncludes, publicDeps.all)
	default:
		publicDeps = lang.resolveIncludes(c, ix, r, from, imports.allIncludes(), collections.Set[label.Label]{})

		// cc_test might have implicit dependency on test runner - cc_library defining main method required when linking
		if testRunnerDep, ok := r.PrivateAttr(ccTestRunnerDepKey).(label.Label); ok {
			publicDeps.addGeneric(testRunnerDep)
		}
	}
	return
}

// Resolves given includes to rule labels and assigns them to the given builder.
// Excludes explicitly provided labels from being assigned.
func (lang *ccLanguage) resolveIncludes(
	c *config.Config,
	ix *resolve.RuleIndex,
	r *rule.Rule,
	from label.Label,
	includes []ccInclude,
	excluded collections.Set[label.Label],
) platformDepsBuilder {
	ccConfig := getCcConfig(c)
	result := newPlatformDepsBuilder()

	for _, include := range includes {
		if path.IsAbs(include.path) || filepath.IsAbs(include.path) {
			// Don't try to resolve absolute paths, even within the repo.
			continue
		}

		resolvedLabel, err := lang.resolveSingleInclude(c, ix, r, from, include)
		if !lang.handleIncludeResolutionError(c, include, resolvedLabel, err) {
			continue
		}

		// Successfully resolved
		resolvedLabel = resolvedLabel.Rel(from.Repo, from.Pkg)
		if !excluded.Contains(resolvedLabel) {
			result.addResolved(resolvedLabel, ccConfig, include)
		}
	}

	return result
}

// Attempts to resolve a single include directive to a rule label. It tries
// multiple resolution strategies in order:
//  1. Fully qualified path (repository-root relative) for non-system includes
//  2. Exact path using the include directive as-is
func (lang *ccLanguage) resolveSingleInclude(
	c *config.Config,
	ix *resolve.RuleIndex,
	r *rule.Rule,
	from label.Label,
	include ccInclude,
) (label.Label, error) {
	resolvedLabel := label.NoLabel
	err := errUnresolved

	// 1. Try resolve using fully qualified path (repository-root relative)
	if !include.isSystemInclude {
		relPath := filepath.Join(include.sourceDirectory(), include.path)
		resolvedLabel, err = lang.resolveImportSpec(c, ix, r, from, resolve.ImportSpec{Lang: languageName, Imp: relPath}, include)
	}

	// 2. Try resolve using exact path - using the exact include directive
	if errors.Is(err, errUnresolved) {
		// Retry to resolve if external dependency was defined using quotes instead of braces
		resolvedLabel, err = lang.resolveImportSpec(c, ix, r, from, resolve.ImportSpec{Lang: languageName, Imp: include.path}, include)
	}

	return resolvedLabel, err
}

// Handles errors from include resolution and logs appropriate warnings. Returns
// true if the resolution should continue (dependency can be added), false if it
// should be skipped.
func (lang *ccLanguage) handleIncludeResolutionError(
	c *config.Config,
	include ccInclude,
	resolvedLabel label.Label,
	err error,
) bool {
	switch {
	case errors.Is(err, errAmbiguousImport):
		// Warn about ambiguous imports, but still add one of the candidates (if
		// appropriate cc_ambiguous_deps is set)
		log.Print(err)
	case errors.Is(err, errMissingModuleDependency):
		if !lang.notFoundBzlModDeps.Contains(resolvedLabel.Repo) {
			// Warn only once per missing module_dep
			lang.notFoundBzlModDeps.Add(resolvedLabel.Repo)
			log.Print(err)
		}
		return false
	case errors.Is(err, errSelfImport):
		// Ignore: the rule exists, but it should not be added as a dependency
		return false
	case errors.Is(err, errUnresolved):
		// Warn about unresolved non-system include directives
		if !include.isSystemInclude {
			lang.handleReportedError(getCcConfig(c).unresolvedDepsMode, err)
		}
		return false
	}

	return resolvedLabel != label.NoLabel
}

var (
	errAmbiguousImport         = errors.New("multiple libraries provide the same header")
	errMissingModuleDependency = errors.New("header file found in external library not declared in MODULE.bazel")
	errSelfImport              = errors.New("library includes itself")
	errUnresolved              = errors.New("could not find a library providing header")
)

func containsMultipleRepos(labels []label.Label) bool {
	if len(labels) > 1 {
		firstRepo := labels[0].Repo
		for _, l := range labels[1:] {
			if l.Repo != firstRepo {
				return true
			}
		}
	}
	return false
}

func resolveAmbiguousDependency(resolvedDeps []label.Label, mode ambiguousDepsMode, r *rule.Rule, from label.Label, include ccInclude) (label.Label, error) {
	// Respect the existing dependency before triggering an ambiguity warning
	if existingDeps, ok := r.PrivateAttr(ccExistingDepsKey).(collections.Set[label.Label]); ok {
		if commonDeps := existingDeps.Intersect(collections.ToSet(resolvedDeps)); len(commonDeps) == 1 {
			// Return the only common dependency
			for dep := range commonDeps {
				return dep, nil
			}
		}
	}

	switch len(resolvedDeps) {
	case 0:
		return label.NoLabel, fmt.Errorf("%v: %w - %v", from, errUnresolved, include)
	case 1:
		return resolvedDeps[0], nil
	default:
		switch mode {
		case ambiguousDepsMode_try_first, ambiguousDepsMode_force_first:
			if mode == ambiguousDepsMode_force_first || !containsMultipleRepos(resolvedDeps) {
				selected := resolvedDeps[0]
				return selected, fmt.Errorf("%v: %w - %v resolved to %v; using %v", from, errAmbiguousImport, include, resolvedDeps, selected)
			}
			fallthrough
		case ambiguousDepsMode_warn:
			return label.NoLabel, fmt.Errorf("%v: %w - %v resolved to %v; don't know which one to use", from, errAmbiguousImport, include, resolvedDeps)
		default:
			// Silently ignore the ambiguous dependency.
			return label.NoLabel, nil
		}
	}
}

// Tries to resolve given importSpec, looking for an external rule other than the source "from" label, using the following strategies:
//  1. Using gazelle:resolve override if defined.
//  2. Using imports registered in Imports.
//  3. Using dependency indexes defined by gazelle:cc_indexfile.
//  4. Using built-in bzlmod index if enabled by gazelle:cc_use_builtin_bzlmod_index.
//
// Returns the resolved label, optionally with a wrapped one of 'err*' errors.
// For errUnresolved the returned label is label.NoLabel.
func (lang *ccLanguage) resolveImportSpec(c *config.Config, ix *resolve.RuleIndex, r *rule.Rule, from label.Label, importSpec resolve.ImportSpec, include ccInclude) (label.Label, error) {
	conf := getCcConfig(c)
	// Resolve the gazele:resolve overrides if defined
	if resolvedLabel, ok := resolve.FindRuleWithOverride(c, importSpec, languageName); ok {
		return resolvedLabel, nil
	}

	// Resolve using imports registered in Imports
	if importedRules := ix.FindRulesByImportWithConfig(c, importSpec, languageName); len(importedRules) > 0 {
		// Any self-import should immediately stop the resolution
		for _, searchResult := range importedRules {
			if searchResult.IsSelfImport(from) {
				return from, fmt.Errorf("%v: %w - %v", from, errSelfImport, include)
			}
		}

		resolvedDeps := collections.MapSlice(importedRules, func(r resolve.FindResult) label.Label { return r.Label })
		return resolveAmbiguousDependency(resolvedDeps, conf.ambiguousDepsMode, r, from, include)
	}

	for _, index := range conf.dependencyIndexes {
		if result, exists := index[importSpec.Imp]; exists {
			return result, nil
		}
	}

	if conf.useBuiltinBzlmodIndex {
		if result, exists := lang.bzlmodBuiltInIndex[importSpec.Imp]; exists && result.Repo != c.RepoName {
			// Empty apparentName means that there is no such a repository added by bazel_dep
			if apparentName := c.ModuleToApparentName(result.Repo); apparentName != "" {
				result.Repo = apparentName
				return result, nil
			} else {
				return result, fmt.Errorf("%v: %w - %v resolved to %v, but 'bazel_dep(name = \"%v\")' is missing", from, errMissingModuleDependency, include, result, result.Repo)
			}
		}
	}

	return label.NoLabel, fmt.Errorf("%v: %w - %v", from, errUnresolved, include)
}

type platformDepsBuilder struct {
	// Tracks all found dependencies
	all collections.Set[label.Label]
	// Dependencies that are shared by all platforms
	generic collections.Set[label.Label]
	// Map of platform specific constraints and dependencies assigned to each of them
	constrained map[label.Label]collections.Set[label.Label]
}

func newPlatformDepsBuilder() platformDepsBuilder {
	return platformDepsBuilder{
		all:         make(collections.Set[label.Label]),
		generic:     make(collections.Set[label.Label]),
		constrained: make(map[label.Label]collections.Set[label.Label]),
	}
}

func (b *platformDepsBuilder) addGeneric(dependency label.Label) {
	b.all.Add(dependency)
	b.generic.Add(dependency)
}

func (b *platformDepsBuilder) addConstrained(condition label.Label, dependency label.Label) {
	b.all.Add(dependency)
	deps, exists := b.constrained[condition]
	if !exists {
		deps = make(collections.Set[label.Label])
		b.constrained[condition] = deps
	}
	deps.Add(dependency)
}

// Pseudo-label for Bazel select() function, considered to match if no other
// condition matches.
var defaultCondition = label.New("", "conditions", "default")

// Adds a resolved dependency to the builder, accounting for platform
// specificity.
func (b *platformDepsBuilder) addResolved(dependency label.Label, config *ccConfig, include ccInclude) {
	switch {
	case !include.isPlatformSpecific:
		b.addGeneric(dependency)
	case len(include.platforms) == 0:
		b.addConstrained(defaultCondition, dependency)
	default:
		for _, platform := range include.platforms {
			if platformConfig, exists := config.platforms[platform]; exists {
				b.addConstrained(platformConfig.constraint, dependency)
			}
		}
	}
}

func (b *platformDepsBuilder) build() ccPlatformStringsExprs {
	return newCcPlatformStringsExprs(b.generic, b.constrained)
}
