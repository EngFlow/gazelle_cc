package cpp

import (
	"log"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cpp/language/internal/cpp/parser"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func (c *cppLanguage) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	srcInfo := collectSourceInfos(args)
	rulesInfo := extractRulesInfo(args)

	var result = language.GenerateResult{}
	c.generateLibraryRules(args, srcInfo, rulesInfo, &result)
	c.generateBinaryRules(args, srcInfo, &rulesInfo, &result)
	c.generateTestRule(args, srcInfo, &rulesInfo, &result)

	// None of the rules generated above can be empty - it's guaranteed by generating them only if sources exists
	// However we need to inspect for existing rules that are no longer matching any files
	result.Empty = slices.Concat(result.Empty, c.findEmptyRules(args.File, srcInfo, rulesInfo, result.Gen))

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

func (c *cppLanguage) generateLibraryRules(args language.GenerateArgs, srcInfo ccSourceInfoSet, rulesInfo rulesInfo, result *language.GenerateResult) {
	conf := getCppConfig(args.Config)
	allSrcs := slices.Concat(srcInfo.srcs, srcInfo.hdrs)
	if len(allSrcs) == 0 {
		return
	}
	var srcGroups sourceGroups
	switch conf.groupingMode {
	case groupSourcesByDirectory:
		// All sources grouped together
		groupName := groupId(filepath.Base(args.Dir))
		srcGroups = sourceGroups{groupName: {sources: allSrcs}}
	case groupSourcesByUnit:
		srcGroups = groupSourcesByUnits(allSrcs, srcInfo.sourceInfos)
	}

	ambigiousRuleAssignments := srcGroups.adjustToExistingRules(rulesInfo)

	for _, groupId := range srcGroups.groupIds() {
		group := srcGroups[groupId]
		ruleName := string(groupId)
		newRule := rule.NewRule("cc_library", ruleName)
		// If there is only 1 target target rule and exactly 1 existing rule reuse it
		switch len(srcGroups) {
		case 1:
			existingRules := rulesInfo.existingRulesOfKind("cc_library", args)
			if len(existingRules) == 1 {
				existing := existingRules[0]
				newRule.SetName(existing.Name())
				// Use exisitng kind only when is an alias. Required to allow for correct merge
				// In case of mapped kinds it would lead to problems in resolve
				if _, exists := args.Config.AliasMap[existing.Kind()]; exists {
					newRule.SetKind(existing.Kind())
				}
			}
		}

		// Deal with rules that conflict with existing defintions
		if ambigiousRuleAssignments, exists := ambigiousRuleAssignments[groupId]; exists {
			if !c.handleAmbigiousRulesAssignment(args, conf, srcInfo, rulesInfo, newRule, result, *group, ambigiousRuleAssignments) {
				continue // Failed to handle issue, skip this group. New rule could have been modified
			}
		}

		// Assign sources to gorups
		srcs, hdrs := partitionCSources(group.sources)
		if len(srcs) > 0 {
			newRule.SetAttr("srcs", sourceFilesToStrings(srcs))
		}
		if len(hdrs) > 0 {
			newRule.SetAttr("hdrs", sourceFilesToStrings(hdrs))
		}
		if args.File == nil || !args.File.HasDefaultVisibility() {
			newRule.SetAttr("visibility", []string{"//visibility:public"})
		}

		result.Gen = append(result.Gen, newRule)
		result.Imports = append(result.Imports, extractImports(args, group.sources, srcInfo.sourceInfos))
	}
}

func (c *cppLanguage) generateBinaryRules(args language.GenerateArgs, srcInfo ccSourceInfoSet, rulesInfo *rulesInfo, result *language.GenerateResult) {
	for _, binSource := range srcInfo.mainSrcs {
		ruleName := binSource.baseName()
		rule := rule.NewRule("cc_binary", ruleName)
		// If there exists exactly 1 existing rule and 1 target reuse it
		switch len(srcInfo.mainSrcs) {
		case 1:
			existingRules := rulesInfo.existingRulesOfKind("cc_binary", args)
			if len(existingRules) == 1 {
				existing := existingRules[0]
				rule.SetName(existing.Name())
				// Use exisitng kind only when is an alias. Required to allow for correct merge
				// In case of mapped kinds it would lead to problems in resolve
				if _, exists := args.Config.AliasMap[existing.Kind()]; exists {
					rule.SetKind(existing.Kind())
				}
			}
		}

		rule.SetAttr("srcs", []string{binSource.stringValue()})
		result.Gen = append(result.Gen, rule)
		result.Imports = append(result.Imports, extractImports(args, []sourceFile{binSource}, srcInfo.sourceInfos))
	}
}

func (c *cppLanguage) generateTestRule(args language.GenerateArgs, srcInfo ccSourceInfoSet, rulesInfo *rulesInfo, result *language.GenerateResult) {
	if len(srcInfo.testSrcs) == 0 {
		return
	}
	// TODO: group tests by framework (unlikely but possible)
	baseName := filepath.Base(args.Dir)
	ruleName := baseName + "_test"
	rule := rule.NewRule("cc_test", ruleName)

	// If there exists exactly 1 existing rule and 1 target reuse it
	existingRules := rulesInfo.existingRulesOfKind("cc_test", args)
	if len(existingRules) == 1 {
		existing := existingRules[0]
		rule.SetName(existing.Name())
		// Use exisitng kind only when is an alias. Required to allow for correct merge
		// In case of mapped kinds it would lead to problems in resolve
		if _, exists := args.Config.AliasMap[existing.Kind()]; exists {
			rule.SetKind(existing.Kind())
		}
	}

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

// Adjust created sourceGroups based of information from existing rules defintions.
// * merges with or renames group if all of it sources were previously assigned to existing rule
// Returns ambigiousRuleAssignments defining a list of groupIds leading to ambigious assignment under the new state -
// it typically happens when previously independant rules are now creating a cycle
func (srcGroups *sourceGroups) adjustToExistingRules(rulesInfo rulesInfo) (ambigiousRuleAssignments map[groupId][]string) {
	ambigiousRuleAssignments = make(map[groupId][]string)
	// Dictionary of groups that previously were assignled to multiple rules
	for id, group := range *srcGroups {
		// Collect info about previous assignment of sources to rules creating this group
		assignedToRules := make(map[string]bool)
		for _, src := range group.sources {
			if groupName, exists := rulesInfo.groupAssignment[src.toGroupId()]; exists {
				assignedToRules[groupName] = true
			}
		}
		assignedToRuleNames := slices.Collect(maps.Keys(assignedToRules))
		switch len(assignedToRuleNames) {
		case 0:
			// None of the sources are assigned to existing groups, would create a fresh one
		case 1:
			// Some of sources were already assigned to rule, would use it as a base
			existingGroupId := groupId(assignedToRuleNames[0])
			if id != existingGroupId {
				srcGroups.renameOrMergeWith(id, existingGroupId)
			}
		default:
			ambigiousRuleAssignments[id] = assignedToRuleNames
		}
	}
	return ambigiousRuleAssignments
}

// Resolve conflicts when resolved sourceGroups do conflict with existing rule definitions.
// It mostly deals with problems when sources creating a cyclic dependency are defined in multiple existing rules:
// * if allowRulesMerge merges all rules refering to this group sources into a single rule
// * otherwise warns user about cyclic deps and sets cyclic deps attributes to newRule and returns false
// Returns true if successfully handled issues and it's possible to finalize creation of newRule
func (c *cppLanguage) handleAmbigiousRulesAssignment(args language.GenerateArgs, conf *cppConfig, srcInfo ccSourceInfoSet, rulesInfo rulesInfo, newRule *rule.Rule, result *language.GenerateResult, group sourceGroup, ambigiousRuleAssignments []string) (handled bool) {
	switch conf.groupsCycleHandlingMode {
	case mergeOnGroupsCycle:
		// Merge rules creating a cyclic dependency into a single rule and remove old ones
		var mergeReason string
		switch conf.groupingMode {
		case groupSourcesByDirectory:
			mergeReason = "are invalidating the 'cc_group directive' setting"
		case groupSourcesByUnit:
			mergeReason = "create a cyclic dependency"
		default:
			log.Panicf("Unexpected groupingMode: %v", conf.groupingMode)
		}
		log.Printf("Rules %v defined in %v %v, their sources %v would be merged into a single rule '%v'. "+
			"To prevent automatic merging of rules set `# gazelle:%v %v`",
			slices.Sorted(slices.Values(ambigiousRuleAssignments)), args.Dir, mergeReason, slices.Sorted(slices.Values(group.sources)), newRule.Name(),
			cc_group_unit_cycles, warnOnGroupsCycle,
		)
		for _, referedRuleName := range ambigiousRuleAssignments {
			referedRule := rulesInfo.definedRules[referedRuleName]
			if err := rule.SquashRules(referedRule, newRule, args.File.Path); err != nil {
				log.Printf("Failed to join rules %v and %v defining a cyclic dependency: %v", referedRuleName, newRule.Name(), err)
				return false // Skip processing these groups, keep existing rules unchanged
			}
			// Remove no longer exisitng rules
			if referedRuleName != newRule.Name() && slices.Contains(group.subGroups, groupId(newRule.Name())) {
				result.Empty = append(result.Empty, rule.NewRule(referedRule.Kind(), referedRule.Name()))
			}
		}
		return true
	case warnOnGroupsCycle:
		// Merging was disabled by user, don't edit existing rules
		slices.Sort(ambigiousRuleAssignments) // for deterministic output
		log.Printf(
			"Existing cc_library rules %v defined in %v form a cyclic dependency. Possible resolutions:\n"+
				"  - Set `# gazelle:%v %v` to automatically merge targets to avoid cyclic dependencies.\n"+
				"  - Manually combine targets to avoid cyclic dependencies.\n"+
				"  - Remove `#include`s from source files that cause cyclic dependencies: %v",
			ambigiousRuleAssignments, args.File.Path, cc_group_unit_cycles, mergeOnGroupsCycle, group.sources)
		// Collect labels to rules creating a cycle
		deps := make([]label.Label, len(ambigiousRuleAssignments))
		for idx, group := range ambigiousRuleAssignments {
			deps[idx] = label.New("", "", group)
		}
		// Set recursive dependencies to all rules creating a cycle
		for _, subGroupId := range group.subGroups {
			rule, exists := rulesInfo.definedRules[string(subGroupId)]
			if !exists {
				continue
			}
			rule.SetAttr("deps", deps)
			result.Gen = append(result.Gen, rule)
			result.Imports = append(result.Imports, extractImports(args, group.sources, srcInfo.sourceInfos))
		}
		return false // Skip processing these groups, keep existing rules unchanged
	default:
		log.Panicf("Unknown group cycle handling mode: %v", conf.groupsCycleHandlingMode)
		return false
	}
}

func (c *cppLanguage) findEmptyRules(file *rule.File, srcInfo ccSourceInfoSet, rulesInfo rulesInfo, generatedRules []*rule.Rule) []*rule.Rule {
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
		sourceFiles := slices.Collect(maps.Keys(rulesInfo.ccRuleSources[r.Name()]))
		// Check whether at least 1 file mentioned in rule definition sources is buildable (exists)
		srcsExist := slices.ContainsFunc(sourceFiles, func(src sourceFile) bool {
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

type rulesInfo struct {
	// Map of all rules defined in existing file for quick reference based on rule name
	definedRules map[string]*rule.Rule
	// Sources previously assigned to cc rules, key is the existing name of the rule
	ccRuleSources map[string]sourceFileSet
	// Mapping between groupId created from sourceFile and existing rule name to which it was previously assigned
	groupAssignment map[groupId]string
}

func extractRulesInfo(args language.GenerateArgs) rulesInfo {
	info := rulesInfo{
		definedRules:    make(map[string]*rule.Rule),
		ccRuleSources:   make(map[string]sourceFileSet),
		groupAssignment: make(map[groupId]string),
	}
	if args.File == nil {
		return info
	}
	for _, rule := range args.File.Rules {
		ruleName := rule.Name()
		info.definedRules[ruleName] = rule
		assignSources := func(srcs []string) {
			for _, filename := range srcs {
				srcFile := sourceFile(filename)
				if _, exists := info.ccRuleSources[ruleName]; !exists {
					info.ccRuleSources[ruleName] = make(sourceFileSet)
				}
				info.ccRuleSources[ruleName][srcFile] = true
				info.groupAssignment[srcFile.toGroupId()] = ruleName
			}
		}
		switch resolveCCRuleKind(rule.Kind(), args.Config) {
		case "cc_library":
			assignSources(rule.AttrStrings("srcs"))
			assignSources(rule.AttrStrings("hdrs"))
		case "cc_binary":
			assignSources(rule.AttrStrings("srcs"))
		case "cc_test":
			assignSources(rule.AttrStrings("srcs"))
		}
	}
	return info
}

func resolveCCRuleKind(kind string, config *config.Config) string {
	if target, exists := config.AliasMap[kind]; exists {
		return target
	}
	for _, mapping := range config.KindMap {
		if mapping.KindName == kind {
			return mapping.FromKind
		}
	}
	return kind
}

// Return list of existing rules of kind or with matching kind mapping
func (info *rulesInfo) existingRulesOfKind(kind string, args language.GenerateArgs) []*rule.Rule {
	rules := make([]*rule.Rule, 0, len(info.ccRuleSources))
	for _, rule := range info.definedRules {
		if resolveCCRuleKind(rule.Kind(), args.Config) == kind {
			rules = append(rules, rule)
		}
	}
	return rules
}
