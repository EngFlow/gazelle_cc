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
	"flag"
	"log"
	"path"
	"path/filepath"
	"unicode"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// config.Configurer methods
func (*ccLanguage) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {}
func (*ccLanguage) CheckFlags(fs *flag.FlagSet, c *config.Config) error          { return nil }

const (
	cc_group             = "cc_group"
	cc_group_unit_cycles = "cc_group_unit_cycles"
	cc_indexfile         = "cc_indexfile"
	cc_search            = "cc_search"
)

func (c *ccLanguage) KnownDirectives() []string {
	return []string{
		cc_group,
		cc_group_unit_cycles,
		cc_indexfile,
		cc_search,
	}
}

func (c *ccLanguage) Configure(config *config.Config, rel string, f *rule.File) {
	var conf *ccConfig
	if parentConf, ok := config.Exts[languageName]; !ok {
		conf = newCcConfig()
	} else {
		conf = parentConf.(*ccConfig).clone()
	}
	config.Exts[languageName] = conf

	if f == nil {
		return
	}

	for _, d := range f.Directives {
		switch d.Key {
		case cc_group:
			selectDirectiveChoice(&conf.groupingMode, sourceGroupingModes, d)
		case cc_group_unit_cycles:
			selectDirectiveChoice(&conf.groupsCycleHandlingMode, groupsCycleHandlingModes, d)
		case cc_indexfile:
			// New indexfiles replace inherited ones
			if d.Value == "" {
				conf.dependencyIndexes = []ccDependencyIndex{}
				continue
			}
			path := filepath.Join(config.WorkDir, d.Value)
			if filepath.IsAbs(d.Value) {
				log.Printf("gazelle_cc: absolute paths for %v directive are not allowed, %v would be ignored", d.Key, d.Value)
				continue
			}
			index, err := loadDependencyIndex(path)
			if err != nil {
				log.Printf("gazelle_cc: failed to load cc dependencies index: %v, it would be ignored. Reason: %v", path, err)
				continue
			}
			conf.dependencyIndexes = append(conf.dependencyIndexes, index)
		case cc_search:
			if d.Value == "" {
				// Special syntax (empty value) to reset directive.
				conf.ccSearch = defaultCcSearch()
			} else {
				args, err := splitQuoted(d.Value)
				if err != nil {
					log.Print(err)
					continue
				}
				if len(args) == 0 || len(args) > 2 {
					log.Printf("# gazelle:cc_search got %d arguments, expected up to 2, an include prefix to strip, and an include prefix to add", len(args))
					continue
				}
				s := ccSearch{stripIncludePrefix: args[0]}
				if s.stripIncludePrefix != "" {
					if path.Clean(s.stripIncludePrefix) != s.stripIncludePrefix {
						log.Printf("# gazelle:cc_search: strip_include_prefix path %q is not clean", s.stripIncludePrefix)
						continue
					}
					if path.IsAbs(s.stripIncludePrefix) {
						log.Printf("# gazelle:cc_search: strip_include_prefix path %q must be relative", s.stripIncludePrefix)
						continue
					}
				}
				if len(args) > 1 {
					s.includePrefix = args[1]
				}
				if s.includePrefix != "" {
					if path.Clean(s.includePrefix) != s.includePrefix {
						log.Printf("# gazelle:cc_search: include_prefix path %q is not clean", s.includePrefix)
						continue
					}
					if path.IsAbs(s.includePrefix) {
						log.Printf("# gazelle:cc_search: include_prefix path %q must be relative", s.includePrefix)
						continue
					}
				}
				conf.ccSearch = append(conf.ccSearch, s)
			}
		}
	}
}

// Compares the directive value with list of expected choices. If there is a match it updates the target with matching value
// If there is no match is emits warning on stderr
func selectDirectiveChoice[T ~string](target *T, options []T, d rule.Directive) {
	for _, choice := range options {
		if string(choice) == d.Value {
			*target = choice
			return
		}
	}
	log.Printf("Invalid value for directive %v, expected one of %v, got: %v", d.Key, options, d.Value)
}

