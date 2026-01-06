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

// Package index defines serializable data structures for representing the
// mapping of C/C++ header include paths to Bazel targets defining them. They
// serve as a protocol for exchanging data between an indexer and gazelle_cc.
package index

import (
	"cmp"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"slices"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/label"
)

type (
	// Label serializable to JSON/Text formats
	Label struct{ label.Label }

	// Unambiguous mapping of header include path to Bazel target defining it
	UniqueDependencyIndex map[string]Label

	// List of at least 2 Bazel targets defining the same header include path
	AmbiguousTargets []Label

	// Ambiguous mapping of header include path to multiple Bazel targets
	// defining it
	AmbiguousDependencyIndex map[string]AmbiguousTargets

	// Full index of both unambiguous and ambiguous resolved dependencies
	FullDependencyIndex struct {
		// Headers assigned to exactly 1 target in 1 module
		Unique UniqueDependencyIndex `json:"unique"`
		// Headers assigned to multiple targets but still within the same module
		AmbiguousWithinModule AmbiguousDependencyIndex `json:"ambiguous_within_module"`
		// Headers assigned to multiple targets across different modules
		AmbiguousAcrossModules AmbiguousDependencyIndex `json:"ambiguous_across_modules"`
	}
)

// Implements encoding.TextMarshaler interpreted by json.Marshal
func (l Label) MarshalText() ([]byte, error) {
	return []byte(l.String()), nil
}

// Implements encoding.TextUnmarshaler interpreted by json.Unmarshal
func (l *Label) UnmarshalText(text []byte) error {
	decoded, err := label.Parse(string(text))
	*l = Label{Label: decoded}
	return err
}

func ParseUniqueDependencyIndex(data []byte) (UniqueDependencyIndex, error) {
	var index UniqueDependencyIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return index, err
	}
	return index, nil
}

func compareLabels(a, b Label) int {
	return cmp.Compare(a.String(), b.String())
}

func (targets AmbiguousTargets) Validate(expectSameRepo bool) error {
	if len(targets) < 2 {
		return fmt.Errorf("ambiguous targets must contain at least 2 elements, got %d", len(targets))
	}
	if duplicates := collections.FindDuplicatesInSlice(targets); len(duplicates) > 0 {
		return fmt.Errorf("duplicate targets in list %v: %v", targets, duplicates.SortedValues(compareLabels))
	}
	repos := collections.CollectToSet(collections.MapSeq(slices.Values(targets), func(l Label) string { return l.Repo }))
	if expectSameRepo && len(repos) > 1 {
		return fmt.Errorf("should share same repo in list %v", targets)
	}
	if !expectSameRepo && len(repos) == 1 {
		return fmt.Errorf("should span multiple repos in list %v", targets)
	}
	return nil
}

func (index AmbiguousDependencyIndex) Validate(expectSameRepo bool) error {
	for _, targets := range index {
		if err := targets.Validate(expectSameRepo); err != nil {
			return err
		}
	}
	return nil
}

func (index FullDependencyIndex) Validate() error {
	allHeaders := collections.ConcatSeq(maps.Keys(index.Unique), maps.Keys(index.AmbiguousWithinModule), maps.Keys(index.AmbiguousAcrossModules))
	if headersDuplicates := collections.FindDuplicatesInSeq(allHeaders); len(headersDuplicates) > 0 {
		return fmt.Errorf("header present in multiple sections: %v", headersDuplicates.SortedValues(cmp.Compare[string]))
	}
	if err := index.AmbiguousWithinModule.Validate(true); err != nil {
		return err
	}
	if err := index.AmbiguousAcrossModules.Validate(false); err != nil {
		return err
	}
	return nil
}

func ParseFullDependencyIndex(data []byte) (FullDependencyIndex, error) {
	var index FullDependencyIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return index, err
	}
	if err := index.Validate(); err != nil {
		return index, err
	}
	return index, nil
}

func (index FullDependencyIndex) Encode() []byte {
	result, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		log.Panicf("failed to encode FullDependencyIndex: %v", err)
	}
	return result
}
