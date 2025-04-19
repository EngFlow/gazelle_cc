package bazel

import (
	"bytes"
	"log"
	"os"
	"os/exec"

	"github.com/EngFlow/gazelle_cc/index/internal/collections"
	"google.golang.org/protobuf/proto"
)

// Execute given bazel query inside directory. Returns nil if query fails
func Query(cwd string, query string) *QueryResult {
	var bufStdout bytes.Buffer
	var bufStderr bytes.Buffer
	cmd := exec.Command("bazel", "query", query, "--output=proto", "--incompatible_disallow_empty_glob=false")
	cmd.Dir = cwd
	cmd.Stdout = &bufStdout
	cmd.Stderr = os.Stderr // &bufStderr
	if err := cmd.Run(); err != nil {
		log.Printf("Bazel query failed for %s: %v. Stderr: %v", cmd.Args, err, bufStderr.String())
		return nil
	}
	var result QueryResult
	if err := proto.Unmarshal(bufStdout.Bytes(), &result); err != nil {
		log.Fatalf("Failed to unmarshal query result: %v", err)
	}
	return &result
}

// Select attribute that defined with given name. Returns nil if no such attribute can be found
func (target *Target) GetNamedAttribute(name string) *Attribute {
	found := collections.Find(target.GetRule().GetAttribute(), func(attr *Attribute) bool {
		return attr.GetName() == name
	})
	if found != nil {
		return *found
	}
	return nil
}
