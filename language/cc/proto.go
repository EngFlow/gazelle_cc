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
	"path"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/language"
	"github.com/bazelbuild/bazel-gazelle/resolve"
	"github.com/bazelbuild/bazel-gazelle/rule"
)

const ccProtoLibraryFilesKey = "_protos"

// Generated a cc_proto_library rules based on outputs of protobuf proto_library
// Returns a set of .pb.h files that should be excluded from normal cc_library rules
func generateProtoLibraryRules(args language.GenerateArgs, result *language.GenerateResult) collections.Set[string] {
	consumedProtoFiles := make(collections.Set[string])
	protoMode := getProtoMode(args.Config)
	if !protoMode.ShouldGenerateRules() {
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
					consumedProtoFiles.Add(baseName + ".pb.h").Add(baseName + ".pb.cc")
				}
			}
			protoRuleLabel, err := label.Parse(":" + protoRule.Name())
			if err != nil {
				log.Panicf("Failed to parse proto_library label of %v", protoRule.Name())
			}
			baseName := strings.TrimSuffix(protoRuleLabel.Name, "_proto")
			ruleName := baseName + ccProtoRuleSufix
			newRule := rule.NewRule("cc_proto_library", ruleName)
			// Every cc_proto_library needs to have exactly 1 deps entry - the label or proto_library
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

func generateProtoImportSpecs(protoLibraryRule *rule.Rule, pkg string) []resolve.ImportSpec {
	if !slices.Contains(protoLibraryRule.PrivateAttrKeys(), ccProtoLibraryFilesKey) {
		return nil
	}

	// For each .proto in the target, index the compiler-generated header (foo.proto -> foo.pb.h).
	// This lets other rules resolve #include "pkg/foo.pb.h" even though the header does not appear in hdrs/outs.
	protos := protoLibraryRule.PrivateAttr(ccProtoLibraryFilesKey).([]string)
	imports := make([]resolve.ImportSpec, len(protos))
	for i, protoFile := range protos {
		if baseFileName, isProto := strings.CutSuffix(protoFile, ".proto"); isProto {
			generatedHeaderName := baseFileName + ".pb.h"
			imports[i] = resolve.ImportSpec{Lang: languageName, Imp: path.Join(pkg, generatedHeaderName)}
		}
	}
	return imports
}
