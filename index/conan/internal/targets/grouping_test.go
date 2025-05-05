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

package targets

import (
	"testing"

	"github.com/EngFlow/gazelle_cc/index/internal/collections"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/stretchr/testify/assert"
)

func TestGroupTargetsByHeaders(t *testing.T) {
	module := indexer.Module{
		Repository: "",
		Targets: []*indexer.Target{
			{
				Name: label.Label{Pkg: "pkg1", Name: "lib1"},
				Hdrs: collections.SetOf(
					label.Label{Pkg: "pkg1", Name: "header1.h"},
					label.Label{Pkg: "pkg1", Name: "header2.h"},
				),
			},
			{
				Name: label.Label{Pkg: "pkg2", Name: "lib2"},
				Hdrs: collections.SetOf(
					label.Label{Pkg: "pkg1", Name: "header2.h"},
					label.Label{Pkg: "pkg2", Name: "header3.h"},
				),
			},
			{
				Name: label.Label{Pkg: "pkg3", Name: "lib3"},
				Hdrs: collections.SetOf(
					label.Label{Pkg: "pkg1", Name: "header3.h"},
					label.Label{Pkg: "pkg2", Name: "header1.h"},
					label.Label{Pkg: "pkg3", Name: "header4.h"},
				),
			},
		},
	}

	groups := GroupTargetsByHeaders(module)
	assert.Equal(t, 2, len(groups))

	// First group should contain lib1 and lib2 (they share pkg1/header2.h)
	assert.Equal(t, 2, len(groups[0]))

	// Second group should contain lib3 (no shared headers)
	assert.Equal(t, 1, len(groups[1]))
	assert.Equal(t, "lib3", groups[1].Values()[0].Name.Name)
}

func TestSelectRootTargets(t *testing.T) {
	targets := collections.SetOf(
		&indexer.Target{
			Name: label.Label{Pkg: "pkg1", Name: "lib1"},
			Deps: collections.SetOf(label.Label{Pkg: "pkg2", Name: "lib2"}),
		},
		&indexer.Target{
			Name: label.Label{Pkg: "pkg2", Name: "lib2"},
			Deps: collections.SetOf[label.Label](),
		},
	)

	roots := SelectRootTargets(targets)
	assert.Equal(t, 1, len(roots))
	assert.Equal(t, "//pkg1:lib1", roots[0].Name.String())
}