type ccConfig struct {
	// Defines how how sources should be grouped when defining rules
	groupingMode sourceGroupingMode
	// Should rules with sources assigned to different targets be merged into single one if they define a cyclic dependency
	groupsCycleHandlingMode groupsCycleHandlingMode
	// User defined dependency indexes based on the filename
	dependencyIndexes []ccDependencyIndex
	// List of 'gazelle:cc_search' directives, used to construct RelsToIndex.
	ccSearch []ccSearch
}

type ccSearch struct {
	// stripIncludePrefix is slash-separated relative path that is removed from
	// the include path when constructing the directory path to search.
	// The include is ignored if this does not match.
	stripIncludePrefix string

	// includePrefix is a slash-separated relative path that is prepended to
	// the included path after stripIncludePrefix is removed when constructing
	// the directory path to search. This does not affect matching.
	includePrefix string
}

func getCcConfig(c *config.Config) *ccConfig {
	return c.Exts[languageName].(*ccConfig)
}

func newCcConfig() *ccConfig {
	return &ccConfig{
		groupingMode:            groupSourcesByDirectory,
		groupsCycleHandlingMode: mergeOnGroupsCycle,
		dependencyIndexes:       []ccDependencyIndex{},
		ccSearch:                defaultCcSearch(),
	}
}

func (conf *ccConfig) clone() *ccConfig {
	return &ccConfig{
		groupingMode:            conf.groupingMode,
		groupsCycleHandlingMode: conf.groupsCycleHandlingMode,
		// No deep cloning of dependency indexes to reduce memory usage
		dependencyIndexes: conf.dependencyIndexes[:len(conf.dependencyIndexes):len(conf.dependencyIndexes)],
		ccSearch:          conf.ccSearch[:len(conf.ccSearch):len(conf.ccSearch)],
	}
}

// defaultCcSearch returns a list of search paths containing only the repository
// root directory with no prefix. This matches what Bazel does by default.
// We don't ask the user to write this explicitly.
func defaultCcSearch() []ccSearch {
	return []ccSearch{{}}
}

type sourceGroupingMode string

var sourceGroupingModes = []sourceGroupingMode{groupSourcesByDirectory, groupSourcesByUnit}

const (
	// single cc_library per directory
	groupSourcesByDirectory sourceGroupingMode = "directory"
	// cc_library per translation unit or group of recursivelly dependant translation units
	groupSourcesByUnit sourceGroupingMode = "unit"
)

type groupsCycleHandlingMode string

var groupsCycleHandlingModes = []groupsCycleHandlingMode{mergeOnGroupsCycle, warnOnGroupsCycle}

const (
	// All groups forming a cycle would be merged into a single one
	mergeOnGroupsCycle groupsCycleHandlingMode = "merge"
	// Don't modify rules forming a cycle, let user handle it manually
	warnOnGroupsCycle groupsCycleHandlingMode = "warn"
)

// splitQuoted splits the string s around each instance of one or more consecutive
// white space characters while taking into account quotes and escaping, and
// returns an array of substrings of s or an empty list if s contains only white space.
// Single quotes and double quotes are recognized to prevent splitting within the
// quoted region, and are removed from the resulting substrings. If a quote in s
// isn't closed err will be set and r will have the unclosed argument as the
// last element. The backslash is used for escaping.
//
// For example, the following string:
//
//	a b:"c d" 'e''f'  "g\""
//
// Would be parsed as:
//
//	[]string{"a", "b:c d", "ef", `g"`}
//
// Copied from go/build.splitQuoted
func splitQuoted(s string) (r []string, err error) {
	var args []string
	arg := make([]rune, len(s))
	escaped := false
	quoted := false
	quote := '\x00'
	i := 0
	for _, rune := range s {
		switch {
		case escaped:
			escaped = false
		case rune == '\\':
			escaped = true
			continue
		case quote != '\x00':
			if rune == quote {
				quote = '\x00'
				continue
			}
		case rune == '"' || rune == '\'':
			quoted = true
			quote = rune
			continue
		case unicode.IsSpace(rune):
			if quoted || i > 0 {
				quoted = false
				args = append(args, string(arg[:i]))
				i = 0
			}
			continue
		}
		arg[i] = rune
		i++
	}
	if quoted || i > 0 {
		args = append(args, string(arg[:i]))
	}
	if quote != 0 {
		err = errors.New("unclosed quote")
	} else if escaped {
		err = errors.New("unfinished escaping")
	}
	return args, err
}
