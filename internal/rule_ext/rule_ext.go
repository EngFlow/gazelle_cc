// Copyright 2026 EngFlow Inc. All rights reserved.
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

// Package rule_ext provides extensions to the
// github.com/bazelbuild/bazel-gazelle/rule package.
package rule_ext

import (
	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
)

// AttrBool returns the value of the rule attribute key as a bool. If the
// attribute is absent or not a boolean literal (True/False), it returns
// defaultVal.
func AttrBool(r *rule.Rule, key string, defaultVal bool) bool {
	expr := r.Attr(key)
	if expr == nil {
		return defaultVal
	}
	ident, ok := expr.(*bzl.Ident)
	if !ok {
		return defaultVal
	}
	switch ident.Name {
	case "True":
		return true
	case "False":
		return false
	default:
		return defaultVal
	}
}
