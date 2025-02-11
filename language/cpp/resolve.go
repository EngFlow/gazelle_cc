package cpp

import (
	"path"

	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/repo"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

// resolve.Resolver methods
func (c *cppLanguage) Name() string                                        { return languageName }
func (c *cppLanguage) Embeds(r *rule.Rule, from label.Label) []label.Label { return nil }

func (*cppLanguage) Imports(c *config.Config, r *rule.Rule, f *rule.File) []resolve.ImportSpec {
	rel := f.Pkg
	prefix := rel
	hdrs := r.AttrStrings("hdrs")
	imports := make([]resolve.ImportSpec, len(hdrs))
	for i, hdr := range hdrs {
		imports[i] = resolve.ImportSpec{Lang: languageName, Imp: path.Join(prefix, hdr)}
	}
	return imports
}

func (*cppLanguage) Resolve(c *config.Config, ix *resolve.RuleIndex, rc *repo.RemoteCache, r *rule.Rule, imports interface{}, from label.Label) {
	if imports == nil {
		return
	}

	cppImports := imports.(cppImports)
	deps := []label.Label{}

	for _, include := range cppImports.includes {
		resolvedLabel := resolveImportSpec(c, ix, from, resolve.ImportSpec{Lang: languageName, Imp: include.normalizedPath})
		if resolvedLabel != label.NoLabel {
			deps = append(deps, resolvedLabel)
		}

		// Retry to resolve is external dependency was defined using quotes instead of braces
		if !include.isSystemInclude {
			resolvedLabel = resolveImportSpec(c, ix, from, resolve.ImportSpec{Lang: languageName, Imp: include.rawPath})
			if resolvedLabel != label.NoLabel {
				deps = append(deps, resolvedLabel)
			}
		}
	}

	if len(deps) > 0 {
		r.SetAttr("deps", deps)
	}
}

func resolveImportSpec(c *config.Config, ix *resolve.RuleIndex, from label.Label, importSpec resolve.ImportSpec) label.Label {
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
	return label.NoLabel
}
