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
	"log"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/EngFlow/gazelle_cc/language/internal/cc/parser"
	"github.com/bazelbuild/bazel-gazelle/config"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/language/proto"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

func (c *ccLanguage) GenerateRules(args language.GenerateArgs) language.GenerateResult {
	srcInfo := collectSourceInfos(args)
	rulesInfo := extractRulesInfo(args)

	var result = language.GenerateResult{}
	consumedProtoFiles := c.generateProtoLibraryRules(args, rulesInfo, &result)
	c.generateLibraryRules(args, srcInfo, rulesInfo, consumedProtoFiles, &result)
	c.generateBinaryRules(args, srcInfo, rulesInfo, &result)
	c.generateTestRules(args, srcInfo, rulesInfo, &result)

	// None of the rules generated above can be empty - it's guaranteed by generating them only if sources exists
	// However we need to inspect for existing rules that are no longer matching any files
	result.Empty = slices.Concat(result.Empty, c.findEmptyRules(args, srcInfo, rulesInfo, result.Gen))

	result.RelsToIndex = c.listRelsToIndex(args, srcInfo)

	return result
}

func extractImports(args language.GenerateArgs, files []sourceFile, sourceInfos map[sourceFile]parser.SourceInfo) ccImports {
	imports := ccImports{}
	for _, file := range files {
		var includes *[]ccInclude
		if file.isHeader() {
			includes = &imports.hdrIncludes
		} else {
			includes = &imports.srcIncludes
		}

		sourceInfo := sourceInfos[file]
		for _, include := range sourceInfo.Includes {
			*includes = append(*includes, ccInclude{
				path:            path.Clean(include.Path),
				fromDirectory:   args.Rel,
				isSystemInclude: include.IsSystemInclude,
			})
		}
	}

	return imports
}

func splitSourcesIntoGroups(args language.GenerateArgs, srcs []sourceFile, srcInfo ccSourceInfoSet) sourceGroups {
	conf := getCcConfig(args.Config)
	var srcGroups sourceGroups
	switch conf.groupingMode {
	case groupSourcesByDirectory:
		// All sources grouped together
		groupName := groupId(filepath.Base(args.Dir))
		srcGroups = sourceGroups{groupName: {sources: srcs}}
	case groupSourcesByUnit:
		srcGroups = groupSourcesByUnits(srcs, srcInfo.sourceInfos)
	}
	return srcGroups
}

/* Helper merthod to create new rule of given type that is aware of existing context.
 * If there exists exactly 1 new group of given kind the returned rule would reuse it's name and possibly aliased kind
 */
