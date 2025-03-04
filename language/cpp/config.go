package cpp

import (
	"flag"
	"log"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// config.Configurer methods
func (*cppLanguage) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {}
func (*cppLanguage) CheckFlags(fs *flag.FlagSet, c *config.Config) error          { return nil }

const (
	cc_group_directive   = "cc_group"
	cc_group_unit_cycles = "cc_group_unit_cycles"
)

func (c *cppLanguage) KnownDirectives() []string {
	return []string{
		cc_group_directive,
		cc_group_unit_cycles,
	}
}

func (*cppLanguage) Configure(c *config.Config, rel string, f *rule.File) {
	var conf *cppConfig
	if parentConf, ok := c.Exts[languageName]; !ok {
		conf = newCppConfig()
	} else {
		conf = parentConf.(*cppConfig).clone()
	}
	c.Exts[languageName] = conf

	if f == nil {
		return
	}

	for _, d := range f.Directives {
		switch d.Key {
		case cc_group_directive:
			selectDirectiveChoice(&conf.groupingMode, sourceGroupingModes, d)
		case cc_group_unit_cycles:
			selectDirectiveChoice(&conf.groupsCycleHandlingMode, groupsCycleHandlingModes, d)
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
}

func getCppConfig(c *config.Config) *cppConfig {
	return c.Exts[languageName].(*cppConfig)
}
func newCppConfig() *cppConfig {
	return &cppConfig{
		groupingMode:            groupSourcesByDirectory,
		groupsCycleHandlingMode: mergeOnGroupsCycle,
	}
}
func (conf *cppConfig) clone() *cppConfig {
	copy := *conf
	return &copy
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
