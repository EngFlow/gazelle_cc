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

	"github.com/EngFlow/gazelle_cc/index/internal/collections"
	"github.com/EngFlow/gazelle_cc/language/internal/cc/parser"
	"github.com/EngFlow/gazelle_cc/language/internal/cc/platform"
)

// PlatformsForExpr returns the list of Bazel platforms for which the C/C++ pre-processor expression `e` evaluates to true.
//
// Parameters:
//
//	e               – An expression produced by the cc/parser package. A nil value means “no condition”, i.e. the expression is implicitly true for all platforms.
//	platformMacros  – For every known platform, the map of macros (and their values) that are assumed to be defined when compiling for that platform.
//
// Semantics of the return value:
//   - If e is nil, the function returns nil to signal a generic include, i.e. the file/target is used by every platform.
//   - If no enabled platform matches, the function returns an empty slice – in Bazel terms the caller would typically attach the file to `//conditions:default`.
//   - Otherwise the function returns the matching platforms in deterministic order as defined by platform.ComparePlatform.
func PlatformsForExpr(e parser.Expr, platformMacros map[platform.Platform]platform.Macros) []platform.Platform {
	// A nil expression means the given expression applies to all platforms.
	if e == nil {
		return nil
	}
	// Convert the expression tree to disjunctive normal form (DNF) exactly once.
	// From here on we work with conjunctions of `macroTest` literals.
	dnf := toDNF(e)
	// Start with the set of currently enabled platforms. Calculated once
	enabledPlatforms := slices.Collect(maps.Keys(platformMacros))

	// The set of platforms that satisfy any of the conjunctions in the DNF.
	matched := collections.Set[platform.Platform]{}

	// Evaluate each conjunction separately and union the result.
	for _, conjunct := range dnf {
		// start with full universe for this term
		termSet := collections.ToSet(enabledPlatforms)
		for _, lit := range conjunct {
			if lit.Comparsion != nil {
				// -- Slow path
				// Generic comparisons (e.g. "__GNUC__ >= 9") cannot be solved by simple set operations;
				// we have to evaluate them for every remaining platform
				filtered := collections.Set[platform.Platform]{}
				for p := range termSet {
					if lit.Comparsion.Eval(platformMacros[p]) == !lit.Negated {
						filtered.Add(p)
					}
				}
				termSet = filtered
				continue
			}

			// -- Fast path
			// Presence/absence of individual macros can be answered by a set intersection (macro defined) or difference (macro not defined).
			macroSet := platformsForMacro(lit.Macro, platformMacros)
			if lit.Negated {
				termSet = termSet.Diff(macroSet)
			} else {
				termSet = termSet.Intersect(macroSet)
			}

			// Early exit: an empty set can never be revived by further literals in the same conjunction.
			if len(termSet) == 0 {
				break
			}
		}
		matched.Join(termSet)
	}

	result := matched.Values()
	// nil means generic include - used by all platforms
	// empty slice means non of enabled platforms match, so would be added to //conditions:default
	if result == nil {
		// Explicitlly reasign value - at this point result should never be nil
		result = []platform.Platform{}
	}
	slices.SortFunc(result, platform.ComparePlatform)
	return result
}

func platformsForMacro(macro string, platformMacros map[platform.Platform]platform.Macros) collections.Set[platform.Platform] {
	platforms := collections.Set[platform.Platform]{}
	for platform, macros := range platformMacros {
		if _, exists := macros[macro]; exists {
			platforms.Add(platform)
		}
	}
	return platforms
}

type (
	// ─────────────────────────────────────────────────────────────────────────────
	// Expression normalisation helpers
	// ─────────────────────────────────────────────────────────────────────────────

	// A macroTest represents a single literal in DNF:
	//   MACRO            -> Macro="MACRO", Negated=false, Comparsion=nil
	//   !MACRO           -> Macro="MACRO", Negated=true,  Comparsion=nil
	//   __GNUC__ >= 9    -> Macro="",      Negated=false, Comparsion=⟦expr⟧
	//
	// For speed the literal carries an extracted macro name when possible so that
	// simple presence tests can be answered with set operations (fast path).
	// Generic comparisons fall back to per‑platform evaluation (slow path).
	macroTest struct {
		Macro      string
		Negated    bool
		Comparsion *parser.Compare // nil for simple presence/absence literals
	}
	// andGroup is a conjunction (logical AND) of literals (macroTest)
	andGroup []macroTest
	// dnf is a disjunction (logical OR) of 'andGroup's – the full DNF.
	dnf []andGroup
)

