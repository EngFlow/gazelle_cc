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

package tests

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type ExecConfig struct {
	Dir string
	Env []string
}

// Utility to execute commands
func Execute(t *testing.T, config ExecConfig, program string, args ...string) exec.Cmd {
	t.Helper()
	cmd := exec.Command(program, args...)
	if config.Dir != "" {
		cmd.Dir = config.Dir
	}
	if config.Env != nil {
		cmd.Env = config.Env
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to execute %v: %v", cmd.Args, err)
	}
	return *cmd
}

func AssertJsonEqual(t *testing.T, jsonA, jsonB []byte) {
	var objA, objB any
	errA := json.Unmarshal(jsonA, &objA)
	errB := json.Unmarshal(jsonB, &objB)

	assert.NoError(t, errA)
	assert.NoError(t, errB)
	assert.Equal(t, objA, objB)
}

// copies all files recursively
func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

func ReplaceAllInFile(filePath string, replacements map[string]string) error {
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(contentBytes)
	for from, to := range replacements {
		content = strings.ReplaceAll(content, from, to)
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write modified file: %w", err)
	}

	return nil
}
