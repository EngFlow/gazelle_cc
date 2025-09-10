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
	"errors"
	"log"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/pathtools"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/bazel-gazelle/walk"
	"github.com/bmatcuk/doublestar/v4"
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
		hdrs, err := collectStringsAttr(*c, r, f.Pkg, "hdrs")
		if err != nil {
			log.Printf("gazelle_cc: failed to collect 'hdrs' attribute of %v defined in %v:%v, these would not be indexed: %v", r.Kind(), f.Pkg, r.Name(), err)
			break
		}
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

	// Resolves given includes to rule labels and assigns them to given attribute.
	// Excludes explicitly provided labels from being assigned
	// Returns a set of successfully assigned labels, allowing to exclude them in following invocations
	resolveIncludes := func(includes []ccInclude, attributeName string, excluded collections.Set[label.Label], deps collections.Set[label.Label]) collections.Set[label.Label] {
		for _, include := range includes {
			var resolvedLabel = label.NoLabel
			// 1. Try resolve using fully qualified path (repository-root relative)
			if !include.isSystemInclude {
				relPath := filepath.Join(include.fromDirectory, include.path)
				resolvedLabel = lang.resolveImportSpec(c, ix, from, resolve.ImportSpec{Lang: languageName, Imp: relPath}, include)
			}
			// 2. Try resolve using exact path - using the exact include directive
			if resolvedLabel == label.NoLabel {
				// Retry to resolve is external dependency was defined using quotes instead of braces
				resolvedLabel = lang.resolveImportSpec(c, ix, from, resolve.ImportSpec{Lang: languageName, Imp: include.path}, include)
			}
			if resolvedLabel == label.NoLabel {
				// We typically can get here is given file does not exists or if is assigned to the resolved rule
				continue // failed to resolve
			}
			resolvedLabel = resolvedLabel.Rel(from.Repo, from.Pkg)
			if _, isExcluded := excluded[resolvedLabel]; !isExcluded {
				deps.Add(resolvedLabel)
			}
		}
		if len(deps) > 0 {
			r.SetAttr(attributeName, deps.SortedValues(compareLabels))
		}
		return deps
	}

	switch resolveCCRuleKind(r.Kind(), c) {
	case "cc_library":
		// Only cc_library has 'implementation_deps' attribute
		// If depenedncy is added by header (via 'deps') ensure it would not be duplicated inside 'implementation_deps'
		publicDeps := resolveIncludes(ccImports.hdrIncludes, "deps", make(collections.Set[label.Label]), make(collections.Set[label.Label]))
		resolveIncludes(ccImports.srcIncludes, "implementation_deps", publicDeps, make(collections.Set[label.Label]))
	default:
		includes := slices.Concat(ccImports.hdrIncludes, ccImports.srcIncludes)
		// cc_test might have implicit dependency on test runner - cc_library defining main method required when linking
		deps := make(collections.Set[label.Label])
		if testRunnerDep, ok := r.PrivateAttr(ccTestRunnerDepKey).(label.Label); ok {
			deps.Add(testRunnerDep)
		}
		resolveIncludes(includes, "deps", make(collections.Set[label.Label]), deps)
	}
}

func extractLabelsFromFindResults(results []resolve.FindResult) collections.Set[label.Label] {
	labels := make(collections.Set[label.Label])
	for _, r := range results {
		labels.Add(r.Label)
	}
	return labels
}

func compareLabels(l, r label.Label) int {
	return strings.Compare(l.String(), r.String())
}

