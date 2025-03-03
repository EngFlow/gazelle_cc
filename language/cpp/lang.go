package cpp

import (
	"path/filepath"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const languageName = "c++"

type cppLanguage struct{}

type cppInclude struct {
	// Include path extracted from brackets or double quotes
	rawPath string
	// Repository root directory relative rawPath for quoted include, rawPath otherwise
	normalizedPath string
	// True when include defined using brackets
	isSystemInclude bool
}

type cppImports struct {
	includes []cppInclude
	// TODO: module imports / exports
}

func NewLanguage() language.Language {
	return &cppLanguage{}
}

// language.Language methods
func (c *cppLanguage) Kinds() map[string]rule.KindInfo {
	return map[string]rule.KindInfo{
		"cc_library": {
			NonEmptyAttrs:  map[string]bool{"srcs": true},
			MergeableAttrs: map[string]bool{"srcs": true, "hdrs": true, "deps": true},
		},
		"cc_binary": {
			NonEmptyAttrs:  map[string]bool{"srcs": true},
			MergeableAttrs: map[string]bool{"srcs": true, "deps": true},
		},
		"cc_test": {
			NonEmptyAttrs:  map[string]bool{"srcs": true},
			MergeableAttrs: map[string]bool{"srcs": true, "deps": true},
		},
	}
}

var ccRuleDefs = []string{
	"cc_library", "cc_shared_libary", "cc_static_library",
	"cc_import",
	"cc_binary",
	"cc_test",
}

func (c *cppLanguage) Loads() []rule.LoadInfo {
	return []rule.LoadInfo{
		{
			Name:    "@rules_cc//cc:defs.bzl",
			Symbols: ccRuleDefs,
		},
	}
}
func (*cppLanguage) Fix(c *config.Config, f *rule.File) {}

var sourceExtensions = []string{".c", ".cc", ".cpp", ".cxx", ".c++", ".S"}
var headerExtensions = []string{".h", ".hh", ".hpp", ".hxx"}
var cExtensions = append(sourceExtensions, headerExtensions...)

func hasMatchingExtension(filename string, extensions []string) bool {
	ext := filepath.Ext(filename)
	for _, validExt := range extensions {
		if strings.EqualFold(ext, validExt) { // Case-insensitive comparison
			return true
		}
	}
	return false
}
