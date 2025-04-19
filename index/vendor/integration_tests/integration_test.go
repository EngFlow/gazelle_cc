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

package test

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/EngFlow/gazelle_cc/index/internal/tests"
)

func TestRulesForeignCCIndexerIntegration(t *testing.T) {
	testCasesDir := filepath.Join(".", "testcases")
	repositoryDir, exists := os.LookupEnv("REPOSITORY_ROOT")
	if !exists {
		t.Fatalf("Missing required env variable REPOSITORY_ROOT pointing to root directory of this bazel repository")
	}
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
			runTestCase(t, filepath.Join(testCasesDir, tcName), repositoryDir)
		})
	}
}

func runTestCase(t *testing.T, readOnlyTestDir, repositoryDir string) {
	testDir, err := os.MkdirTemp(os.TempDir(), "test"+filepath.Base(readOnlyTestDir))
	if err != nil {
		t.Fatalf("Failed to create tmp dir")
	}
	tests.CopyDir(readOnlyTestDir, testDir)
	log.Printf("testDir: %v", testDir)

	if err := tests.ReplaceAllInFile(filepath.Join(testDir, "MODULE.bazel"), map[string]string{
		"<GAZELLE_CC_REPO>": repositoryDir,
	}); err != nil {
		t.Fatalf("Failed to prepare module file: %v", err)
	}

	indexPath := filepath.Join(testDir, "generated.ccindex")
	expectedIndexPath := filepath.Join(testDir, "expected.ccindex")

	t.Logf("==> [%s] Running indexer...", testDir)
	bazelConfig := tests.ExecConfig{Dir: testDir}
	bazelOutputBase, err := os.MkdirTemp(os.TempDir(), "bazel-outputs"+filepath.Base(readOnlyTestDir))
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	tests.Execute(bazelConfig, t, "bazel", "--output_base="+bazelOutputBase,
		"run", "@gazelle_cc//index/vendor",
		"--", "--verbose", "--select=//third_party/...", "--output="+indexPath, testDir)

	t.Logf("==> [%s] Checking index file...", testDir)
	expectedIndex, _ := os.ReadFile(expectedIndexPath)
	actualIndex, _ := os.ReadFile(indexPath)
	if !tests.JsonEqual(expectedIndex, actualIndex) {
		t.Errorf("index.json doesn't match expected")
	}

	t.Logf("==> [%s] Running gazelle...", testDir)
	tests.Execute(bazelConfig, t, "bazel", "--output_base="+bazelOutputBase,
		"run", "//:gazelle")

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
	tests.Execute(bazelConfig, t, "bazel", "--output_base="+bazelOutputBase,
		"build", "//...",
		"--incompatible_disallow_empty_glob=false")
}
