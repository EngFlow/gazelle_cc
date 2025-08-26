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
	"path/filepath"
	"slices"
	"strings"

	"github.com/EngFlow/gazelle_cc/internal/collections"
	"github.com/bazelbuild/bazel-gazelle/label"
)

// Indexes headers that have no known implementation as embedable by other rule
// It allows gazelle_cc to add additional dependencies when resolving headers that have implementation defined in different directory
func (c *ccLanguage) registerEmbedableHeaders(from label.Label, hdrs []sourceFile, srcs []sourceFile, conf ccConfig) {
	for _, conf := range conf.headerEmbedingConfigs {
		if !strings.HasPrefix(from.Pkg, conf.headersDir) {
			continue
		}
		for _, hdr := range hdrs {
			baseName := hdr.baseName()
			hasImpl := slices.ContainsFunc(srcs, func(src sourceFile) bool { return src.baseName() == baseName })
			if !hasImpl {
				relPath := strings.TrimPrefix(hdr.pathWithoutExt(), conf.headersDir)
				relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
				c.embedableHeaders[relPath] = from
			}
		}
	}
}

// Resolves targets of previously indexed embedable headers if srcs may contain their implementation
func (c *ccLanguage) resolveEmbedableHeaders(from label.Label, srcs []string) []label.Label {
	embeds := collections.SetOf[label.Label]()
	for _, conf := range c.headerEmbedingConfigs {
		if strings.HasPrefix(from.Pkg, conf.sourcesDir) {
			relPath := strings.TrimPrefix(from.Pkg, conf.sourcesDir)
			relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
			for _, src := range srcs {
				expectedPath := filepath.Join(relPath, src)
				expectedPath = strings.TrimSuffix(expectedPath, filepath.Ext(src))

				if label, exists := c.embedableHeaders[expectedPath]; exists {
					embeds.Add(label)
				}
			}
		}
	}
	return embeds.Values()
}
