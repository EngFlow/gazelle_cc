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

package cc

import (
	"fmt"
	"slices"

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
// The two collections may appear in any order, and one or both of them may be
// omitted (all fields are nil for a nil expression).
type ccPlatformStringsExprs struct {
	genericDeps     *bzl.ListExpr // always active dependencies
	constrainedDeps *bzl.DictExpr // constrained dependencies

	// Explicitly encode an empty list when neither genericDeps nor
	// constrainedDeps are present. This is important, e.g., for cc_grpc_library
	// macro, which requires an explicit "deps" argument.
	explicitlyEmpty bool
}

func newCcPlatformStringsExprs(
	generic collections.Set[label.Label],
	constrainted map[label.Label]collections.Set[label.Label],
	explicitlyEmpty bool,
) ccPlatformStringsExprs {
	return ccPlatformStringsExprs{
		genericDeps:     labelsSetToListExpr(generic),
		constrainedDeps: labelsMapToDictExpr(constrainted),
		explicitlyEmpty: explicitlyEmpty,
	}
}

var _ rule.BzlExprValue = ccPlatformStringsExprs{}
var _ rule.Merger = ccPlatformStringsExprs{}

func labelsSetToStringSlice(labels collections.Set[label.Label]) []string {
	labelToString := func(l label.Label) string { return l.String() }
	return slices.Sorted(collections.MapSeq(labels.All(), labelToString))
}

func labelsSetToListExpr(labels collections.Set[label.Label]) *bzl.ListExpr {
	if len(labels) == 0 {
		return nil
	}
	return rule.SortedStrings(labelsSetToStringSlice(labels)).BzlExpr().(*bzl.ListExpr)
}

func labelsMapToDictExpr(labels map[label.Label]collections.Set[label.Label]) *bzl.DictExpr {
	if len(labels) == 0 {
		return nil
	}
	stringMap := make(map[string][]string, len(labels)+1)
	stringMap[selectDefaultKey] = nil // always include default condition
	for key, value := range labels {
		stringMap[key.String()] = labelsSetToStringSlice(value)
	}
	return rule.SelectStringListValue(stringMap).BzlExpr().(*bzl.CallExpr).List[0].(*bzl.DictExpr)
}

func (ps ccPlatformStringsExprs) makeSelectExpr() bzl.Expr {
	return &bzl.CallExpr{
		X:    &bzl.Ident{Name: selectFunctionName},
		List: []bzl.Expr{ps.constrainedDeps},
	}
}

func (ps ccPlatformStringsExprs) makeBinaryExpr() bzl.Expr {
	ps.genericDeps.ForceMultiLine = true
	ps.constrainedDeps.ForceMultiLine = true
	return &bzl.BinaryExpr{
		Op: "+",
		X:  ps.genericDeps,
		Y:  ps.makeSelectExpr(),
	}
}

func (ps ccPlatformStringsExprs) BzlExpr() bzl.Expr {
	switch {
	case ps.genericDeps != nil && ps.constrainedDeps != nil:
		return ps.makeBinaryExpr()
	case ps.genericDeps != nil:
		return ps.genericDeps
	case ps.constrainedDeps != nil:
		return ps.makeSelectExpr()
	case ps.explicitlyEmpty:
		return &bzl.ListExpr{}
	default:
		return nil
	}
}

func (ps ccPlatformStringsExprs) Merge(other bzl.Expr) bzl.Expr {
	otherPS, err := parseCcPlatformStringsExprs(other)
	if err != nil {
		return other // leave current BUILD content unchanged on error
	}

	ps.genericDeps = rule.MergeList(ps.genericDeps, otherPS.genericDeps)
	ps.constrainedDeps, err = rule.MergeDict(ps.constrainedDeps, otherPS.constrainedDeps)
	if err != nil {
		return other // leave current BUILD content unchanged on error
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

func parseCcPlatformStringsExprs(expr bzl.Expr) (ccPlatformStringsExprs, error) {
	var result ccPlatformStringsExprs
	if expr == nil {
		return result, nil
	}

	var parseGenericOrConstrained func(expr bzl.Expr) error
	parseGenericOrConstrained = func(expr bzl.Expr) error {
		switch expr := expr.(type) {
		case *bzl.ListExpr:
			if result.genericDeps != nil {
				return fmt.Errorf("expression could not be matched: unexpected [] + []")
			}
			result.genericDeps = expr
		case *bzl.CallExpr:
			dict, err := parseSelectExpr(expr)
			if err != nil {
				return err
			}
			if result.constrainedDeps != nil {
				return fmt.Errorf("expression could not be matched: unexpected select({}) + select({})")
			}
			result.constrainedDeps = dict
		case *bzl.BinaryExpr:
			if expr.Op != "+" {
				return fmt.Errorf("expression could not be matched: binary expression with unsupported operator %q", expr.Op)
			}
			if err := parseGenericOrConstrained(expr.X); err != nil {
				return err
			}
			if err := parseGenericOrConstrained(expr.Y); err != nil {
				return err
			}
		default:
			return fmt.Errorf("expression could not be matched: unexpected expression type %T", expr)
		}
		return nil
	}

	err := parseGenericOrConstrained(expr)
	return result, err
}
