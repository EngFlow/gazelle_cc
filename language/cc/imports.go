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
	"log"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/pathtools"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
	"github.com/bazelbuild/bazel-gazelle/walk"
	"github.com/bmatcuk/doublestar/v4"
)

// resolve.Resolver method
func (*ccLanguage) Imports(config *config.Config, rule *rule.Rule, buildFile *rule.File) []resolve.ImportSpec {
	switch rule.Kind() {
	case "cc_proto_library":
		return generateProtoImportSpecs(rule, buildFile.Pkg)
	case "cc_import", "cc_library", "cc_shared_library", "cc_static_library":
		return generateLibraryImportSpecs(config, rule, buildFile.Pkg)
	default:
		return nil
	}
}

func generateLibraryImportSpecs(config *config.Config, rule *rule.Rule, pkg string) []resolve.ImportSpec {
	attrs, err := getPublicInterfaceAttributes(config, rule, pkg)
	if err != nil {
		log.Printf("gazelle_cc: failed to collect attributes of %s(name = %q) defined in %s: %v", rule.Kind(), rule.Name(), pkg, err)
		return nil
	}

	// Maximum possible slice, each header is indexed:
	// - once for its fully-qualified path
	// - once for its virtual path (if include_prefix or strip_include_prefix is specified)
	// - at most once for every matching declared -I include directory
	imports := make([]resolve.ImportSpec, 0, len(attrs.hdrs)*(2+len(attrs.includes)))
	for _, hdr := range attrs.hdrs {
		// fullyQualifiedPath is the repository-root-relative path to the header. This path is always reachable via
		// #include, regardless of the rule's attributes: includes, include_prefix, and strip_include_prefix.
		fullyQualifiedPath := path.Join(pkg, hdr)
		imports = append(imports, resolve.ImportSpec{Lang: languageName, Imp: fullyQualifiedPath})

		// virtualPath allows to reference the header using modified path according to strip_include_prefix and
		// include_prefix attributes.
		if virtualPath := transformIncludePath(pkg, attrs.stripIncludePrefix, attrs.includePrefix, fullyQualifiedPath); virtualPath != fullyQualifiedPath {
			imports = append(imports, resolve.ImportSpec{Lang: languageName, Imp: virtualPath})
		}

		// Index shorter includes paths made valid by each -I <includeDir>
		// Bazel adds every entry in the `includes` attribute to the compiler’s search path.
		// With `includes=[include, include/ext]` header `include/ext/foo.h` can be referenced in 3 different ways:
		// - include/ext/foo.h - the fully qualified (canonical) form
		// - ext/foo.h - relative to the `include/` directory (1st 'includes' entry)
		// - foo.h - relative to the `include/ext/` directory (2nd 'includes' entry)
		// We index the an alterantive variants here if they are matching the includes directory.
		for _, includeDir := range attrs.includes {
			relativeTo := path.Join(pkg, includeDir)
			if includeDir == "." {
				// Include '.' is special: it makes the path resolvable based from directory defining BUILD file instead of repository root
				relativeTo = pkg
			}
			// Ensure the prefix ends with path separator to distinguish include=foo hdrs=[foo.h, foo/bar.h]
			// It was already cleaned so there won't be duplicate path seperators here
			relativeTo = relativeTo + string(filepath.Separator)
			relativePath, matching := strings.CutPrefix(fullyQualifiedPath, relativeTo)
			if !matching {
				// If the include directory is not relative to canonical form it's would be simply ignored.
				continue
			}
			imports = append(imports, resolve.ImportSpec{Lang: languageName, Imp: relativePath})
		}
	}

	return imports
}

type publicInterfaceAttributes struct {
	hdrs               []string
	includePrefix      string
	stripIncludePrefix string
	includes           []string
}

func getPublicInterfaceAttributes(config *config.Config, rule *rule.Rule, pkg string) (publicInterfaceAttributes, error) {
	hdrs, err := collectStringsAttr(config, rule, pkg, "hdrs")
	if err != nil {
		return publicInterfaceAttributes{}, err
	}
	return publicInterfaceAttributes{
		hdrs:               hdrs,
		includePrefix:      cleanPath(rule.AttrString("include_prefix")),
		stripIncludePrefix: cleanPath(rule.AttrString("strip_include_prefix")),
		includes:           collections.MapSlice(rule.AttrStrings("includes"), cleanPath),
	}, nil
}

func cleanPath(p string) string {
	if p != "" {
		return path.Clean(p)
	}
	return p
}

// transformIncludePath converts a path to a header file into a string by which
// the header file may be included, accounting for the library's
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
// includePrefix is the value of the target's include_prefix attribute. It's
// prepended to hdrRel after stripIncludePrefix is applied.
//
// Both includePrefix and stripIncludePrefix must be clean (with path.Clean) if
// they are non-empty.
//
// hdrRel is the slash-separated, repo-root-relative path to the header file.
func transformIncludePath(libRel, stripIncludePrefix, includePrefix, hdrRel string) string {
	// Strip the prefix.
	var effectiveStripIncludePrefix string
	if path.IsAbs(stripIncludePrefix) {
		effectiveStripIncludePrefix = stripIncludePrefix[len("/"):]
	} else if stripIncludePrefix != "" {
		effectiveStripIncludePrefix = path.Join(libRel, stripIncludePrefix)
	} else if includePrefix != "" {
		// Match Bazel's undocumented behavior by stripping the package name.
		effectiveStripIncludePrefix = libRel
	}
	cleanRel := pathtools.TrimPrefix(hdrRel, effectiveStripIncludePrefix)

	// Apply the new prefix.
	cleanRel = path.Join(includePrefix, cleanRel)

	return cleanRel
}

// collectStringsAttr collects the values of the given attribute from the rule.
// If the attribute is a list of strings, it returns the list. If the attribute
// is a glob, it expands the glob patterns relative to dir and returns the
// resulting paths.
func collectStringsAttr(config *config.Config, r *rule.Rule, dir, attrName string) ([]string, error) {
	// Fast path: plain list of strings in the BUILD file.
	if ss := r.AttrStrings(attrName); ss != nil {
		return ss, nil
	}

	expr := r.Attr(attrName) // nil if the attribute is not present
	if expr == nil {
		return nil, nil
	}
	if globValue, ok := rule.ParseGlobExpr(expr); ok {
		return expandGlob(config, dir, globValue)
	}
	return nil, nil
}

// expandGlob expands the glob patterns in the given glob value relative to
// relPath. It returns a sorted list of paths that match the patterns, excluding
// those that match the excludes. The paths are relative to relPath, and they
// are sorted in lexicographical order. It does not use I/O, it uses cached
// directory info obtained from walk.GetDirInfo so it might panic if the
// directory was not walked before.
func expandGlob(config *config.Config, dir string, glob rule.GlobValue) ([]string, error) {
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
