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

package indexer

import (
	"log"
	"testing"

	"github.com/EngFlow/gazelle_cc/index/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/stretchr/testify/assert"
)

func TestIndexableIncludePaths(t *testing.T) {
	tests := []struct {
		name     string
		hdrPath  string
		target   Target
		expected []string
	}{
		{
			name:    "strip include prefix",
			hdrPath: "include/header.h",
			target: Target{
				StripIncludePrefix: "include",
			},
			expected: []string{"header.h", "include/header.h"},
		},
		{
			name:    "add include prefix",
			hdrPath: "header.h",
			target: Target{
				IncludePrefix: "include",
			},
			expected: []string{"header.h", "include/header.h"},
		},
		{
			name:    "multiple include paths",
			hdrPath: "include/subdir/header.h",
			target: Target{
				Includes: collections.SetOf("include", "include/subdir"),
			},
			expected: []string{"include/subdir/header.h", "subdir/header.h", "header.h"},
		},
		{
			name:    "use package path when no includes",
			hdrPath: "header.h",
			target: Target{
				Name: label.Label{Pkg: "pkg"},
			},
			expected: []string{"header.h", "pkg/header.h"},
		},
		{
			name:    "strip include prefix with package path",
			hdrPath: "include/header.h",
			target: Target{
				Name:               label.Label{Pkg: "pkg"},
				StripIncludePrefix: "include",
			},
			expected: []string{"pkg/include/header.h", "header.h"},
		},
		{
			name:    "multiple includes with package path",
			hdrPath: "include/subdir/header.h",
			target: Target{
				Name:     label.Label{Pkg: "pkg"},
				Includes: collections.SetOf("include", "include/subdir"),
			},
			expected: []string{"include/subdir/header.h", "pkg/include/subdir/header.h", "subdir/header.h", "header.h"},
		}, {
			name:    "includes dot allows raw header path",
			hdrPath: "subdir/header.h",
			target: Target{
				Name:     label.Label{Pkg: "pkg"},
				Includes: collections.SetOf("."),
			},
			expected: []string{"subdir/header.h", "pkg/subdir/header.h"},
		},
		{
			name:    "include prefix with includes and strip",
			hdrPath: "src/include/header.h",
			target: Target{
				Name:               label.Label{Pkg: "third_party/lib"},
				StripIncludePrefix: "src/include",
				IncludePrefix:      "libapi",
				Includes:           collections.SetOf("."),
			},
			expected: []string{
				"libapi/header.h",      // stripped and prefixed
				"src/include/header.h", // full path
				"third_party/lib/src/include/header.h",
			},
		},
		{
			name:    "strip_include_prefix with include_prefix",
			hdrPath: "include/foo/bar.h",
			target: Target{
				Name:               label.Label{Pkg: "third_party/mylib"},
				StripIncludePrefix: "include",
				IncludePrefix:      "mylib",
			},
			expected: []string{
				"mylib/foo/bar.h",
				"third_party/mylib/include/foo/bar.h",
			},
		},
		{
			name:    "deep includes with base file",
			hdrPath: "include/a/b/c/header.h",
			target: Target{
				Name:     label.Label{Pkg: "dep"},
				Includes: collections.SetOf("include", "include/a", "include/a/b", "include/a/b/c"),
			},
			expected: []string{
				"include/a/b/c/header.h",
				"a/b/c/header.h",
				"b/c/header.h",
				"c/header.h",
				"header.h",
				"dep/include/a/b/c/header.h",
			},
		},
		{
			name:    "realistic mixed layout (lib3 with includes)",
			hdrPath: "include/header3.h",
			target: Target{
				Name:          label.Label{Pkg: "lib"},
				IncludePrefix: "mylib",
				Includes:      collections.SetOf(".", "include"),
			},
			expected: []string{
				"include/header3.h",       // from includes
				"header3.h",               // from includes = ["."]
				"mylib/include/header3.h", // prefixed
				"lib/include/header3.h",   // full
			},
		},
		{
			name:    "inner header",
			hdrPath: "include/inner/other.h",
			target: Target{
				Name:               label.Label{Pkg: "lib"},
				StripIncludePrefix: "include",
			},
			expected: []string{
				"inner/other.h",
				"lib/include/inner/other.h",
			},
		},
		{
			name:    "pkg root headers",
			hdrPath: "pkg1.h",
			target: Target{
				Name:     label.Label{Pkg: "lib/pkg"},
				Includes: collections.SetOf("."),
			},
			expected: []string{
				"pkg1.h",
				"lib/pkg/pkg1.h",
			},
		},
		{
			name:    "pkg subdir headers",
			hdrPath: "subdir/pkg3.h",
			target: Target{
				Name:     label.Label{Pkg: "lib/pkg"},
				Includes: collections.SetOf("."),
			},
			expected: []string{
				"subdir/pkg3.h",
				"lib/pkg/subdir/pkg3.h",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log.Printf("\ntest %v", tt.name)
			result := IndexableIncludePaths(tt.hdrPath, tt.target)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestShouldExcludeHeader(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"empty path", "", true},
		{"blank path", "   ", true},
		{"hidden file", ".header.h", true},
		{"hidden directory", "dir/.header.h", true},
		{"underscore prefix", "_header.h", true},
		{"valid path", "header.h", false},
		{"valid path with subdir", "dir/header.h", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldExcludeHeader(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

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

func TestCreateHeaderIndex(t *testing.T) {
	tests := []struct {
		name     string
		modules  []Module
		expected IndexingResult
	}{
		{
			name: "single module single target",
			modules: []Module{
				{
					Repository: "",
					Targets: []*Target{
						{
							Name: label.Label{Pkg: "pkg", Name: "lib"},
							Hdrs: collections.SetOf(label.Label{Pkg: "pkg", Name: "header.h"}),
						},
					},
				},
			},
			expected: IndexingResult{
				HeaderToRule: map[string]label.Label{
					"header.h":     {Pkg: "pkg", Name: "lib"},
					"pkg/header.h": {Pkg: "pkg", Name: "lib"},
				},
				Ambiguous: map[string][]label.Label{},
			},
		},
		{
			name: "ambiguous headers",
			modules: []Module{
				{
					Repository: "",
					Targets: []*Target{
						{
							Name:     label.Label{Pkg: "pkg1", Name: "lib1"},
							Hdrs:     collections.SetOf(label.Label{Pkg: "pkg1", Name: "common.h"}),
							Includes: collections.SetOf("."),
						},
						{
							Name:               label.Label{Pkg: "pkg2", Name: "lib2"},
							Hdrs:               collections.SetOf(label.Label{Pkg: "pkg2", Name: "common.h"}),
							StripIncludePrefix: "pkg2",
						},
					},
				},
			},
			expected: IndexingResult{
				HeaderToRule: map[string]label.Label{
					"pkg1/common.h": {Pkg: "pkg1", Name: "lib1"},
					"pkg2/common.h": {Pkg: "pkg2", Name: "lib2"},
				},
				Ambiguous: map[string][]label.Label{
					"common.h": {
						label.Label{Pkg: "pkg1", Name: "lib1"},
						label.Label{Pkg: "pkg2", Name: "lib2"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateHeaderIndex(tt.modules)
			assert.Equal(t, tt.expected, result)
		})
	}
}
