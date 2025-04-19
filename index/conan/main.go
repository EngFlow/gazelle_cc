package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/EngFlow/gazelle_cc/index/internal/bazel"
	"github.com/EngFlow/gazelle_cc/index/internal/collections"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
	"github.com/bazelbuild/bazel-gazelle/label"
)

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	install := flag.Bool("install", false, "Should conan deps be installed before indexing")
	output := flag.String("output", "conan.ccidx", "Output file path for index")
	conanDir := flag.String("conan_dir", "conan", "Path to conan directory created after running `conan install`")
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		log.Fatalf("Program requires exactly 1 argument - a path to the caller project directory, typically $PWD. Flags needs to be defined before arguments")
	}
	callerRoot := flag.Arg(0)

	outputFile := *output
	if !filepath.IsAbs(outputFile) {
		outputFile = filepath.Join(callerRoot, outputFile)
	}

	conanDirectory := *conanDir
	if !filepath.IsAbs(conanDirectory) {
		conanDirectory = filepath.Join(callerRoot, conanDirectory)
	}

	if *install {
		for _, args := range [][]string{
			[]string{"profile", "detect"},
			[]string{"install", ".", "--build=missing"},
		} {
			cmd := exec.Command("conan", args...)
			cmd.Dir = callerRoot
			var buf bytes.Buffer
			if *verbose {
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			} else {
				cmd.Stdout = &buf
				cmd.Stderr = &buf
			}
			log.Printf("Exec %v in %v", cmd.Args, cmd.Dir)
			if cmd.Run() != nil {
				log.Println(buf.String())
				log.Fatalf("Failed to install conan dependenices")
			}
		}
	}

	subdirs, err := listSubdirectories(conanDirectory)
	if err != nil {
		log.Fatalf("Failed to list subdirectories in %s: %v", conanDirectory, err)
	}

	modules := []indexer.Module{}
	for _, dir := range subdirs {
		repoName := dir
		result := bazel.Query(callerRoot, fmt.Sprintf("kind(cc_library, @%s//...)", repoName))
		module := extractIndexerModule(result, repoName)

		// If multiple rules refer to the same headers (typicall in Conan integration) then
		// pick to targets that are on top of dependency chain - does not depend on other rules in group
		selectedTargets := []*indexer.ModuleTarget{}
		for _, intersectingTargets := range module.GroupTargetsByHeaders() {
			roots := indexer.SelectRootTargets(intersectingTargets)
			if len(roots) != 1 {
				log.Fatal("Incosistient state, should be only 1 root header")
			}
			root := roots[0]
			for target := range intersectingTargets {
				if target != root {
					root.Hdrs.Join(target.Hdrs)
					root.Includes.Join(target.Includes)
				}
			}
			selectedTargets = append(selectedTargets, root)
		}
		module.Targets = selectedTargets
		modules = append(modules, module)
	}

	indexingResult := indexer.CreateHeaderIndex(modules)
	indexingResult.WriteToFile(outputFile)

	if *verbose {
		indexingResult.Show()
	}

}

func extractIndexerModule(query *bazel.QueryResult, moduleName string) indexer.Module {
	targets := []*indexer.ModuleTarget{}
	for _, info := range query.GetTarget() {
		name, err := label.Parse(info.GetRule().GetName())
		if err != nil {
			log.Printf("Failed to parse queried target label: %v", info.GetRule().GetName())
			continue
		}

		target := &indexer.ModuleTarget{
			Name: name,
			Hdrs: collections.ToSet(collections.Collect(
				info.GetNamedAttribute("hdrs").GetStringListValue(),
				label.Parse)),
			Includes:           collections.ToSet(info.GetNamedAttribute("includes").GetStringListValue()),
			StripIncludePrefix: info.GetNamedAttribute("strip_include_prefix").GetStringValue(),
			IncludePrefix:      info.GetNamedAttribute("include_prefix").GetStringValue(),
			Deps: collections.ToSet(collections.Collect(
				info.GetNamedAttribute("deps").GetStringListValue(),
				label.Parse)),
		}
		targets = append(targets, target)
	}
	return indexer.Module{
		Repository: moduleName,
		Targets:    targets,
	}
}

func listSubdirectories(root string) ([]string, error) {
	var dirs []string
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}
