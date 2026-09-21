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
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/label"
)

// Repositories describes the external repositories visible to the root module,
// keyed both ways so that labels reported by `bazel query` can be normalized
// back to the apparent names that may actually be written in a BUILD file.
type Repositories struct {
	// Apparent names, in the order reported by Bazel.
	Apparent []string
	// canonicalToApparent maps e.g. "+http_archive+au" to "au".
	canonicalToApparent map[string]string
}

// ResolveRepositories lists the repositories visible to the root module using
// `bazel mod dump_repo_mapping`.
//
// Anything that can be named as a dependency from the root module will appear
// in the result, including overrides (e.g. `single_version_override()`,
// `archive_override()`) and extensions (e.g. `http_archive()`).
func ResolveRepositories(workingDir string, include, exclude *regexp.Regexp) (Repositories, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("bazel", "mod", "dump_repo_mapping", "", "--noshow_progress")
	cmd.Dir = workingDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Repositories{}, fmt.Errorf("failed to dump repository mapping: %w\n%s", err, stderr.String())
	}

	var mapping map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &mapping); err != nil {
		return Repositories{}, fmt.Errorf("failed to parse repository mapping: %w", err)
	}

	repos := Repositories{canonicalToApparent: make(map[string]string, len(mapping))}
	for apparent, canonical := range mapping {
		// Skip the main repository, as that's the one that will actually use
		// this index.
		if apparent == "" || canonical == "" || canonical == "_main" {
			continue
		}
		if include != nil && !include.MatchString(apparent) {
			continue
		}
		if exclude != nil && exclude.MatchString(apparent) {
			continue
		}
		repos.Apparent = append(repos.Apparent, apparent)
		repos.canonicalToApparent[canonical] = apparent
	}
	slices.Sort(repos.Apparent)
	return repos, nil
}

// ParseLabel parses a label as reported by `bazel query`, rewriting canonical
// repository names (e.g. `@@rules_cc+//cc:foo`) into apparent ones (e.g.
// `@rules_cc//cc:foo`). Bazel normally prints labels using the root module's
// repository mapping already, but it is not guaranteed to for every repository.
//
// Fails (returns `_, false`) if the label cannot be depended upon, in which
// case it should be ignored.
func (r Repositories) ParseLabel(raw string) (label.Label, bool) {
	if canonical, rest, ok := splitCanonicalRepo(raw); ok {
		if apparent, known := r.canonicalToApparent[canonical]; known {
			raw = "@" + apparent + rest
		} else {
			// Not visible to the root module, so it cannot be depended upon.
			return label.NoLabel, false
		}
	}
	parsed, err := label.Parse(raw)
	if err != nil {
		return label.NoLabel, false
	}
	return parsed, true
}

// splitCanonicalRepo splits "@@canonical//pkg:name" into "canonical" and
// "//pkg:name". It reports false for labels that are not in canonical form.
func splitCanonicalRepo(raw string) (canonical, rest string, ok bool) {
	if !strings.HasPrefix(raw, "@@") {
		return "", "", false
	}
	trimmed := raw[len("@@"):]
	idx := strings.Index(trimmed, "//")
	if idx < 0 {
		return "", "", false
	}
	return trimmed[:idx], trimmed[idx:], true
}
