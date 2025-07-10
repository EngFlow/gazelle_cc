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
	"slices"

	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
)

// Represents bzl.Expr build from concatenation of []string and select expressions.
// Similar to @gazelle//language/go PlatformStrings but is decoupled from it's go specific constraints
type CcPlatformStrings struct {
	Generic     rule.SortedStrings         // always‑active strings
	Constrained rule.SelectStringListValue // keyed by constraint label
}

func (ps CcPlatformStrings) BzlExpr() bzl.Expr {
	var parts []bzl.Expr

	if len(ps.Generic) > 0 {
		parts = append(parts, rule.ExprFromValue(ps.Generic))
	}
	if len(ps.Constrained) > 0 {
		sel := make(rule.SelectStringListValue)
		sel["//conditions:default"] = nil // keep Gazelle formatting stable
		maps.Copy(sel, ps.Constrained)
		parts = append(parts, sel.BzlExpr())
	}

	switch len(parts) {
	case 0:
		return &bzl.ListExpr{}
	case 1:
		return parts[0]
	default:
		expr := parts[0]
		if lst, ok := expr.(*bzl.ListExpr); ok {
			lst.ForceMultiLine = true
		}
		for _, p := range parts[1:] {
			expr = &bzl.BinaryExpr{Op: "+", X: expr, Y: p}
		}
		return expr
	}
}

func (ps CcPlatformStrings) Merge(other bzl.Expr) bzl.Expr {
	otherPS := parseCcPlatformStrings(other)

	// Merge generic list via rule helper
	mergedGeneric := rule.MergeList(
		rule.ExprFromValue(ps.Generic).(*bzl.ListExpr),
		rule.ExprFromValue(otherPS.Generic).(*bzl.ListExpr),
	)

	// Helper: convert map -> *bzl.DictExpr so we can use MergeDict
	toDict := func(m map[string][]string) *bzl.DictExpr {
		d := &bzl.DictExpr{}
		for k, v := range m {
			d.List = append(d.List, &bzl.KeyValueExpr{
				Key:   &bzl.StringExpr{Value: k},
				Value: rule.ExprFromValue(v),
			})
		}
		return d
	}

	mergedDict, err := rule.MergeDict(toDict(ps.Constrained), toDict(otherPS.Constrained))
	if err != nil {
		log.Panicf("Failed to merge dicts: %v", err)
	}

	// Dict back to Go map
	mergedConstrained := map[string][]string{}
	if mergedDict != nil {
		for _, kv := range mergedDict.List {
			k := kv.Key.(*bzl.StringExpr).Value
			var items []string
			for _, it := range kv.Value.(*bzl.ListExpr).List {
				items = append(items, it.(*bzl.StringExpr).Value)
			}
			mergedConstrained[k] = items
		}
	}

	return CcPlatformStrings{
		Generic:     listToStrings(mergedGeneric),
		Constrained: mergedConstrained,
	}.BzlExpr()
}

// Strings flattens Generic + all constrained lists (in that order).
func (ps CcPlatformStrings) Strings() []string {
	out := slices.Clone(ps.Generic)
	for _, grp := range ps.Constrained {
		out = append(out, grp...)
	}
	return out
}

func parseCcPlatformStrings(expr bzl.Expr) CcPlatformStrings {
	ps := CcPlatformStrings{Constrained: make(map[string][]string)}

	switch e := expr.(type) {
	case *bzl.ListExpr:
		for _, it := range e.List {
			if s, ok := it.(*bzl.StringExpr); ok {
				ps.Generic = append(ps.Generic, s.Value)
			}
		}

	case *bzl.BinaryExpr:
		left := parseCcPlatformStrings(e.X)
		right := parseCcPlatformStrings(e.Y)

		ps.Generic = append(left.Generic, right.Generic...)
		for k, v := range left.Constrained {
			ps.Constrained[k] = append(ps.Constrained[k], v...)
		}
		for k, v := range right.Constrained {
			ps.Constrained[k] = append(ps.Constrained[k], v...)
		}

	case *bzl.CallExpr:
		if sel, ok := e.X.(*bzl.Ident); ok && sel.Name == "select" && len(e.List) == 1 {
			if dict, ok := e.List[0].(*bzl.DictExpr); ok {
				for _, kv := range dict.List {
					key, ok1 := kv.Key.(*bzl.StringExpr)
					val, ok2 := kv.Value.(*bzl.ListExpr)
					if !ok1 || !ok2 {
						continue
					}
					var items []string
					for _, v := range val.List {
						if s, ok := v.(*bzl.StringExpr); ok {
							items = append(items, s.Value)
						}
					}
					ps.Constrained[key.Value] = items
				}
			}
		}
	}
	return ps
}

func listToStrings(lst *bzl.ListExpr) []string {
	if lst == nil {
		return []string{}
	}
	out := make([]string, 0, len(lst.List))
	for _, e := range lst.List {
		out = append(out, e.(*bzl.StringExpr).Value)
	}
	return out
}