func (lang *ccLanguage) resolveImportSpec(c *config.Config, ix *resolve.RuleIndex, from label.Label, importSpec resolve.ImportSpec, include ccInclude) label.Label {
	conf := getCcConfig(c)
	// Resolve the gazele:resolve overrides if defined
	if resolvedLabel, ok := resolve.FindRuleWithOverride(c, importSpec, languageName); ok {
		return resolvedLabel
	}

	// Resolve using imports registered in Imports
	importedRules := ix.FindRulesByImportWithConfig(c, importSpec, languageName)
	for _, searchResult := range importedRules {
		if !searchResult.IsSelfImport(from) {
			if len(importedRules) > 1 {
				ambiguousLabels := extractLabelsFromFindResults(importedRules).SortedValues(compareLabels)
				log.Printf("%v: found ambiguous rules providing '#include %q' at %s:%d: %v; using %v", from, include.path, include.sourceFile, include.lineNumber, ambiguousLabels, searchResult.Label)
			}
			return searchResult.Label
		}
	}

	for _, index := range conf.dependencyIndexes {
		if label, exists := index[importSpec.Imp]; exists {
			return label
		}
	}

	if conf.useBuiltinBzlmodIndex {
		if label, exists := lang.bzlmodBuiltInIndex[importSpec.Imp]; exists && label.Repo != c.RepoName {
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
	}

	return label.NoLabel
}

// collectStringsAttr collects the values of the given attribute from the rule.
// If the attribute is a list of strings, it returns the list.
// If the attribute is a glob, it expands the glob patterns relative to dir and returns
// the resulting paths.
func collectStringsAttr(c config.Config, r *rule.Rule, dir, attrName string) ([]string, error) {
	// Fast path: plain list of strings in the BUILD file.
	if ss := r.AttrStrings(attrName); ss != nil {
		return ss, nil
	}

	expr := r.Attr(attrName) // nil if the attribute is not present
	if expr == nil {
		return nil, nil
	}
	if globValue, ok := rule.ParseGlobExpr(expr); ok {
		return expandGlob(c, dir, globValue)
	}
	return nil, nil
}

// expandGlob expands the glob patterns in the given glob value relative to relPath.
// It returns a sorted list of paths that match the patterns, excluding those that match the excludes.
// The paths are relative to relPath, and they are sorted in lexicographical order.
// It does not use I/O, it uses cached directory info obtained from walk.GetDirInfo
// so it might panic if the directory was not walked before.
func expandGlob(config config.Config, dir string, glob rule.GlobValue) ([]string, error) {
	if len(glob.Patterns) == 0 {
		return nil, nil
	}
	// Filter out invalid patterns
	validatedPatterns := func(patterns []string) []string {
		validated := make([]string, 0, len(patterns))
		for _, pattern := range patterns {
			if doublestar.ValidatePattern(pattern) {
				validated = append(validated, pattern)
			}
		}
		return validated
	}

	includePatterns := validatedPatterns(glob.Patterns)
	if len(includePatterns) == 0 {
		return nil, errors.New("no valid include patterns found")
	}
	excludePatterns := validatedPatterns(glob.Excludes)

	// Traverse the file tree using walk.GetDirInfo and collect all matching files
	matched := []string{}
	var traverse func(string)
	traverse = func(relativePath string) {
		di, err := walk.GetDirInfo(relativePath)
		if err != nil {
			return // swallow errors
		}
		if relativePath != "" {
			// When walking the subdirectories, we need to exclude dirs containg BUILD files
			if slices.ContainsFunc(di.RegularFiles, config.IsValidBuildFileName) {
				return // BUILD file found, stop walking
			}
		}
		for _, file := range di.RegularFiles {
			path := filepath.Join(relativePath, file)
			// Check matches include pattern
			if !slices.ContainsFunc(
				includePatterns,
				func(pattern string) bool { return doublestar.MatchUnvalidated(pattern, path) },
			) {
				continue // not included
			}
			// Check matched exclude pattern
			if slices.ContainsFunc(
				excludePatterns,
				func(pattern string) bool { return doublestar.MatchUnvalidated(pattern, path) },
			) {
				continue // excluded
			}
			matched = append(matched, path)
		}
		for _, dir := range di.Subdirs {
			traverse(filepath.Join(relativePath, dir))
		}
	}
	traverse(dir)
	sort.Strings(matched)
	return matched, nil
}
