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

package bcr

import (
	"testing"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/stretchr/testify/assert"
)

func TestShouldExcludeTarget(t *testing.T) {
	tests := []struct {
		name     string
		label    label.Label
		expected bool
	}{
		{"internal package", label.Label{Pkg: "internal/pkg"}, true},
		{"impl package", label.Label{Pkg: "impl/pkg"}, true},
		{"valid package", label.Label{Pkg: "pkg"}, false},
		{"valid package with subdir", label.Label{Pkg: "pkg/subdir"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldExcludeTarget(tt.label)
			assert.Equal(t, tt.expected, result)
		})
	}
}
