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

// Common utilities for indexer CLIs
package cli

import (
	"flag"
	"log"
	"os"
	"path/filepath"
)

// Common flags available in all indexers, added as sideeffect of importing package
var (
	Verbose       = flag.Bool("verbose", false, "Enable verbose logging")
	output        = flag.String("output", "output.ccidx", "Output file path for index")
	repositoryDir = flag.String("repository", "", "Explicit path to bazel repository, if ommited BUILD_WORKSPACE_DIRECTORY env variable or current working directory is used")
)

// Resolve working directory for indexer, uses either explicit --repository path, BUILD_WORKSPACE_DIRECTORY env variable or current working directory
func ResolveWorkingDir() (string, error) {
	if !flag.Parsed() {
		log.Panicln("Flags not parsed yet")
	}
	dir := *repositoryDir
	if dir != "" {
		return dir, nil
	}

	dir = os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if dir != "" {
		return dir, nil
	}

	var err error
	if dir, err = os.Getwd(); err != nil {
		return "", err
	}
	return dir, nil
}

func ResolveOutputFile() string {
	if !flag.Parsed() {
		log.Panicln("Flags not parsed yet")
	}
	outputFile := *output
	if !filepath.IsAbs(outputFile) {
		if workdir, err := ResolveWorkingDir(); err != nil {
			outputFile = filepath.Join(workdir, outputFile)
		}
	}
	return outputFile
}
