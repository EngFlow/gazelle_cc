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

// Defines a framework to be used when testing indexer
package tests

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	gazelleBinaryPath = flag.String("gazelle_binary_path", "", "rlocationpath to the gazelle binary to test.")
	indexerBinaryPath = flag.String("indexer_binary_path", "", "rlocationpath to the cc indexer binary to test.")
)

type IndexerIntegrationContext struct {
	// Root directory of test case bazel repository
	Dir string
}
type IndexerIntegration struct {
	// Function to exuecute before each test case, typically integration specific preperation logc
	BeforeTestCase func(t *testing.T, ctx IndexerIntegrationContext)
}

// Entry point for integration tests, needs to be pointed by at least 1 of `indexer_integration_test` rules src
func ExecuteIndexerIntegrationTest(t *testing.T, integration IndexerIntegration) {
	relativeGazelleBinary, err := runfiles.Rlocation(*gazelleBinaryPath)
	if err != nil {
		t.Fatalf("Failed to find gazelle binary %s. Error: %v", *gazelleBinaryPath, err)
	}
	absoluteGazelleBinary, err := filepath.Abs(relativeGazelleBinary)

	relativeIndexerPath, err := runfiles.Rlocation(*indexerBinaryPath)
	if err != nil {
		t.Fatalf("Failed to find indexer binary %s. Error: %v", *gazelleBinaryPath, err)
	}
	absoluteIndexerPath, err := filepath.Abs(relativeIndexerPath)

	testCasesDir := filepath.Join(".", "testcases")
	entries, err := os.ReadDir(testCasesDir)
	if err != nil {
		t.Fatalf("failed to read test dir: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		tcName := entry.Name()
		t.Run(tcName, func(t *testing.T) {
			executeTestCase(t, integration, absoluteGazelleBinary, absoluteIndexerPath, filepath.Join(testCasesDir, tcName))
		})
	}
}

func executeTestCase(
	t *testing.T,
	integration IndexerIntegration,
	gazelleBinary, indexerBinary, readOnlyTestDir string,
) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test"+filepath.Base(readOnlyTestDir))
	if err != nil {
		t.Fatalf("Failed to create tmp dir")
	}
	CopyDir(readOnlyTestDir, testDir)

	// Execute indexer specific setup
	if integration.BeforeTestCase != nil {
		integration.BeforeTestCase(t, IndexerIntegrationContext{Dir: testDir})
	}

	indexPath := filepath.Join(testDir, "generated.ccindex")
	expectedIndexPath := filepath.Join(testDir, "expected.ccindex")

	t.Logf("==> [%s] Running indexer...", testDir)
	defaultExecConfig := ExecConfig{Dir: testDir}

	Execute(t, defaultExecConfig, indexerBinary, "--verbose", "--output="+indexPath, "--repository="+testDir)

	t.Logf("==> [%s] Checking index file...", testDir)
	expectedIndex, _ := os.ReadFile(expectedIndexPath)
	actualIndex, _ := os.ReadFile(indexPath)
	AssertJsonEqual(t, expectedIndex, actualIndex)

	t.Logf("==> [%s] Running gazelle...", testDir)
	Execute(t, defaultExecConfig, gazelleBinary)

	t.Logf("==> [%s] Validating generated BUILD.bazel", testDir)
	err = filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && filepath.Base(path) == "BUILD.expected" {
			dir := filepath.Dir(path)
			buildPath := filepath.Join(dir, "BUILD")
			if _, err := os.Stat(buildPath); os.IsNotExist(err) {
				t.Errorf("Missing BUILD file: %v", buildPath)
			} else if err != nil {
				return err // propagate errors
			}
			expected, _ := os.ReadFile(path)
			actual, _ := os.ReadFile(buildPath)
			if !bytes.Equal(bytes.TrimSpace(expected), bytes.TrimSpace(actual)) {
				t.Errorf("BUILD.bazel doesn't match expected.\nExpected:\n%s\nActual:\n%s", expected, actual)
			}
		}
		return nil
	})
	if err != nil {
		t.Errorf("Error during walk: %v\n", err)
	}

	t.Logf("==> [%s] Building project with bazel...", testDir)
	bazelOutputBase, err := os.MkdirTemp(os.TempDir(), "bazel-outputs"+filepath.Base(readOnlyTestDir))
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	Execute(t, defaultExecConfig, "bazel", "--output_base="+bazelOutputBase,
		"build", "//...",
		"--incompatible_disallow_empty_glob=false")
}
