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
)

type ExecConfig struct {
	Dir string
	Env []string
}

// Utility to execute commands
func Execute(config ExecConfig, t *testing.T, program string, args ...string) exec.Cmd {
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
		t.Errorf("Failed to execute %v: %v", cmd.Args, err)
	}
	return *cmd
}

func JsonEqual(a, b []byte) bool {
	var j1, j2 any
	return json.Unmarshal(a, &j1) == nil && json.Unmarshal(b, &j2) == nil && deepEqual(j1, j2)
}

func deepEqual(a, b any) bool {
	return strings.TrimSpace(fmtJSON(a)) == strings.TrimSpace(fmtJSON(b))
}

func fmtJSON(v any) string {
	out, _ := json.MarshalIndent(v, "", "  ")
	return string(out)
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
