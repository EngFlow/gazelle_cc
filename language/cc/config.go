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
	"flag"
	"log"
	"path/filepath"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// config.Configurer methods
func (*ccLanguage) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {}
func (*ccLanguage) CheckFlags(fs *flag.FlagSet, c *config.Config) error          { return nil }

const (
	cc_group_directive   = "cc_group"
	cc_group_unit_cycles = "cc_group_unit_cycles"
	cc_indexfile         = "cc_indexfile"
)

func (c *ccLanguage) KnownDirectives() []string {
	return []string{
		cc_group_directive,
		cc_group_unit_cycles,
		cc_indexfile,
	}
}

func (c *ccLanguage) Configure(config *config.Config, rel string, f *rule.File) {
	var conf *cppConfig
	if parentConf, ok := config.Exts[languageName]; !ok {
		conf = newCppConfig()
	} else {
		conf = parentConf.(*cppConfig).clone()
	}
	config.Exts[languageName] = conf

	if f == nil {
		return
	}

	for _, d := range f.Directives {
		switch d.Key {
		case cc_group_directive:
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

type cppConfig struct {
	// Defines how how sources should be grouped when defining rules
	groupingMode sourceGroupingMode
	// Should rules with sources assigned to different targets be merged into single one if they define a cyclic dependency
	groupsCycleHandlingMode groupsCycleHandlingMode
	// User defined dependency indexes based on the filename
	dependencyIndexes []ccDependencyIndex
}

func getCppConfig(c *config.Config) *cppConfig {
	return c.Exts[languageName].(*cppConfig)
}
func newCppConfig() *cppConfig {
	return &cppConfig{
		groupingMode:            groupSourcesByDirectory,
		groupsCycleHandlingMode: mergeOnGroupsCycle,
		dependencyIndexes:       []ccDependencyIndex{},
	}
}
func (conf *cppConfig) clone() *cppConfig {
	return &cppConfig{
		groupingMode:            conf.groupingMode,
		groupsCycleHandlingMode: conf.groupsCycleHandlingMode,
		// No deep cloning of dependency indexes to reduce memory usage
		dependencyIndexes: conf.dependencyIndexes[:len(conf.dependencyIndexes):len(conf.dependencyIndexes)],
	}
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
