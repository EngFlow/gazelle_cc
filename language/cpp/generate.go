package cpp

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cpp/language/internal/cpp/parser"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func (c *cppLanguage) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	return c.genPackageByDirectory(args)
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
		sourceInfo, err := parser.ParseSourceFile(filePath)
		if err != nil {
			log.Printf("Failed to parse source %v, reason: %v", filePath, filePath)
			continue
		}
		sourceInfos[file] = sourceInfo
		switch {
		case hasMatchingExtension(file, headerExtensions):
			hdrs = append(hdrs, file)
		case strings.Contains(file, "_test."):
			testSrcs = append(testSrcs, file)
		case sourceInfo.HasMain:
			mainSrcs = append(mainSrcs, file)
		default:
			srcs = append(srcs, file)
		}
	}

	var result = language.GenerateResult{}
	baseName := filepath.Base(args.Dir)
	if len(srcs) > 0 || len(hdrs) > 0 {
		rule := rule.NewRule("cc_library", baseName)
		if len(srcs) > 0 {
			rule.SetAttr("srcs", srcs)
		}
		if len(hdrs) > 0 {
			rule.SetAttr("hdrs", hdrs)
		}
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

	if len(testSrcs) > 0 {
		// TODO: group tests by framework (unlikely but possible)
		ruleName := baseName + "_test"
		rule := rule.NewRule("cc_test", ruleName)
		rule.SetAttr("srcs", testSrcs)
		result.Gen = append(result.Gen, rule)
		result.Imports = append(result.Imports, extractImports(args, testSrcs, sourceInfos))
	}

	// None of the rules generated above can be empty - it's guaranteed by generating them only if sources exists
	// However we need to inspect for existing rules that are no longer matching any files
	result.Empty = append(result.Empty, c.findEmptyRules(args.File, result.Gen)...)

	return result
}

func (c *cppLanguage) findEmptyRules(file *rule.File, generatedRules []*rule.Rule) []*rule.Rule {
	if file == nil {
		return nil
	}

	emptyRules := []*rule.Rule{}
	for _, r := range file.Rules {
		// Nothing to check if rule with that name was just generated
		if slices.ContainsFunc(generatedRules, func(elem *rule.Rule) bool {
			return elem.Name() == r.Name()
		}) {
			continue
		}

		srcs := []string{}
		switch r.Kind() {
		case "cc_library":
			srcs = r.AttrStrings("srcs")
			srcs = append(srcs, r.AttrStrings("hdrs")...)
		case "cc_binary", "cc_test":
			srcs = r.AttrStrings("srcs")
		default:
			continue
		}

		srcsExist := slices.ContainsFunc(srcs, func(src string) bool {
			path := filepath.Join(file.Path, src)
			_, err := os.Stat(path)
			return err == nil // file exists and can be accessed
		})

		if srcsExist {
			continue
		}
		// Create a copy of the rule, using the original one might prevent it from deletion
		emptyRules = append(emptyRules, rule.NewRule(r.Kind(), r.Name()))
	}

	return emptyRules
}
