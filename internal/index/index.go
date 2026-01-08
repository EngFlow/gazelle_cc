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
	"encoding/json"
	"fmt"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/label"
)

// DependencyIndex maps C/C++ header include paths to the Bazel targets (more
// than one in case of ambiguity). Serializable to/from JSON. Single label is
// represented in JSON as a string, multiple labels as a list of strings.
type DependencyIndex map[string][]label.Label

var _ json.Marshaler = (*DependencyIndex)(nil)
var _ json.Unmarshaler = (*DependencyIndex)(nil)

func (index DependencyIndex) MarshalJSON() ([]byte, error) {
	jsonDict := make(map[string]any, len(index))
	for header, labels := range index {
		if len(labels) == 1 {
			jsonDict[header] = labels[0].String()
		} else {
			jsonDict[header] = collections.MapSlice(labels, func(lbl label.Label) string { return lbl.String() })
		}
	}
	return json.Marshal(jsonDict)
}

func parseLabels(jsonDictValue any) ([]label.Label, error) {
	var jsonList []any
	switch value := jsonDictValue.(type) {
	case string:
		jsonList = []any{value}
	case []any:
		jsonList = value
	default:
		return nil, fmt.Errorf("invalid JSON type: %T", jsonDictValue)
	}

	labels := make([]label.Label, 0, len(jsonList))
	for _, jsonListValue := range jsonList {
		strValue, ok := jsonListValue.(string)
		if !ok {
			return nil, fmt.Errorf("invalid JSON type in list: %T", jsonListValue)
		}
		parsedLabel, err := label.Parse(strValue)
		if err != nil {
			return nil, err
		}
		labels = append(labels, parsedLabel)
	}
	return labels, nil
}

func (index *DependencyIndex) UnmarshalJSON(data []byte) error {
	var jsonDict map[string]any
	if err := json.Unmarshal(data, &jsonDict); err != nil {
		return err
	}

	result := make(DependencyIndex, len(jsonDict))
	for header, v := range jsonDict {
		labels, err := parseLabels(v)
		if err != nil {
			return fmt.Errorf("%q: %w", header, err)
		}
		result[header] = labels
	}

	*index = result
	return nil
}
