package cpplang

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/EngFlow/gazelle_cpp/language/cpp/parser"

	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func (c *cppLanguage) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	return c.genPackageByDirectory(args)
}

func containsString(elements []string, value string) bool {
	for _, elem := range elements {
		if value == elem {
			return true
		}
	}
	return false
}

func extractImports(args language.GenerateArgs, files []string, sourceInfos map[string]parser.SourceInfo) cppImports {
	includes := []cppInclude{}
	for _, file := range files {
		sourceInfo := sourceInfos[file]
		for _, include := range sourceInfo.Includes.DoubleQuote {
			includes = append(includes, cppInclude{rawPath: include, normalizedPath: path.Join(args.Rel, include), isSystemInclude: false})
		}
		for _, include := range sourceInfo.Includes.Bracket {
			includes = append(includes, cppInclude{rawPath: include, normalizedPath: include, isSystemInclude: true})
		}
	}
	return cppImports{includes: includes}
}

func (c *cppLanguage) genPackageByDirectory(args language.GenerateArgs) language.GenerateResult {
	sourceInfos := map[string]parser.SourceInfo{}
	hdrs := []string{}
	srcs := []string{}
	testSrcs := []string{}
	mainSrcs := []string{}
	unmatchedFiles := []string{}

	for _, file := range args.RegularFiles {
		if !hasMatchingExtension(file, cExtensions) {
			unmatchedFiles = append(unmatchedFiles, file)
			continue
		}
		filePath := filepath.Join(args.Dir, file)
		if sourceInfo, err := parser.ParseSourceFile(filePath); err == nil {
			sourceInfos[file] = sourceInfo
			if hasMatchingExtension(file, headerExtensions) {
				hdrs = append(hdrs, file)
			} else {
				if strings.Contains(file, "_test.") {
					testSrcs = append(testSrcs, file)
				} else if sourceInfo.HasMain {
					mainSrcs = append(mainSrcs, file)
				} else {
					srcs = append(srcs, file)
				}
			}
		}
	}

	var result = language.GenerateResult{}
	baseName := filepath.Base(args.Dir)
	if len(srcs) > 0 || len(hdrs) > 0 {
		rule := rule.NewRule("cc_library", baseName)
		if len(srcs) > 0 {
			rule.SetAttr("srcs", srcs)
		}
		rule.SetAttr("hdrs", hdrs)
		if args.File == nil || !args.File.HasDefaultVisibility() {
			rule.SetAttr("visibility", []string{"//visibility:public"})
		}
		result.Gen = append(result.Gen, rule)
		result.Imports = append(result.Imports, extractImports(args, append(srcs, hdrs...), sourceInfos))
	}

	for _, mainSrc := range mainSrcs {
		ruleName := strings.TrimSuffix(mainSrc, filepath.Ext(mainSrc))
		rule := rule.NewRule("cc_binary", ruleName)
		rule.SetAttr("srcs", []string{mainSrc})
		result.Gen = append(result.Gen, rule)
		result.Imports = append(result.Imports, extractImports(args, []string{mainSrc}, sourceInfos))
	}

	for _, testSrc := range testSrcs {
		// The rule is named the same as the test file
		ruleName := strings.TrimSuffix(testSrc, filepath.Ext(testSrc))
		rule := rule.NewRule("cc_test", ruleName)
		rule.SetAttr("srcs", []string{testSrc})
		result.Gen = append(result.Gen, rule)
		result.Imports = append(result.Imports, extractImports(args, testSrcs, sourceInfos))
	}

	return result
}