// toDNF converts the parser.Expr tree into minimal DNF where negation occurs
// only on literals (¬p) using De‑Morgan rules; it does this once, so later
// code never needs to re‑walk the AST.
func toDNF(e parser.Expr) dnf {
	// Step 1: push negations down so we reach NNF (negation normal form)
	normalizedExpr := toNegationNormalForm(e)
	// Step 2: recursively distribute AND over OR to get full DNF
	return exprToDnf(normalizedExpr)
}

// toNegationNormalForm pushes logical NOT operators inward so that they wrap only atomic literals (parser.Defined or bare identifiers).
//
//	!!A        -> A
//	!(A && B)  -> !A || !B
//	!(A || B)  -> !A && !B
func toNegationNormalForm(e parser.Expr) parser.Expr {
	switch n := e.(type) {
	case parser.Not:
		inner := n.X
		switch v := inner.(type) {
		case parser.Not:
			return toNegationNormalForm(v.X) // !!A → A
		case parser.And:
			return parser.Or{L: toNegationNormalForm(parser.Not{X: v.L}), R: toNegationNormalForm(parser.Not{X: v.R})} // !(A&&B) → !A||!B
		case parser.Or:
			return parser.And{L: toNegationNormalForm(parser.Not{X: v.L}), R: toNegationNormalForm(parser.Not{X: v.R})} // !(A||B) → !A&&!B
		default: // Defined or bare ident
			return parser.Not{X: toNegationNormalForm(inner)}
		}
	case parser.And:
		return parser.And{L: toNegationNormalForm(n.L), R: toNegationNormalForm(n.R)}
	case parser.Or:
		return parser.Or{L: toNegationNormalForm(n.L), R: toNegationNormalForm(n.R)}
	default:
		return e // literal
	}
}

// exprToDnf recursively converts an expression already in NNF to full DNF by
// applying the distributive law:
//
//	(l1 || l2) && (r1 || r2)  ->  l1&&r1 || l1&&r2 || l2&&r1 || l2&&r2
func exprToDnf(e parser.Expr) dnf {
	switch n := e.(type) {
	case parser.And:
		left := exprToDnf(n.L)
		right := exprToDnf(n.R)
		// distributive law: (l1||l2) && (r1||r2) = l1&&r1 || l1&&r2 || l2&&r1 || l2&&r2
		var out dnf
		for _, lt := range left {
			for _, rt := range right {
				combined := make(andGroup, 0, len(lt)+len(rt))
				combined = append(combined, lt...)
				combined = append(combined, rt...)
				out = append(out, combined)
			}
		}
		return out

	case parser.Or:
		d := exprToDnf(n.L)
		return append(d, exprToDnf(n.R)...)

	case parser.Not:
		name, _ := extractMacro(n.X) // guaranteed literal after nnf
		switch x := n.X.(type) {
		case parser.Compare:
			negated := x.Negate()
			return dnf{{{Comparsion: &negated}}}
		default:
			return dnf{{{Macro: name, Negated: true}}}
		}

	case parser.Compare:
		// Generic comparison must be evaluated per-platform later.
		return dnf{{{Comparsion: &n}}}

	default:
		name, _ := extractMacro(n)
		return dnf{{{Macro: name, Negated: false}}}
	}
}

// extractMacro attempts to extract the 'macro' name referenced by the literal expression 'e'.
// It returns (name, true) when successful, otherwise ("", false).
// The function understands two cases:
//  1. Simple defined‑tests:   #if defined(FOO)          -> "FOO"
//  2. Comparisons involving a single macro on exactly one side:
//     #if __GNUC__ >= 9        → "__GNUC__"
//
// A literal that involves two different macros (e.g. "A == B") does not yield a single name and therfore cannot be extracted
func extractMacro(e parser.Expr) (string, bool) {
	switch v := e.(type) {
	case parser.Defined:
		return string(v.Name), true
	case parser.Compare:
		lName, lDefined := extractValueMacro(v.Left)
		rName, rDefined := extractValueMacro(v.Right)
		switch {
		case lDefined && !rDefined:
			return lName, true
		case !lDefined && rDefined:
			return rName, true
		case lDefined && rDefined && lName == rName:
			return lName, true
		default:
			return "", false
		}
	default:
		log.Panicf("unknown in extract macro expr on %+v", e)
		return "", false
	}
}

// extractValueMacro is the helper for extractMacro that inspects a parser.Value (either Ident or Constant)
// returns (macroName, true) when the value is an identifier.
func extractValueMacro(e parser.Value) (string, bool) {
	switch v := e.(type) {
	case parser.Ident:
		return string(v), true
	case parser.Constant:
		return "", false
	default:
		log.Panicf("unknown case extract value macro on %+v", e)
		return "", false
	}
}
