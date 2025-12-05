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
	"fmt"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bazelbuild/bazel-gazelle/rule"
	bzl "github.com/bazelbuild/buildtools/build"
)

const (
	selectFunctionName = "select"
	selectDefaultKey   = "//conditions:default"
)

// Represents bzl.Expr build from concatenation of []string and select
// expressions. Similar to @gazelle//rule platformStringsExprs but is decoupled
// from it's go specific constraints.
//
// The matched expression has the form:
//
// [] + select({})
//
// The 2 collections may appear in any order, and some or all of them may be
// omitted (all fields are nil for a nil expression).
type ccPlatformStringsExprs struct {
	Generic     *bzl.ListExpr // always active dependencies
	Constrained *bzl.DictExpr // constrained dependencies
}

var _ rule.BzlExprValue = ccPlatformStringsExprs{}
var _ rule.Merger = ccPlatformStringsExprs{}

func labelsSetToStringSlice(labels collections.Set[label.Label]) []string {
	return collections.MapSlice(labels.Values(), func(l label.Label) string { return l.String() })
}

func labelsSetToListExpr(labels collections.Set[label.Label]) *bzl.ListExpr {
	return rule.SortedStrings(labelsSetToStringSlice(labels)).BzlExpr().(*bzl.ListExpr)
}

func labelsSetToOptionalListExpr(labels collections.Set[label.Label]) *bzl.ListExpr {
	if len(labels) == 0 {
		return nil
	}
	return labelsSetToListExpr(labels)
}

func labelsMapToStringMap(labels map[label.Label]collections.Set[label.Label]) map[string][]string {
	result := make(map[string][]string, len(labels))
	for key, value := range labels {
		result[key.String()] = labelsSetToStringSlice(value)
	}
	return result
}

func labelsMapToDictExpr(labels map[label.Label]collections.Set[label.Label]) *bzl.DictExpr {
	stringMap := labelsMapToStringMap(labels)
	if _, haveDefault := stringMap[selectDefaultKey]; !haveDefault {
		// always include default condition
		stringMap[selectDefaultKey] = nil
	}
	return rule.SelectStringListValue(stringMap).BzlExpr().(*bzl.CallExpr).List[0].(*bzl.DictExpr)
}

func labelsMapToOptionalDictExpr(labels map[label.Label]collections.Set[label.Label]) *bzl.DictExpr {
	if len(labels) == 0 {
		return nil
	}
	return labelsMapToDictExpr(labels)
}

func newCcPlatformStringsExprs(
	generic collections.Set[label.Label],
	constrainted map[label.Label]collections.Set[label.Label],
) ccPlatformStringsExprs {
	return ccPlatformStringsExprs{
		Generic:     labelsSetToOptionalListExpr(generic),
		Constrained: labelsMapToOptionalDictExpr(constrainted),
	}
}

func (ps ccPlatformStringsExprs) makeSelectExpr() bzl.Expr {
	return &bzl.CallExpr{
		X:    &bzl.Ident{Name: selectFunctionName},
		List: []bzl.Expr{ps.Constrained},
	}
}

func (ps ccPlatformStringsExprs) makeBinaryExpr() bzl.Expr {
	ps.Generic.ForceMultiLine = true
	ps.Constrained.ForceMultiLine = true
	return &bzl.BinaryExpr{
		Op: "+",
		X:  ps.Generic,
		Y:  ps.makeSelectExpr(),
	}
}

func (ps ccPlatformStringsExprs) BzlExpr() bzl.Expr {
	if ps.Constrained == nil {
		// always active dependencies only
		return ps.Generic
	}

	if ps.Generic == nil {
		// constrained dependencies only
		return ps.makeSelectExpr()
	}

	// both always active and constrained dependencies
	return ps.makeBinaryExpr()
}

func (ps ccPlatformStringsExprs) Merge(other bzl.Expr) bzl.Expr {
	otherPS, err := parseCcPlatformStringsExprs(other)
	if err != nil {
		// leave current BUILD content unchanged on error
		return other
	}

	ps.Generic = rule.MergeList(ps.Generic, otherPS.Generic)
	ps.Constrained, err = rule.MergeDict(ps.Constrained, otherPS.Constrained)
	if err != nil {
		// leave current BUILD content unchanged on error
		return other
	}

	return ps.BzlExpr()
}

func parseSelectExpr(expr *bzl.CallExpr) (*bzl.DictExpr, error) {
	function, ok := expr.X.(*bzl.Ident)
	if !ok || function.Name != selectFunctionName || len(expr.List) != 1 {
		return nil, fmt.Errorf("expression could not be matched: callee other than select or wrong number of args")
	}
	arg, ok := expr.List[0].(*bzl.DictExpr)
	if !ok {
		return nil, fmt.Errorf("expression could not be matched: select argument not dict")
	}
	return arg, nil
}

func (ps ccPlatformStringsExprs) union(other ccPlatformStringsExprs) (ccPlatformStringsExprs, error) {
	if ps.Generic != nil && other.Generic != nil {
		return ccPlatformStringsExprs{}, fmt.Errorf("unexpected [] + []")
	}
	if ps.Constrained != nil && other.Constrained != nil {
		return ccPlatformStringsExprs{}, fmt.Errorf("unexpected select({}) + select({})")
	}
	if ps.Generic == nil {
		ps.Generic = other.Generic
	}
	if ps.Constrained == nil {
		ps.Constrained = other.Constrained
	}
	return ps, nil
}

func parseCcPlatformStringsExprs(expr bzl.Expr) (ccPlatformStringsExprs, error) {
	var ps ccPlatformStringsExprs
	if expr == nil {
		return ps, nil
	}

	switch expr := expr.(type) {
	case *bzl.ListExpr:
		ps.Generic = expr

	case *bzl.CallExpr:
		dict, err := parseSelectExpr(expr)
		if err != nil {
			return ccPlatformStringsExprs{}, err
		}
		ps.Constrained = dict

	case *bzl.BinaryExpr:
		left, err := parseCcPlatformStringsExprs(expr.X)
		if err != nil {
			return ccPlatformStringsExprs{}, err
		}
		right, err := parseCcPlatformStringsExprs(expr.Y)
		if err != nil {
			return ccPlatformStringsExprs{}, err
		}
		ps, err = left.union(right)
		if err != nil {
			return ccPlatformStringsExprs{}, err
		}

	default:
		return ccPlatformStringsExprs{}, fmt.Errorf("expression could not be matched: unexpected expression type %T", expr)
	}

	return ps, nil
}
