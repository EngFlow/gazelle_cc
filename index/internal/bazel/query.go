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

// Package bazel provides functionality for interacting with Bazel build system,
// including query execution and result parsing.
package bazel

import (
	"bytes"
	"os/exec"
	"slices"

	"github.com/EngFlow/gazelle_cc/index/internal/bazel/proto"
	protobuf "google.golang.org/protobuf/proto"
)

func Query(cwd string, query string) (proto.QueryResult, error) {
	return ConfiguredQuery(cwd, query, QueryConfig{
		KeepGoing: false,
	})
}

type QueryConfig struct {
	KeepGoing bool
}

// Execute given bazel query inside directory. Returns nil if query fails
func ConfiguredQuery(cwd string, query string, opts QueryConfig) (proto.QueryResult, error) {
	var bufStdout bytes.Buffer
	var bufStderr bytes.Buffer
	args := []string{"query", query,
		"--output=proto",
		"--incompatible_disallow_empty_glob=false",
	}
	if opts.KeepGoing {
		args = append(args, "--keep_going")
	}
	cmd := exec.Command("bazel", args...)
	cmd.Dir = cwd
	cmd.Stdout = &bufStdout
	cmd.Stderr = &bufStderr
	if err := cmd.Run(); err != nil {
		if cmd.ProcessState.ExitCode() != 3 && !opts.KeepGoing {
			return proto.QueryResult{}, err
		}
	}

	var result proto.QueryResult
	if err := protobuf.Unmarshal(bufStdout.Bytes(), &result); err != nil {
		return proto.QueryResult{}, err
	}
	return result, nil
}

// Select attribute that defined with given name. Returns nil if no such attribute can be found
func GetNamedAttribute(target *proto.Target, name string) *proto.Attribute {
	attrs := target.GetRule().GetAttribute()
	idx := slices.IndexFunc(attrs, func(attr *proto.Attribute) bool {
		return attr.GetName() == name
	})
	if idx < 0 {
		return nil
	}
	return attrs[idx]
}
