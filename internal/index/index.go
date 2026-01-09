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
	"encoding"
	"encoding/json"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/label"
)

type (
	// labelUnmarshaler is a wrapper around label.Label that is parsable from
	// JSON text.
	labelUnmarshaler label.Label

	// DependencyIndex maps C/C++ header include paths to the Bazel targets
	// (more than one in case of ambiguity). Serializable to/from JSON.
	DependencyIndex map[string][]label.Label
)

var (
	_ encoding.TextUnmarshaler = (*labelUnmarshaler)(nil)
	_ json.Marshaler           = (*DependencyIndex)(nil)
	_ json.Unmarshaler         = (*DependencyIndex)(nil)
)

func (lm *labelUnmarshaler) UnmarshalText(data []byte) error {
	parsedLabel, err := label.Parse(string(data))
	*lm = labelUnmarshaler(parsedLabel)
	return err
}

func (index DependencyIndex) MarshalJSON() ([]byte, error) {
	jsonDict := make(map[string][]string, len(index))
	for header, labels := range index {
		jsonDict[header] = collections.MapSlice(labels, func(lbl label.Label) string { return lbl.String() })
	}
	return json.Marshal(jsonDict)
}

func (index *DependencyIndex) UnmarshalJSON(data []byte) error {
	var jsonDict map[string][]labelUnmarshaler
	if err := json.Unmarshal(data, &jsonDict); err != nil {
		return err
	}

	*index = make(DependencyIndex, len(jsonDict))
	for header, labels := range jsonDict {
		(*index)[header] = collections.MapSlice(labels, func(lbl labelUnmarshaler) label.Label { return label.Label(lbl) })
	}
	return nil
}
