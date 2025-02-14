package cpp

import (
	"log"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cpp/language/internal/cpp/parser"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func (c *cppLanguage) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	srcInfo := collectSourceInfos(args)
	var result = language.GenerateResult{}
	c.generateLibraryRule(args, srcInfo, &result)
	c.generateBinaryRules(args, srcInfo, &result)
	c.generateTestRule(args, srcInfo, &result)

	// None of the rules generated above can be empty - it's guaranteed by generating them only if sources exists
	// However we need to inspect for existing rules that are no longer matching any files
	result.Empty = c.findEmptyRules(args.File, srcInfo, result.Gen)

	return result
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

func (c *cppLanguage) generateLibraryRule(args language.GenerateArgs, srcInfo ccSourceInfoSet, result *language.GenerateResult) {
	allSrcs := slices.Concat(srcInfo.srcs, srcInfo.hdrs)
	if len(allSrcs) == 0 {
		return
	}
	baseName := filepath.Base(args.Dir)
	rule := rule.NewRule("cc_library", baseName)
	if len(srcInfo.srcs) > 0 {
		rule.SetAttr("srcs", srcInfo.srcs)
	}
	if len(srcInfo.hdrs) > 0 {
		rule.SetAttr("hdrs", srcInfo.hdrs)
	}
	if args.File == nil || !args.File.HasDefaultVisibility() {
		rule.SetAttr("visibility", []string{"//visibility:public"})
	}
	result.Gen = append(result.Gen, rule)
	result.Imports = append(result.Imports, extractImports(args, allSrcs, srcInfo.sourceInfos))
}

func (c *cppLanguage) generateBinaryRules(args language.GenerateArgs, srcInfo ccSourceInfoSet, result *language.GenerateResult) {
	for _, mainSrc := range srcInfo.mainSrcs {
		ruleName := strings.TrimSuffix(mainSrc, filepath.Ext(mainSrc))
		rule := rule.NewRule("cc_binary", ruleName)
		rule.SetAttr("srcs", []string{mainSrc})
		result.Gen = append(result.Gen, rule)
		result.Imports = append(result.Imports, extractImports(args, []string{mainSrc}, srcInfo.sourceInfos))
	}
}

func (c *cppLanguage) generateTestRule(args language.GenerateArgs, srcInfo ccSourceInfoSet, result *language.GenerateResult) {
	if len(srcInfo.testSrcs) == 0 {
		return
	}
	// TODO: group tests by framework (unlikely but possible)
	baseName := filepath.Base(args.Dir)
	ruleName := baseName + "_test"
	rule := rule.NewRule("cc_test", ruleName)
	rule.SetAttr("srcs", srcInfo.testSrcs)
	result.Gen = append(result.Gen, rule)
	result.Imports = append(result.Imports, extractImports(args, srcInfo.testSrcs, srcInfo.sourceInfos))
}

type ccSourceInfoSet struct {
	// Sources of regular (library) files
	srcs []string
	// Headers
	hdrs []string
	// Sources containing main methods
	mainSrcs []string
	// Sources containing tests or defined in tests context
	testSrcs []string
	// Files that are unrecognised as CC sources
	unmatched []string
	// Map contaning informations extracted from recognized CC source
	sourceInfos map[string]parser.SourceInfo
}

func (s *ccSourceInfoSet) buildableSources() []string {
	return slices.Concat(s.srcs, s.hdrs, s.mainSrcs, s.testSrcs)
}
func (s *ccSourceInfoSet) containsBuildableSource(src string) bool {
	return slices.Contains(s.srcs, src) ||
		slices.Contains(s.hdrs, src) ||
		slices.Contains(s.mainSrcs, src) ||
		slices.Contains(s.testSrcs, src)
}

// Collects and groups files that can be used to generate CC rules based on it's local context
// Parses all matched CC source files to extract additional context
func collectSourceInfos(args language.GenerateArgs) ccSourceInfoSet {
	res := ccSourceInfoSet{}
	res.sourceInfos = map[string]parser.SourceInfo{}

	for _, file := range args.RegularFiles {
		if !hasMatchingExtension(file, cExtensions) {
			res.unmatched = append(res.unmatched, file)
			continue
		}
		filePath := filepath.Join(args.Dir, file)
		sourceInfo, err := parser.ParseSourceFile(filePath)
		if err != nil {
			log.Printf("Failed to parse source %v, reason: %v", filePath, err)
			continue
		}
		res.sourceInfos[file] = sourceInfo
		switch {
		case hasMatchingExtension(file, headerExtensions):
			res.hdrs = append(res.hdrs, file)
		case strings.Contains(file, "_test."):
			res.testSrcs = append(res.testSrcs, file)
		case sourceInfo.HasMain:
			res.mainSrcs = append(res.mainSrcs, file)
		default:
			res.srcs = append(res.srcs, file)
		}
	}
	return res
}

func (c *cppLanguage) findEmptyRules(file *rule.File, srcInfo ccSourceInfoSet, generatedRules []*rule.Rule) []*rule.Rule {
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

		// Check wheter at least 1 file mentioned in rule definition sources is buildable (exists)
		srcsExist := slices.ContainsFunc(srcs, func(src string) bool {
			return srcInfo.containsBuildableSource(src)
		})

		if srcsExist {
			continue
		}
		// Create a copy of the rule, using the original one might prevent it from deletion
		emptyRules = append(emptyRules, rule.NewRule(r.Kind(), r.Name()))
	}

	return emptyRules
}