func newOrExistingRule(kind string, ruleName string, srcGroups sourceGroups, rulesInfo rulesInfo, args language.GenerateArgs) *rule.Rule {
	newRule := rule.NewRule(kind, ruleName)
	// If there is only 1 target target rule and exactly 1 existing rule reuse it
	if len(srcGroups) == 1 {
		existingRules := rulesInfo.existingRulesOfKind(kind, args)
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
	return newRule
}

func (c *ccLanguage) generateLibraryRules(args language.GenerateArgs, srcInfo ccSourceInfoSet, rulesInfo rulesInfo, excludedSources sourceFileSet, result *language.GenerateResult) {
	conf := getCcConfig(args.Config)
	// Ignore files that might have been consumed by other rules
	allSrcs := []sourceFile{}
	for _, file := range slices.Concat(srcInfo.srcs, srcInfo.hdrs) {
		if isExcluded := excludedSources[file]; !isExcluded {
			allSrcs = append(allSrcs, file)
		}
	}
	if len(allSrcs) == 0 {
		return
	}
	srcGroups := splitSourcesIntoGroups(args, allSrcs, srcInfo)
	ambigiousRuleAssignments := srcGroups.adjustToExistingRules(rulesInfo)

	for _, groupId := range srcGroups.groupIds() {
		group := srcGroups[groupId]
		ruleName := string(groupId)
		newRule := newOrExistingRule("cc_library", ruleName, srcGroups, rulesInfo, args)

		// Deal with rules that conflict with existing defintions
		if ambigiousRuleAssignments, exists := ambigiousRuleAssignments[groupId]; exists {
			if !c.handleAmbigiousRulesAssignment(args, conf, srcInfo, rulesInfo, newRule, result, *group, ambigiousRuleAssignments) {
				continue // Failed to handle issue, skip this group. New rule could have been modified
			}
		}

		// Assign sources to gorups
		srcs, hdrs := partitionCSources(group.sources)
		if len(srcs) > 0 {
			newRule.SetAttr("srcs", toRelativePaths(args.Rel, srcs))
		}
		if len(hdrs) > 0 {
			newRule.SetAttr("hdrs", toRelativePaths(args.Rel, hdrs))
		}
		if args.File == nil || !args.File.HasDefaultVisibility() {
			newRule.SetAttr("visibility", []string{"//visibility:public"})
		}

		result.Gen = append(result.Gen, newRule)
		result.Imports = append(result.Imports, extractImports(args, group.sources, srcInfo.sourceInfos))
	}
}

func (c *ccLanguage) generateBinaryRules(args language.GenerateArgs, srcInfo ccSourceInfoSet, rulesInfo rulesInfo, result *language.GenerateResult) {
	srcGroups := identitySourceGroups(srcInfo.mainSrcs)
	for _, groupId := range srcGroups.groupIds() {
		group := srcGroups[groupId]
		ruleName := group.sources[0].baseName()
		newRule := newOrExistingRule("cc_binary", ruleName, srcGroups, rulesInfo, args)
		newRule.SetAttr("srcs", toRelativePaths(args.Rel, group.sources))
		result.Gen = append(result.Gen, newRule)
		result.Imports = append(result.Imports, extractImports(args, group.sources, srcInfo.sourceInfos))
	}
}

func (c *ccLanguage) generateTestRules(args language.GenerateArgs, srcInfo ccSourceInfoSet, rulesInfo rulesInfo, result *language.GenerateResult) {
	if len(srcInfo.testSrcs) == 0 {
		return
	}
	// TODO: group tests by framework (unlikely but possible)
	conf := getCcConfig(args.Config)
	srcGroups := splitSourcesIntoGroups(args, srcInfo.testSrcs, srcInfo)
	ambigiousRuleAssignments := srcGroups.adjustToExistingRules(rulesInfo)

	for _, groupId := range srcGroups.groupIds() {
		group := srcGroups[groupId]
		ruleName := string(groupId)
		if !(strings.HasSuffix(ruleName, "test") || strings.HasPrefix(ruleName, "test")) {
			ruleName = ruleName + "_test"
		}
		newRule := newOrExistingRule("cc_test", ruleName, srcGroups, rulesInfo, args)

		// Deal with rules that conflict with existing defintions
		if ambigiousRuleAssignments, exists := ambigiousRuleAssignments[groupId]; exists {
			if !c.handleAmbigiousRulesAssignment(args, conf, srcInfo, rulesInfo, newRule, result, *group, ambigiousRuleAssignments) {
				continue // Failed to handle issue, skip this group. New rule could have been modified
			}
		}
		newRule.SetAttr("srcs", toRelativePaths(args.Rel, group.sources))
		result.Gen = append(result.Gen, newRule)
		result.Imports = append(result.Imports, extractImports(args, group.sources, srcInfo.sourceInfos))
	}
}

// Generated a cc_proto_library rules based on outputs of protobuf proto_library
// Returns a set of .pb.h files that should be excluded from normal cc_library rules
func (c *ccLanguage) generateProtoLibraryRules(args language.GenerateArgs, rulesInfo rulesInfo, result *language.GenerateResult) sourceFileSet {
	consumedProtoFiles := make(sourceFileSet)
	protoConfig := proto.GetProtoConfig(args.Config)
	if protoConfig == nil || !protoConfig.Mode.ShouldGenerateRules() {
		// Don't create or delete proto rules in this mode.
		// All pb.h would be added to cc_library
		return consumedProtoFiles
	}
	const ccProtoRuleSufix = "_cc_proto"
	for _, protoRule := range args.OtherGen {
		switch protoRule.Kind() {
		case "proto_library":
			protoFiles := protoRule.AttrStrings("srcs")
			if len(protoFiles) == 0 {
				continue
			}
			for _, file := range protoFiles {
				// If generated pb.h files exists exclude it, refer to cc_proto_library instead
				if baseName, isProto := strings.CutSuffix(file, ".proto"); isProto {
					consumedProtoFiles[newSourceFile(args.Rel, baseName+".pb.h")] = true
					consumedProtoFiles[newSourceFile(args.Rel, baseName+".pb.cc")] = true
				}
			}
			protoRuleLabel, err := label.Parse(":" + protoRule.Name())
			if err != nil {
				log.Panicf("Failed to parse proto_library label of %v", protoRule.Name())
			}
			baseName := strings.TrimSuffix(protoRuleLabel.Name, "_proto")
			ruleName := baseName + ccProtoRuleSufix
			newRule := newOrExistingRule("cc_proto_library", ruleName, nil, rulesInfo, args)
			// Every cc_proto_library needs to have exactyl 1 deps entry - the label or proto_library
			// https://github.com/protocolbuffers/protobuf/blob/d3560e72e791cb61c24df2a1b35946efbd972738/bazel/private/bazel_cc_proto_library.bzl#L132-L142
			newRule.SetAttr("deps", []label.Label{protoRuleLabel})
			newRule.SetPrivateAttr(ccProtoLibraryFilesKey, protoFiles)

			if args.File == nil || !args.File.HasDefaultVisibility() {
				newRule.SetAttr("visibility", []string{"//visibility:public"})
			}

			result.Gen = append(result.Gen, newRule)
			result.Imports = append(result.Imports, ccImports{})
		}
	}
	for _, r := range args.OtherEmpty {
		if r.Kind() == "proto_library" {
			ccProtoName := strings.TrimSuffix(r.Name(), "_proto") + ccProtoRuleSufix
			result.Empty = append(result.Empty, rule.NewRule("cc_proto_library", ccProtoName))
		}
	}
	return consumedProtoFiles
}

// Source file path relative to the workspace directory
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

func newSourceFile(directory string, filename string) sourceFile {
	return sourceFile(path.Join(directory, filename))
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
		file := newSourceFile(args.Rel, fileName)
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
		baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		baseName = strings.ToLower(baseName)
		switch {
		case hasMatchingExtension(fileName, headerExtensions):
			res.hdrs = append(res.hdrs, file)
		case strings.HasPrefix(baseName, "test") || strings.HasSuffix(baseName, "test"):
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
func (c *ccLanguage) handleAmbigiousRulesAssignment(args language.GenerateArgs, conf *ccConfig, srcInfo ccSourceInfoSet, rulesInfo rulesInfo, newRule *rule.Rule, result *language.GenerateResult, group sourceGroup, ambigiousRuleAssignments []string) (handled bool) {
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
			slices.Sorted(slices.Values(ambigiousRuleAssignments)), args.Dir, mergeReason, slices.Sorted(slices.Values(toRelativePaths(args.Rel, group.sources))), newRule.Name(),
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

func (c *ccLanguage) findEmptyRules(args language.GenerateArgs, srcInfo ccSourceInfoSet, rulesInfo rulesInfo, generatedRules []*rule.Rule) []*rule.Rule {
	file := args.File
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

		if !slices.Contains(knownRuleKinds, resolveCCRuleKind(r.Kind(), args.Config)) {
			// This rule is not managed by gazelle_cc
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

func (c *ccLanguage) listRelsToIndex(args language.GenerateArgs, srcInfo ccSourceInfoSet) []string {
	relsToIndex := []string{}
	relsToIndexSeen := make(map[string]struct{})
	conf := getCcConfig(args.Config)
	for _, si := range srcInfo.sourceInfos {
		for _, inc := range si.Includes {
			dir := path.Dir(path.Clean(inc.Path))
			if dir == "." {
				dir = ""
			}
			for _, ccSearch := range conf.ccSearch {
				relToIndex := transformIncludePath("", ccSearch.stripIncludePrefix, ccSearch.includePrefix, dir)
				if _, ok := relsToIndexSeen[relToIndex]; ok {
					continue
				}
				relsToIndexSeen[relToIndex] = struct{}{}
				relsToIndex = append(relsToIndex, relToIndex)
			}
		}
	}
	sort.Strings(relsToIndex) // for determinism
	return relsToIndex
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
				srcFile := newSourceFile(args.Rel, filename)
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
