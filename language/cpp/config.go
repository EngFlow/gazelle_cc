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
func (c *cppLanguage) KnownDirectives() []string {
	return []string{
		"cc_grouping_mode",
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
		case "cc_grouping_mode":
			switch d.Value {
			case string(groupSourcesByDirectory):
				conf.groupingMode = groupSourcesByDirectory
			case string(groupSourcesByHeader):
				conf.groupingMode = groupSourcesByHeader
			case "default":
				conf.groupingMode = newCppConfig().groupingMode
			default:
				log.Printf("%v is invalid value for directive %v, expected one of %v, %v or default", d.Value, d.Key, groupSourcesByDirectory, groupSourcesByHeader)
			}
		}
	}
}

type cppConfig struct {
	groupingMode sourceGroupingMode
}

func getCppConfig(c *config.Config) *cppConfig {
	return c.Exts[languageName].(*cppConfig)
}
func newCppConfig() *cppConfig {
	return &cppConfig{
		groupingMode: groupSourcesByDirectory,
	}
}
func (conf *cppConfig) clone() *cppConfig {
	copy := *conf
	return &copy
}

type sourceGroupingMode string

const (
	groupSourcesByDirectory sourceGroupingMode = "directory"
	groupSourcesByHeader    sourceGroupingMode = "header"
)
