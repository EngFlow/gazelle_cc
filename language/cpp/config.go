package cpp

import (
	"flag"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// config.Configurer methods
func (*cppLanguage) Configure(c *config.Config, rel string, f *rule.File)         {}
func (*cppLanguage) RegisterFlags(fs *flag.FlagSet, cmd string, c *config.Config) {}
func (*cppLanguage) CheckFlags(fs *flag.FlagSet, c *config.Config) error          { return nil }
func (c *cppLanguage) KnownDirectives() []string                                  { return nil }
