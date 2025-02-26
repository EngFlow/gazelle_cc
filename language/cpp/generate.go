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
	c.generateLibraryRules(args, srcInfo, &result)
	c.generateBinaryRules(args, srcInfo, &result)
	c.generateTestRule(args, srcInfo, &result)

	// None of the rules generated above can be empty - it's guaranteed by generating them only if sources exists
	// However we need to inspect for existing rules that are no longer matching any files
	result.Empty = c.findEmptyRules(args.File, srcInfo, result.Gen)

	return result
}

func extractImports(args language.GenerateArgs, files []sourceFile, sourceInfos map[sourceFile]parser.SourceInfo) cppImports {
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

func (c *cppLanguage) generateLibraryRules(args language.GenerateArgs, srcInfo ccSourceInfoSet, result *language.GenerateResult) {
	allSrcs := slices.Concat(srcInfo.srcs, srcInfo.hdrs)
	if len(allSrcs) == 0 {
		return
	}
	var srcGroups sourceGroups
	switch getCppConfig(args.Config).groupingMode {
	case groupSourcesByDirectory:
		// All sources grouped together
		groupName := groupId(filepath.Base(args.Dir))
		srcGroups = sourceGroups{groupName: {sources: allSrcs}}
	case groupSourcesByUnit:
		srcGroups = groupSourcesByHeaders(allSrcs, srcInfo.sourceInfos)
	}

	for _, groupId := range srcGroups.groupIds() {
		group := srcGroups[groupId]
		rule := rule.NewRule("cc_library", string(groupId))
		srcs, hdrs := partitionCSources(group.sources)
		if len(srcs) > 0 {
			rule.SetAttr("srcs", sourceFilesToStrings(srcs))
		}
		if len(hdrs) > 0 {
			rule.SetAttr("hdrs", sourceFilesToStrings(hdrs))
		}
		if args.File == nil || !args.File.HasDefaultVisibility() {
			rule.SetAttr("visibility", []string{"//visibility:public"})
		}
		imports := extractImports(
			args,
			group.sources,
			srcInfo.sourceInfos,
		)
		result.Gen = append(result.Gen, rule)
		result.Imports = append(result.Imports, imports)
	}
}

func (c *cppLanguage) generateBinaryRules(args language.GenerateArgs, srcInfo ccSourceInfoSet, result *language.GenerateResult) {
	for _, mainSrc := range srcInfo.mainSrcs {
		ruleName := mainSrc.baseName()
		rule := rule.NewRule("cc_binary", ruleName)
		rule.SetAttr("srcs", []string{mainSrc.stringValue()})
		result.Gen = append(result.Gen, rule)
		result.Imports = append(result.Imports, extractImports(args, []sourceFile{mainSrc}, srcInfo.sourceInfos))
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
	rule.SetAttr("srcs", sourceFilesToStrings(srcInfo.testSrcs))
	result.Gen = append(result.Gen, rule)
	result.Imports = append(result.Imports, extractImports(args, srcInfo.testSrcs, srcInfo.sourceInfos))
}

type sourceFile string
type sourceInfos map[sourceFile]parser.SourceInfo
type ccSourceInfoSet struct {
	// Sources of regular (library) files
	srcs []sourceFile
	// Headers
	hdrs []sourceFile
	// Sources containing main methods
	mainSrcs []sourceFile
	// Sources containing tests or defined in tests context
	testSrcs []sourceFile
	// Files that are unrecognized as CC sources
	unmatched []sourceFile
	// Map containing information extracted from recognized CC source
	sourceInfos sourceInfos
}

func (s *ccSourceInfoSet) buildableSources() []sourceFile {
	return slices.Concat(s.srcs, s.hdrs, s.mainSrcs, s.testSrcs)
}
func (s *ccSourceInfoSet) containsBuildableSource(src sourceFile) bool {
	return slices.Contains(s.srcs, src) ||
		slices.Contains(s.hdrs, src) ||
		slices.Contains(s.mainSrcs, src) ||
		slices.Contains(s.testSrcs, src)
}

// Collects and groups files that can be used to generate CC rules based on it's local context
// Parses all matched CC source files to extract additional context
func collectSourceInfos(args language.GenerateArgs) ccSourceInfoSet {
	res := ccSourceInfoSet{}
	res.sourceInfos = map[sourceFile]parser.SourceInfo{}

	for _, fileName := range args.RegularFiles {
		file := sourceFile(fileName)
		if !hasMatchingExtension(fileName, cExtensions) {
			res.unmatched = append(res.unmatched, file)
			continue
		}
		filePath := filepath.Join(args.Dir, fileName)
		sourceInfo, err := parser.ParseSourceFile(filePath)
		if err != nil {
			log.Printf("Failed to parse source %v, reason: %v", filePath, err)
			continue
		}
		res.sourceInfos[file] = sourceInfo
		switch {
		case hasMatchingExtension(fileName, headerExtensions):
			res.hdrs = append(res.hdrs, file)
		case strings.Contains(fileName, "_test."):
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

		var srcs []string
		switch r.Kind() {
		case "cc_library":
			srcs = r.AttrStrings("srcs")
			srcs = append(srcs, r.AttrStrings("hdrs")...)
		case "cc_binary", "cc_test":
			srcs = r.AttrStrings("srcs")
		default:
			continue
		}

		// Check whether at least 1 file mentioned in rule definition sources is buildable (exists)
		srcsExist := slices.ContainsFunc(srcs, func(src string) bool {
			return srcInfo.containsBuildableSource(sourceFile(src))
		})

		if srcsExist {
			continue
		}
		// Create a copy of the rule, using the original one might prevent it from deletion
		emptyRules = append(emptyRules, rule.NewRule(r.Kind(), r.Name()))
	}

	return emptyRules
}
