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
	"sort"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/pathtools"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/bazel-gazelle/walk"
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
		hdrs, err := collectStringsAttr(r, f.Pkg, "hdrs")
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

	type labelsSet map[label.Label]struct{}
	// Resolves given includes to rule labels and assigns them to given attribute.
	// Excludes explicitly provided labels from being assigned
	// Returns a set of successfully assigned labels, allowing to exclude them in following invocations
	resolveIncludes := func(includes []ccInclude, attributeName string, excluded labelsSet) labelsSet {
		deps := make(map[label.Label]struct{})
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
		publicDeps := resolveIncludes(ccImports.hdrIncludes, "deps", make(labelsSet))
		resolveIncludes(ccImports.srcIncludes, "implementation_deps", publicDeps)
	default:
		includes := slices.Concat(ccImports.hdrIncludes, ccImports.srcIncludes)
		resolveIncludes(includes, "deps", make(labelsSet))
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

// collectStringsAttr collects the values of the given attribute from the rule.
// If the attribute is a list of strings, it returns the list.
// If the attribute is a glob, it expands the glob patterns relative to dir and returns
// the resulting paths.
func collectStringsAttr(r *rule.Rule, dir, name string) ([]string, error) {
	// Fast path: plain list of strings in the BUILD file.
	if ss := r.AttrStrings(name); ss != nil {
		return ss, nil
	}

	expr := r.Attr(name) // nil if the attribute is not present
	if expr == nil {
		return nil, nil
	}
	if globValue, ok := rule.ParseGlobExpr(expr); ok {
		return expandGlob(dir, globValue)
	}
	return nil, nil
}

// expandGlob expands the glob patterns in the given glob value relative to relPath.
// It returns a sorted list of paths that match the patterns, excluding those that match the excludes.
// The paths are relative to relPath, and they are sorted in lexicographical order.
// It does not use I/O, it uses cached directory info obtained from walk.GetDirInfo
// so it might panic if the directory was not walked before.
func expandGlob(relPath string, glob rule.GlobValue) ([]string, error) {
	if len(glob.Patterns) == 0 {
		return nil, nil
	}

	matched := map[string]struct{}{}
	for _, pattern := range glob.Patterns {
		if err := walkGlob(relPath, pattern, matched); err != nil {
			return nil, err
		}
	}
	excluded := map[string]struct{}{}
	for _, pattern := range glob.Excludes {
		if err := walkGlob(relPath, pattern, excluded); err != nil {
			return nil, err
		}
	}

	result := make([]string, 0, len(matched))
	for path := range matched {
		if _, excluded := excluded[path]; !excluded {
			result = append(result, path)
		}
	}
	log.Printf("gazelle_cc: glob %q expanded to %d files in %q: %v", strings.Join(glob.Patterns, ","), len(result), relPath, result)
	sort.Strings(result)
	return result, nil
}

// Recursively walks the directory tree rooted at dirRel, following glob pattern,
// Records every matching file path in found
// Supports "**" for zero or more segments, and ordinary glob patterns
// Resolving does not use I/O, it uses cached directory info obtained from walk.GetDirInfo - it might panic if the directory was not walked before.
func walkGlob(dirRel string, pattern string, found map[string]struct{}) error {
	patternParts := strings.Split(path.Clean(pattern), "/")
	return walkGlobImpl(dirRel, patternParts, "", found)
}

// walkGlobImpl is the implementation of walkGlob that does the actual walking.
func walkGlobImpl(dirRel string, patternSegments []string, prefix string, found map[string]struct{}) error {
	di, err := walk.GetDirInfo(dirRel) // cached; no I/O
	if err != nil {
		return err
	}

	// Pattern exhausted -> add every regular file in this directory.
	if len(patternSegments) == 0 {
		for _, f := range di.RegularFiles {
			found[path.Join(prefix, f)] = struct{}{}
		}
		return nil
	}

	head, tail := patternSegments[0], patternSegments[1:]
	switch head {
	case "**": // zero or more segments
		// Zero-segment case: keep matching in the same directory.
		if err := walkGlobImpl(dirRel, tail, prefix, found); err != nil {
			return err
		}
		// One-or-more: recurse into every subdirectory, keep "**".
		for _, sd := range di.Subdirs {
			err := walkGlobImpl(path.Join(dirRel, sd), patternSegments, path.Join(prefix, sd), found)
			if err != nil {
				return err
			}
		}

	default: // ordinary component (may contain *, ?, [class])
		if len(tail) == 0 { // last segment — matches files
			for _, f := range di.RegularFiles {
				if ok, _ := path.Match(head, f); ok {
					found[path.Join(prefix, f)] = struct{}{}
				}
			}
			return nil
		}
		// Still more segments — match subdirectories
		for _, subDir := range di.Subdirs {
			if ok, _ := path.Match(head, subDir); !ok {
				continue
			}
			err := walkGlobImpl(path.Join(dirRel, subDir), tail, path.Join(prefix, subDir), found)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
