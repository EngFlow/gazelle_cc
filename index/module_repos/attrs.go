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

package main

import (
	"slices"

	"github.com/EngFlow/gazelle_cc/index/internal/bazel"
	"github.com/EngFlow/gazelle_cc/index/internal/bazel/proto"
	"github.com/bazelbuild/bazel-gazelle/label"
)

// Decoders for the attribute values reported by `bazel query --output=proto`,
// used by `targets.go`.
//
// Configurable attributes are unioned across all select() arms rather than
// resolved to one of them. `bazel query` runs the loading phase only, so it
// reports every arm (and by default, with --proto:flatten_selects, has already
// flattened them into a plain list). That union is what an index wants: a
// header reachable only in the aarch64 arm of a select() should still map to
// its target when the index is generated on x86_64. Resolving a configuration
// instead -- which is what cquery or an aspect would do -- would make the
// generated index platform-dependent.

// stringListAttr reads a list attribute, unioning select() arms.
func stringListAttr(target *proto.Target, name string) []string {
	attr := bazel.GetNamedAttribute(target, name)
	if attr == nil {
		return nil
	}
	if selectorList := attr.GetSelectorList(); selectorList != nil {
		var values []string
		for _, selector := range selectorList.GetElements() {
			for _, entry := range selector.GetEntries() {
				values = append(values, entry.GetStringListValue()...)
			}
		}
		return values
	}
	if values := attr.GetStringListValue(); len(values) > 0 {
		return slices.Clone(values)
	}
	// Some attributes are scalar in the proto but wanted with list semantics.
	if value := attr.GetStringValue(); value != "" {
		return []string{value}
	}
	return nil
}

// stringAttr reads a scalar attribute, resolving select() arms.
//
// A scalar cannot be unioned, so a select() is resolved to its default arm when
// there is one, and to the first non-empty arm otherwise.
func stringAttr(target *proto.Target, name string) (string, bool) {
	attr := bazel.GetNamedAttribute(target, name)
	if attr == nil {
		return "", false
	}
	if selectorList := attr.GetSelectorList(); selectorList != nil {
		var fallback string
		for _, selector := range selectorList.GetElements() {
			for _, entry := range selector.GetEntries() {
				value := entry.GetStringValue()
				if value == "" {
					continue
				}
				if entry.GetIsDefaultValue() {
					return value, true
				}
				if fallback == "" {
					fallback = value
				}
			}
		}
		return fallback, fallback != ""
	}
	value := attr.GetStringValue()
	return value, value != ""
}

// labelListAttr reads a label list attribute, unioning select() arms, parsing
// values as labels.
func (r repositories) labelListAttr(target *proto.Target, name string) []label.Label {
	values := stringListAttr(target, name)
	labels := make([]label.Label, 0, len(values))
	for _, value := range values {
		if parsed, ok := r.parseLabel(value); ok {
			labels = append(labels, parsed)
		}
	}
	return labels
}

// labelAttr reads a scalar attribute, resolving select() arms, and parsing
// values as labels.
func (r repositories) labelAttr(target *proto.Target, name string) (label.Label, bool) {
	value, ok := stringAttr(target, name)
	if !ok {
		return label.NoLabel, false
	}
	return r.parseLabel(value)
}
