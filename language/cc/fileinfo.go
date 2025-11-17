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
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/EngFlow/gazelle_cc/language/internal/cc/parser"
	"github.com/EngFlow/gazelle_cc/language/internal/cc/platform"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/pathtools"
)

// fileKind determines how a file should be added to rules, based on its
// extension, location, and contents. fileKind influences but doesn't completely
// determine what kind of rule (cc_library, cc_binary, cc_test) or what
// attribute (srcs, hdrs) a file gets assigned to. This can also depend on
// the grouping mode and includes between files.
type fileKind byte

const (
	// unknownKind is assigned to files not handled by gazelle_cc.
	unknownKind fileKind = iota

	// libHdrKind is a header file (.h) that's not in a test directory.
	libHdrKind

	// libSrcKind is an implementation file (.cc) that's not in a test directory
	// and doesn't have a main function.
	libSrcKind

	// binSrcKind is an implementation file that has a main function.
	binSrcKind

	// testSrcKind is an implementation file (.cc) that is in a test directory
	// or has "test" in its name.
	testSrcKind
)

// fileInfo collects metadata about an individual source or header file.
type fileInfo struct {
	// Relative path to the file from the directory containing the build file.
	// May contain slashes if we're including contents of subdirectories.
	name string

	kind fileKind

	// hasMain is true if the file contains a main function. Two or more files
	// with main functions usually can't be grouped into the same rule.
	hasMain bool

	// List of files included by this file.
	includes []ccInclude
}

// getFileInfo parses a file and returns metadata describing it.
func getFileInfo(
	args language.GenerateArgs,
	platformEnvs map[platform.Platform]parser.Environment,
	name string,
	subdirKind subdirKind) (fileInfo, error) {

	if !hasMatchingExtension(name, ccExtensions) {
		return fileInfo{}, errUnmatchedExtension
	}
	conf := getCcConfig(args.Config)
	filePath := filepath.Join(args.Dir, name)
	sourceInfo, err := parser.ParseSourceFile(filePath)
	if err != nil {
		return fileInfo{}, err
	}

	// Evaluate the directives and search for platform specific include paths
	// We do it for each enabled platform using it's unique set of macros
	platformIncludes := map[string][]platform.Platform{}
	for platform, macros := range platformEnvs {
		reachable := sourceInfo.CollectReachableIncludes(macros)
		for _, include := range reachable {
			platformIncludes[include.Path] = append(platformIncludes[include.Path], platform)
		}
	}

	// Assign all includes found in the directives
	includeDirectives := sourceInfo.CollectIncludes()
	includes := make([]ccInclude, len(includeDirectives))
	for i, include := range sourceInfo.CollectIncludes() {
		usedByPlatforms := platformIncludes[include.Path]
		isPlatformSpecific := len(usedByPlatforms) != len(platformEnvs)
		includes[i] = ccInclude{
			sourceFile:         path.Join(args.Rel, name),
			lineNumber:         include.LineNumber,
			path:               path.Clean(include.Path),
			isSystemInclude:    include.IsSystem,
			isPlatformSpecific: isPlatformSpecific,
			platforms:          usedByPlatforms,
		}
	}

	base := path.Base(name)
	stem := base[:len(base)-len(path.Ext(base))]
	isTest := strings.HasPrefix(stem, "test") || strings.HasSuffix(stem, "test")
	var kind fileKind
	if subdirKind != noSubdir {
		// In subdirectory mode, classify files mostly based on their directory
		// names. File extensions are less important.
		switch {
		case subdirKind == includeSubidr && fileNameIsHeader(name):
			kind = libHdrKind
		case isTest || subdirKind == testSubdir:
			kind = testSrcKind
		case sourceInfo.HasMain:
			kind = binSrcKind
		default:
			kind = libSrcKind
		}
	} else {
		// In other modes or in subdirectory mode when a file is not in a
		// subdirectory, classify files mostly based on their extension.
		inTestDirectory := pathtools.Index(args.Rel, "test") >= 0 ||
			pathtools.Index(args.Rel, "tests") >= 0 ||
			conf.matchesSubdirectoryTestPatterns(path.Dir(name))

		switch {
		case inTestDirectory:
			kind = testSrcKind
		case fileNameIsHeader(name):
			kind = libHdrKind
		case isTest:
			kind = testSrcKind
		case sourceInfo.HasMain:
			kind = binSrcKind
		default:
			kind = libSrcKind
		}
	}

	return fileInfo{
		name:     name,
		includes: includes,
		kind:     kind,
		hasMain:  sourceInfo.HasMain,
	}, nil
}

type subdirKind byte

const (
	noSubdir subdirKind = iota
	srcSubdir
	includeSubidr
	testSubdir
)

// checkSubdirKind returns how the subdirectory's contents should be treated
// in subdirectory mode. It returns noSubdir if we're not in subdirectory mode,
// if the subdirectory doesn't match any patterns, or if the subdirectory
// contains a build file. It returns an error if the subdirecotry matches
// multiple patterns.
func checkSubdirKind(conf *ccConfig, buildFileDirRels collections.Set[string], rel, subdir string) (subdirKind, error) {
	subdirRel := path.Join(rel, subdir)
	if conf.groupingMode != groupSourcesBySubdirectory || buildFileDirRels.Contains(subdirRel) {
		return noSubdir, nil
	}
	if buildFileDirRels.Contains(subdirRel) {
		return noSubdir, nil
	}

	var sty subdirKind
	matchCount := 0
	if conf.matchesSubdirectorySrcPatterns(subdir) {
		sty = srcSubdir
		matchCount++
	}
	if conf.matchesSubdirectoryIncludePatterns(subdir) {
		sty = includeSubidr
		matchCount++
	}
	if conf.matchesSubdirectoryTestPatterns(subdir) {
		sty = testSubdir
		matchCount++
	}
	if matchCount > 1 {
		return noSubdir, fmt.Errorf("directory %s matches more than one of the cc_subdirectory_srcs, cc_subdirectory_hdrs, cc_subdirectory_test patterns. Make sure the patterns are unambiguous.", path.Join(rel, subdir))
	}
	return sty, nil
}

var errUnmatchedExtension = errors.New("unmatched file extension")
