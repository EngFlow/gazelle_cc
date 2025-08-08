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

package main

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/ulikunitz/xz"

	coll "github.com/EngFlow/gazelle_cc/internal/collections"

	bzl "github.com/EngFlow/gazelle_cc/index/internal/bazel"
	qproto "github.com/EngFlow/gazelle_cc/index/internal/bazel/proto"
)

// Script responsible for creating index of external header include to the label of extern dependency defining it's rule.
// It does process all modules defined in https://registry.bazel.build/ and extracts information about defined header for public rules using bazel query.
// The mapping keys are always in the normalized form of include paths that should be valid when refering using #include directive in C/C++ sources assuming include paths were not overriden
// The values of the mapping is a string representation on Bazel label where the repository is the name of the module
// Mappings are always based on the last version of module available in the registry. If the latest available version is yanked then whole module would be skipped.
//
// The script needs to checkout (download) sources of each module and execute bazel query using a fresh instance of Bazel server.
// This step can be ignored if .cache/modules/ contains extracted module informations from previous run.
//
// When processing the results of the query script might exclude targets or headers that are assumed to be internal, the excluded files would be written in textual file on the disk.
// Mapping contains only headers that are assigned to exactly 1 rule. Header with ambigious rule definitions are also written in textual format for manual inspection.
// It does also use system binaries: git, patch (gpatch is required on MacOs instead to correctly apply patches to Bazel modules) and bazel (bazelisk preferred)
func main() {
	cfg := parseFlags()
	if err := NewApp(cfg).Run(); err != nil {
		die(err)
	}
}

// =====================================================================================
// Config & CLI
// =====================================================================================

type Config struct {
	cacheDir     string
	outputPath   string
	verbose      bool
	keepSources  bool
	recomputeBad bool
	cacheBad     bool
}

func parseFlags() Config {
	var cfg Config
	pwd, _ := os.Getwd()
	defaultCache := filepath.Join(pwd, ".cache")
	flag.StringVar(&cfg.cacheDir, "cache-dir", defaultCache, "Path to cache directory")
	flag.StringVar(&cfg.outputPath, "output-mappings", filepath.Join(defaultCache, "header-mappings.json"), "Output path for header mappings")
	flag.BoolVar(&cfg.verbose, "v", false, "Verbose")
	flag.BoolVar(&cfg.keepSources, "keep-sources", false, "Keep fetched sources (default false)")
	flag.BoolVar(&cfg.recomputeBad, "recompute-unresolved", false, "Recompute previously unresolved modules (default false)")
	flag.BoolVar(&cfg.cacheBad, "cache-unresolved", true, "Cache unresolved module results (default true)")
	flag.Parse()
	return cfg
}

// =====================================================================================
// App wiring
// =====================================================================================

type App struct {
	cfg        Config
	httpClient *http.Client
}

func NewApp(cfg Config) *App {
	// Generous timeout; individual operations may retry.
	tr := &http.Transport{
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxConnsPerHost:       8,
		MaxIdleConnsPerHost:   8,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   5 * time.Minute, // overall per request
	}
	return &App{cfg: cfg, httpClient: client}
}

func (a *App) Run() error {
	repo, err := a.checkoutRegistry()
	if err != nil {
		return err
	}

	infos, err := a.gatherModuleInfos(repo)
	if err != nil {
		return err
	}
	if a.cfg.verbose {
		showModuleInfos(infos)
	}

	m := a.createHeaderIndex(infos)
	fmt.Printf("Direct mapping created for %d headers\n", len(m.HeaderToRule))
	fmt.Printf("Ambigious header assignment for %d entries\n", len(m.Ambigious))
	var exclCount int
	for _, v := range m.Excluded {
		exclCount += len(v)
	}
	fmt.Printf("Excluded %d headers in %d targets\n", exclCount, len(m.Excluded))

	if err := writeJSON(a.cfg.outputPath, m.HeaderToRuleStrings()); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(a.cfg.cacheDir, "ambigious.json"), m.AmbigiousStrings()); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(a.cfg.cacheDir, "excluded.json"), m.ExcludedStrings()); err != nil {
		return err
	}
	return nil
}

// =====================================================================================
// Types
// =====================================================================================

type ModuleVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (m ModuleVersion) String() string { return fmt.Sprintf("%s @ %s", m.Name, m.Version) }

type ModuleTarget struct {
	Name             label.Label   `json:"name"`
	Alias            *label.Label  `json:"alias,omitempty"`
	Hdrs             []label.Label `json:"hdrs"`
	Includes         []string      `json:"includes"`
	StripIncludePref *string       `json:"strip_include_prefix,omitempty"`
	IncludePrefix    *string       `json:"include_prefix,omitempty"`
	Deps             []label.Label `json:"deps"`
}

type ModuleInfo struct {
	Module  ModuleVersion  `json:"module"`
	Targets []ModuleTarget `json:"targets"`
}

type ResolveResult struct {
	Info       *ModuleInfo `json:"info,omitempty"`
	Unresolved *struct {
		Module ModuleVersion `json:"module"`
		Reason string        `json:"reason"`
	} `json:"unresolved,omitempty"`
}

func (r ResolveResult) isUnresolved() bool { return r.Unresolved != nil }

func normalizeRelativePath(s string) string { return strings.TrimLeft(strings.TrimSpace(s), "/") }

type mappingOutput struct {
	HeaderToRule map[string]label.Label
	Ambigious    map[string][]label.Label
	Excluded     map[label.Label][]label.Label
}

func (out mappingOutput) HeaderToRuleStrings() map[string]string {
	o := make(map[string]string, len(out.HeaderToRule))
	for k, v := range out.HeaderToRule {
		o[k] = v.String()
	}
	return o
}
func (out mappingOutput) AmbigiousStrings() map[string][]string {
	o := make(map[string][]string, len(out.Ambigious))
	for k, v := range out.Ambigious {
		o[k] = coll.Map(v, func(elem label.Label) string { return elem.String() })
	}
	return o
}
func (out mappingOutput) ExcludedStrings() map[string][]string {
	o := make(map[string][]string, len(out.Excluded))
	for k, v := range out.Excluded {
		o[k.String()] = coll.Map(v, func(elem label.Label) string { return elem.String() })
	}
	return o
}

// =====================================================================================
// Registry checkout
// =====================================================================================

func (a *App) checkoutRegistry() (string, error) {
	repoDir := filepath.Join(a.cfg.cacheDir, "bazel-central-registry")
	if _, err := os.Stat(repoDir); err == nil {
		cmds := [][]string{
			{"git", "reset", "--hard"},
			{"git", "checkout", "main"},
			{"git", "fetch", "origin"},
			{"git", "reset", "--hard", "origin/main"},
		}
		for _, c := range cmds {
			cmd := exec.Command(c[0], c[1:]...)
			cmd.Dir = repoDir
			cmd.Stdout = io.Discard
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return "", fmt.Errorf("git refresh failed: %w", err)
			}
		}
		return repoDir, nil
	}

	if err := os.MkdirAll(a.cfg.cacheDir, 0o755); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "clone", "https://github.com/bazelbuild/bazel-central-registry", "--depth=1", repoDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed: %w", err)
	}
	return repoDir, nil
}

// =====================================================================================
// Module processing
// =====================================================================================

type metadataJSON struct {
	Repository     []string          `json:"repository"`
	Versions       []string          `json:"versions"`
	YankedVersions map[string]string `json:"yanked_versions"`
}

type sourceJSON struct {
	Type        string            `json:"type"` // "" => archive
	URL         string            `json:"url"`
	StripPrefix string            `json:"strip_prefix"`
	PatchStrip  int               `json:"patch_strip"`
	Patches     map[string]string `json:"patches"`
	Overlay     map[string]string `json:"overlay"`
	Remote      string            `json:"remote"` // git_repository (unsupported)
	Commit      string            `json:"commit"` // git_repository (unsupported)
}

func (a *App) processModule(modulePath string) ResolveResult {
	moduleName := filepath.Base(modulePath)
	metaPath := filepath.Join(modulePath, "metadata.json")
	b, err := os.ReadFile(metaPath)
	if err != nil {
		return unresolved(moduleName, "No metadata.json")
	}
	var meta metadataJSON
	if err := json.Unmarshal(b, &meta); err != nil || len(meta.Versions) == 0 {
		return unresolved(moduleName, "Invalid metadata.json")
	}
	latest := meta.Versions[len(meta.Versions)-1]
	mv := ModuleVersion{Name: moduleName, Version: latest}
	if _, yanked := meta.YankedVersions[latest]; yanked {
		return unresolvedMV(mv, "latest version is yanked - ignore")
	}

	cacheFile := filepath.Join(a.cfg.cacheDir, "modules", moduleName, latest, "module-info.json")
	if cached, err := tryLoadCached(cacheFile); err == nil {
		if cached.isUnresolved() && a.cfg.recomputeBad {
			// recompute
		} else {
			return cached
		}
	}

	sourcesDir := filepath.Join(modulePath, latest)
	srcRootDir, projectRoot, err := a.prepareModuleSources(sourcesDir)
	if err != nil {
		rr := unresolvedMV(mv, "Failed to prepare project sources: "+err.Error())
		saveMaybe(a.cfg, cacheFile, rr)
		return rr
	}

	targets, err := resolveTargets(projectRoot)
	if !a.cfg.keepSources {
		_ = os.RemoveAll(srcRootDir)
	}
	if err != nil {
		rr := unresolvedMV(mv, "Failed to resolve module targets: "+err.Error())
		saveMaybe(a.cfg, cacheFile, rr)
		return rr
	}

	info := ModuleInfo{Module: mv, Targets: targets}
	rr := ResolveResult{Info: &info}
	saveMaybe(a.cfg, cacheFile, rr)
	return rr
}

func unresolved(name, reason string) ResolveResult {
	return ResolveResult{Unresolved: &struct {
		Module ModuleVersion `json:"module"`
		Reason string        `json:"reason"`
	}{Module: ModuleVersion{Name: name, Version: "invalid"}, Reason: reason}}
}
func unresolvedMV(mv ModuleVersion, reason string) ResolveResult {
	return ResolveResult{Unresolved: &struct {
		Module ModuleVersion `json:"module"`
		Reason string        `json:"reason"`
	}{Module: mv, Reason: reason}}
}

func tryLoadCached(path string) (ResolveResult, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ResolveResult{}, err
	}
	var rr ResolveResult
	if err := json.Unmarshal(b, &rr); err != nil {
		return ResolveResult{}, err
	}
	return rr, nil
}

func saveMaybe(cfg Config, path string, rr ResolveResult) {
	if rr.isUnresolved() && !cfg.cacheBad {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, mustJSON(rr), 0o644)
}

// =====================================================================================
// Sources: download / extract / patch
// =====================================================================================

func (a *App) prepareModuleSources(moduleVersionDir string) (sourcesDir, projectRoot string, err error) {
	rel, err := filepath.Rel(filepath.Join(moduleVersionDir, "..", ".."), moduleVersionDir)
	if err != nil {
		return "", "", err
	}
	targetDir := filepath.Join(a.cfg.cacheDir, "modules", filepath.FromSlash(rel), "sources")
	_ = os.RemoveAll(targetDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", "", err
	}

	var src sourceJSON
	if err := json.Unmarshal(mustRead(filepath.Join(moduleVersionDir, "source.json")), &src); err != nil {
		return "", "", err
	}
	if src.Type == "git_repository" {
		return "", "", errors.New("git_repository modules not supported yet")
	}

	archivePath, err := a.downloadWithRetries(src.URL)
	if err != nil {
		return "", "", err
	}
	if err := extractArchive(archivePath, targetDir); err != nil {
		return "", "", err
	}
	_ = os.Remove(archivePath)
	root := targetDir
	if src.StripPrefix != "" {
		root = filepath.Join(targetDir, filepath.FromSlash(src.StripPrefix))
	}
	// Patches
	patchesDir := filepath.Join(moduleVersionDir, "patches")
	if st, err := os.Stat(patchesDir); err == nil && st.IsDir() && len(src.Patches) > 0 {
		for name := range src.Patches {
			patchFile := filepath.Join(patchesDir, name)
			patchBin := "patch"
			if isMacOS() {
				patchBin = "gpatch"
			}
			cmd := exec.Command(patchBin, fmt.Sprintf("-p%d", src.PatchStrip), "-f", "-l", "-i", patchFile)
			cmd.Dir = root
			cmd.Stdout = io.Discard
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return "", "", fmt.Errorf("applying patch %s failed: %w", name, err)
			}
		}
	}
	// Overlay
	overlayDir := filepath.Join(moduleVersionDir, "overlay")
	if st, err := os.Stat(overlayDir); err == nil && st.IsDir() {
		_ = filepath.WalkDir(overlayDir, func(p string, d os.DirEntry, e error) error {
			if e != nil || d.IsDir() {
				return e
			}
			rel, _ := filepath.Rel(overlayDir, p)
			dst := filepath.Join(root, rel)
			_ = os.MkdirAll(filepath.Dir(dst), 0o755)
			b := mustRead(p)
			return os.WriteFile(dst, b, 0o644)
		})
	}

	return targetDir, root, nil
}

func (a *App) downloadWithRetries(url string) (string, error) {
	tmpDir, _ := os.MkdirTemp("", "bcr-dl-")
	name := filepath.Base(strings.Split(url, "?")[0])
	dst := filepath.Join(tmpDir, name)

	var last error
	const maxAttempts = 3

	isTimeoutErr := func(err error) bool {
		return err != nil && (os.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded))
	}
	isTimeoutStatus := func(code int) bool {
		return code == http.StatusRequestTimeout || code == http.StatusGatewayTimeout // 408 / 504
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resp, err := a.httpClient.Get(url)
		if err != nil {
			// Do NOT retry on timeouts
			if isTimeoutErr(err) {
				return "", fmt.Errorf("download aborted due to timeout: %w", err)
			}
			last = err
			// retry (non-timeout failure)
			time.Sleep(time.Duration(rand.Intn(15000)) * time.Millisecond)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			// Treat timeout-like HTTP statuses as non-retryable
			if isTimeoutStatus(resp.StatusCode) {
				resp.Body.Close()
				return "", fmt.Errorf("download aborted due to server timeout (HTTP %d)", resp.StatusCode)
			}
			last = fmt.Errorf("http %d", resp.StatusCode)
			resp.Body.Close()
			// retry (non-timeout HTTP error)
			time.Sleep(time.Duration(rand.Intn(15000)) * time.Millisecond)
			continue
		}

		f, err := os.Create(dst)
		if err != nil {
			resp.Body.Close()
			return "", err
		}

		_, err = io.Copy(f, resp.Body)
		resp.Body.Close()
		f.Close()
		if err != nil {
			// Fail fast on read timeouts too
			if isTimeoutErr(err) {
				return "", fmt.Errorf("download aborted due to timeout while reading body: %w", err)
			}
			last = err
			// retry (non-timeout copy failure)
			time.Sleep(time.Duration(rand.Intn(15000)) * time.Millisecond)
			continue
		}

		return dst, nil
	}

	return "", fmt.Errorf("download failed after retries: %w", last)
}

// =====================================================================================
// Archive extraction helpers
// =====================================================================================

func extractArchive(archivePath, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	name := strings.ToLower(filepath.Base(archivePath))
	switch {
	case strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz"):
		return withFile(archivePath, func(f *os.File) error {
			gzr, err := gzip.NewReader(f)
			if err != nil {
				return err
			}
			defer gzr.Close()
			return untar(gzr, outDir)
		})
	case strings.HasSuffix(name, ".tar.xz"):
		return withFile(archivePath, func(f *os.File) error {
			xzr, err := xz.NewReader(f)
			if err != nil {
				return err
			}
			return untar(xzr, outDir)
		})
	case strings.HasSuffix(name, ".tar.bz2"):
		return withFile(archivePath, func(f *os.File) error {
			bz := bzip2.NewReader(f)
			return untar(bz, outDir)
		})
	case strings.HasSuffix(name, ".tar"):
		return withFile(archivePath, func(f *os.File) error { return untar(f, outDir) })
	case strings.HasSuffix(name, ".zip"):
		return unzip(archivePath, outDir)
	default:
		return fmt.Errorf("unsupported archive: %s", name)
	}
}

func withFile(path string, fn func(*os.File) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return fn(f)
}

func untar(r io.Reader, outDir string) error {
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		dst := filepath.Join(outDir, filepath.FromSlash(h.Name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			f, err := os.Create(dst)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			_ = f.Close()
		}
	}
	return nil
}

func unzip(zipPath, outDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		dst := filepath.Join(outDir, filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(dst)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(w, rc)
		rc.Close()
		w.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// =====================================================================================
// Bazel query via protobuf
// =====================================================================================

// resolveTargets runs a single protobuf-based bazel query and converts it into ModuleTarget[].
// Mirrors the XML path logic (aliases, filegroups, expand_template, public cc_*library).
func resolveTargets(projectRoot string) ([]ModuleTarget, error) {
	// Single query composing the same selector set as before.
	query := `(kind("cc_.*library|alias", //...) intersect attr(visibility, //visibility:public, //...)) union kind("expand_template|filegroup", //...)`

	result, err := bzl.ConfiguredQuery(projectRoot, query, bzl.QueryConfig{KeepGoing: true})
	if err != nil {
		return nil, err
	}

	// First pass: build maps for helpers.
	aliases := map[label.Label]label.Label{}         // cc_library -> alias
	filegroups := map[label.Label][]label.Label{}    // filegroup target -> expanded labels
	expandTemplates := map[label.Label]label.Label{} // expand_template target -> out

	// Quick index targets by name for later if needed.
	for i := range result.Target {
		t := result.Target[i]
		rule := t.GetRule()
		if rule == nil {
			continue
		}
		name, ok := parseLabel(rule.GetName())
		if !ok {
			continue
		}
		switch rule.GetRuleClass() {
		case "alias":
			if actual, ok := getLabelAttr(t, "actual"); ok {
				aliases[actual] = name
			}
		case "filegroup":
			if srcs := getLabelListAttr(t, "srcs"); len(srcs) > 0 {
				// relativize each src to this filegroup's package
				for i := range srcs {
					srcs[i] = srcs[i].Rel(name.Repo, name.Pkg)
				}
				filegroups[name] = srcs
			}
		case "expand_template":
			// out is typed as "output" in Starlark. Handle as string/output interchangeably.
			if out, ok := getOutputOrStringAttr(t, "out"); ok {
				expandTemplates[name] = out.Rel(name.Repo, name.Pkg)
			}
		}
	}

	// Second pass: accumulate cc_*library-like rules.
	var targets []ModuleTarget
	for i := range result.Target {
		t := result.Target[i]
		rule := t.GetRule()
		if rule == nil {
			continue
		}
		class := rule.GetRuleClass()
		if class == "alias" || class == "filegroup" || class == "expand_template" {
			continue
		}
		nameLb, ok := parseLabel(rule.GetName())
		if !ok {
			continue
		}
		// hdrs: labels + expand filegroups/expand_template
		var hdrs []label.Label
		for _, s := range getLabelListAttr(t, "hdrs") {
			if fg, ok := filegroups[s]; ok {
				for _, f := range fg {
					hdrs = append(hdrs, f.Rel(nameLb.Repo, nameLb.Pkg))
				}
				continue
			}
			if out, ok := expandTemplates[s]; ok {
				hdrs = append(hdrs, out.Rel(nameLb.Repo, nameLb.Pkg))
				continue
			}
			hdrs = append(hdrs, s.Rel(nameLb.Repo, nameLb.Pkg))
		}
		// includes / prefixes
		includes := getStringListAttr(t, "includes")
		var strip *string
		if s, ok := getStringAttr(t, "strip_include_prefix"); ok && s != "" {
			ss := normalizeRelativePath(s)
			strip = &ss
		}
		var pref *string
		if s, ok := getStringAttr(t, "include_prefix"); ok && s != "" {
			ps := normalizeRelativePath(s)
			pref = &ps
		}
		// deps
		deps := getLabelListAttr(t, "deps")
		for i := range deps {
			deps[i] = deps[i].Rel(nameLb.Repo, nameLb.Pkg)
		}
		// alias (if any) pointing to this rule
		var alias *label.Label
		if a, ok := aliases[nameLb]; ok {
			alias = &a
		}

		targets = append(targets, ModuleTarget{
			Name: nameLb, Alias: alias, Hdrs: hdrs,
			Includes: includes, StripIncludePref: strip, IncludePrefix: pref, Deps: deps,
		})
	}
	return targets, nil
}

// ---------------------- Attribute decoders ----------------------

func parseLabel(s string) (label.Label, bool) {
	lb, err := label.Parse(s)
	if err != nil {
		return label.NoLabel, false
	}
	return lb, true
}

func getStringAttr(t *qproto.Target, name string) (string, bool) {
	a := bzl.GetNamedAttribute(t, name)
	if a == nil {
		return "", false
	}

	// Handle configurable values: prefer a default arm if present; else first non-empty.
	if sl := a.GetSelectorList(); sl != nil {
		var fallback string
		for _, sel := range sl.GetElements() {
			for _, e := range sel.GetEntries() {
				v := e.GetStringValue()
				if v == "" {
					continue
				}
				if e.GetIsDefaultValue() {
					return v, true
				}
				if fallback == "" {
					fallback = v
				}
			}
		}
		if fallback != "" {
			return fallback, true
		}
		return "", false
	}
	v := a.GetStringValue()
	if v == "" {
		return "", false
	}
	return v, true
}

func getStringListAttr(t *qproto.Target, name string) []string {
	a := bzl.GetNamedAttribute(t, name)
	if a == nil {
		return nil
	}

	// Handle configurable lists: if a default arm exists and is non-empty, use only it.
	// Otherwise union all non-empty arms.
	if sl := a.GetSelectorList(); sl != nil {
		var out []string
		var haveDefault bool
		for _, sel := range sl.GetElements() {
			for _, e := range sel.GetEntries() {
				vals := e.GetStringListValue()
				if len(vals) == 0 {
					continue
				}
				if e.GetIsDefaultValue() {
					out = append([]string(nil), vals...)
					haveDefault = true
					break
				}
				out = append(out, vals...)
			}
			if haveDefault {
				break
			}
		}
		if haveDefault || len(out) > 0 {
			return out
		}
		return nil
	}
	if vals := a.GetStringListValue(); len(vals) > 0 {
		return append([]string(nil), vals...)
	}
	// Some attrs are scalar but you want list semantics.
	if s := a.GetStringValue(); s != "" {
		return []string{s}
	}
	return nil
}

func getLabelAttr(t *qproto.Target, name string) (label.Label, bool) {
	if s, ok := getStringAttr(t, name); ok {
		return parseLabel(s)
	}
	return label.NoLabel, false
}

func getLabelListAttr(t *qproto.Target, name string) []label.Label {
	var out []label.Label
	for _, s := range getStringListAttr(t, name) {
		if lb, ok := parseLabel(s); ok {
			out = append(out, lb)
		}
	}
	return out
}

// For OUTPUT-like attrs (e.g., expand_template.out): still strings in this proto.
func getOutputOrStringAttr(t *qproto.Target, name string) (label.Label, bool) {
	if s, ok := getStringAttr(t, name); ok {
		return parseLabel(s)
	}
	return label.NoLabel, false
}

// =====================================================================================
// Mapping creation
// =====================================================================================

func (a *App) createHeaderIndex(infos []ModuleInfo) mappingOutput {
	headers := map[string]coll.Set[label.Label]{}
	excluded := map[label.Label]coll.Set[label.Label]{}

	recordExcluded := func(owner label.Label, x any) {
		set, ok := excluded[owner]
		if !ok {
			set = make(coll.Set[label.Label])
		}
		switch t := x.(type) {
		case ModuleTarget:
			for _, h := range t.Hdrs {
				(&set).Add(h)
			}
		case label.Label:
			(&set).Add(t)
		}
		excluded[owner] = set
	}

	shouldExcludeHeader := func(path string) bool {
		if path == "" {
			return true
		}
		if _, exists := stdHeaders[path]; exists {
			return true
		}
		parts := strings.Split(path, "/")
		for _, seg := range parts {
			if strings.HasPrefix(seg, ".") || strings.HasPrefix(seg, "_") {
				return true
			}
		}
		if len(parts) > 0 {
			switch parts[0] {
			case "third-party", "third_party", "deps", "test":
				return true
			}
		}
		return false
	}

	badNameToken := regexp.MustCompile(`\W+`)
	shouldExcludeTarget := func(t label.Label) bool {
		// name tokens
		toks := func(s string) []string {
			s = badNameToken.ReplaceAllString(s, " ")
			parts := strings.Fields(s)
			for i := range parts {
				parts[i] = strings.TrimFunc(parts[i], func(r rune) bool {
					return !('A' <= r && r <= 'Z' || 'a' <= r && r <= 'z')
				})
			}
			return parts
		}
		nameSegs := toks(t.Name)
		for _, tok := range nameSegs {
			if tok == "internal" || tok == "impl" {
				return true
			}
		}
		if t.Pkg != "" {
			segs := strings.Split(t.Pkg, "/")
			for _, seg := range segs {
				for _, bad := range []string{"third-party", "third_party", "3rd_party", "deps", "tests", "internal", "impl", "test"} {
					if strings.Contains(seg, bad) {
						return true
					}
				}
			}
		}
		return false
	}

	for _, mod := range infos {
		for _, tgt := range mod.Targets {
			lbl := normalizeRepo(mod.Module.Name, tgt)
			if shouldExcludeTarget(lbl) {
				recordExcluded(lbl, tgt)
				continue
			}
			for _, hdr := range tgt.Hdrs {
				for _, p := range normalizeHeaderPath(strings.TrimPrefix(hdr.Name, "/"), tgt) {
					if shouldExcludeHeader(p) {
						recordExcluded(lbl, hdr)
						continue
					}
					set, ok := headers[p]
					if !ok {
						set = make(coll.Set[label.Label])
					}
					(&set).Add(lbl)
					headers[p] = set
				}
			}
		}
	}

	nonConf := map[string]label.Label{}
	conf := map[string][]label.Label{}
	for h, owners := range headers {
		vals := owners.Values()
		if len(vals) == 1 {
			nonConf[h] = vals[0]
		} else {
			slices.SortFunc(vals, func(a, b label.Label) int { return strings.Compare(a.String(), b.String()) })
			conf[h] = vals
		}
	}

	exclOut := map[label.Label][]label.Label{}
	for k, v := range excluded {
		exclOut[k] = v.Values()
	}

	if a.cfg.verbose {
		fmt.Println("Modules with conflicts:")
		seen := coll.Set[string]{}
		var repos []string
		for _, ls := range conf {
			for _, l := range ls {
				if l.Repo != "" && !(&seen).Contains(l.Repo) {
					(&seen).Add(l.Repo)
					repos = append(repos, l.Repo)
				}
			}
		}
		sort.Strings(repos)
		for i, r := range repos {
			fmt.Printf("%4d - %s\n", i, r)
		}
	}

	return mappingOutput{HeaderToRule: nonConf, Ambigious: conf, Excluded: exclOut}
}

func normalizeRepo(moduleName string, tgt ModuleTarget) label.Label {
	base := tgt.Name
	if tgt.Alias != nil {
		alias := *tgt.Alias
		use := false
		if alias.Pkg != "" && strings.Contains(alias.Pkg, base.Name) {
			use = true
		}
		if alias.Pkg == "" && alias.Name == moduleName {
			use = true
		}
		if use {
			base = alias
		}
	}
	base.Repo = moduleName
	return base
}

// normalizeHeaderPath mirrors the Scala pipeline:
// targetPkgResolved -> strip_include_prefix -> resolveIncludes -> include_prefix
func normalizeHeaderPath(hdrPath string, tgt ModuleTarget) []string {
	join := func(a, b string) string {
		if a == "" {
			return b
		}
		if b == "" {
			return a
		}
		return filepath.ToSlash(filepath.Join(a, b))
	}
	targetPkgResolved := func(p string) string {
		if tgt.Name.Pkg == "" {
			return p
		}
		return join(tgt.Name.Pkg, p)
	}
	stripIncludePrefix := func(p string) string {
		if tgt.StripIncludePref == nil {
			return p
		}
		cands := []string{*tgt.StripIncludePref, targetPkgResolved(*tgt.StripIncludePref)}
		for _, pref := range cands {
			if hasPrefixPath(p, pref) {
				return strings.TrimPrefix(strings.TrimPrefix(p, pref), "/")
			}
		}
		return p
	}
	resolveIncludes := func(p string) []string {
		var res []string
		for _, inc := range tgt.Includes {
			ip := targetPkgResolved(inc)
			if hasPrefixPath(p, ip) {
				pp := strings.TrimPrefix(strings.TrimPrefix(p, ip), "/")
				res = append(res, pp)
			}
		}
		if len(res) == 0 {
			return []string{p}
		}
		return res
	}
	includePrefix := func(p string) string {
		if tgt.IncludePrefix == nil {
			return p
		}
		return join(*tgt.IncludePrefix, p)
	}

	p0 := targetPkgResolved(hdrPath)
	p1 := stripIncludePrefix(p0)
	var outs []string
	for _, x := range resolveIncludes(p1) {
		outs = append(outs, includePrefix(x))
	}
	return outs
}

func hasPrefixPath(p, pref string) bool {
	p = filepath.ToSlash(p)
	pref = filepath.ToSlash(pref)
	if p == pref {
		return true
	}
	return strings.HasPrefix(p, pref+"/")
}

// =====================================================================================
// Driver
// =====================================================================================

func (a *App) gatherModuleInfos(registry string) ([]ModuleInfo, error) {
	modRoot := filepath.Join(registry, "modules")
	entries, err := os.ReadDir(modRoot)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Scanning %d modules for cc_rules\n", len(entries))

	type res struct{ r ResolveResult }
	in := make(chan string)
	out := make(chan res)
	workers := runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for m := range in {
			rr := a.processModule(filepath.Join(modRoot, m))
			if a.cfg.verbose {
				if rr.Info != nil {
					fmt.Fprintf(os.Stderr, "%-50s: resolved - cc_libraries: %d\n", rr.Info.Module.String(), len(rr.Info.Targets))
				} else {
					fmt.Fprintf(os.Stderr, "%-50s: failed   - %s\n", rr.Unresolved.Module.String(), rr.Unresolved.Reason)
				}
			}
			out <- res{r: rr}
		}
	}

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go worker()
	}

	go func() {
		for _, e := range entries {
			if e.IsDir() {
				in <- e.Name()
			}
		}
		close(in)
		wg.Wait()
		close(out)
	}()

	var infos []ModuleInfo
	var failed int
	for r := range out {
		if r.r.Info != nil && len(r.r.Info.Targets) > 0 {
			infos = append(infos, *r.r.Info)
		} else if r.r.isUnresolved() {
			failed++
		}
	}
	fmt.Printf("Found %d modules with non-empty cc_library defs\n", len(infos))
	fmt.Printf("Failed to gather module information in %d modules\n", failed)
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Module.Name == infos[j].Module.Name {
			return infos[i].Module.Version < infos[j].Module.Version
		}
		return infos[i].Module.Name < infos[j].Module.Name
	})
	return infos, nil
}

// =====================================================================================
// Utils & constants
// =====================================================================================

func writeJSON(path string, v any) error {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return os.WriteFile(path, mustJSON(v), 0o644)
}

func mustRead(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}

func mustJSON(v any) []byte {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return b
}

func isMacOS() bool { return strings.Contains(strings.ToLower(runtime.GOOS), "darwin") }

func die(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

func showModuleInfos(infos []ModuleInfo) {
	for i, mi := range infos {
		fmt.Printf("%d: %s - %d\n", i, mi.Module.String(), len(mi.Targets))
		for _, t := range mi.Targets {
			fmt.Printf("\t%s: %d headers\n", t.Name, len(t.Hdrs))
			for _, h := range t.Hdrs {
				fmt.Printf("\t\t%s\n", h)
			}
		}
	}
}

// POSIX/C std headers — used for excluding.
var posixStd = []string{
	"aio.h", "arpa/inet.h", "assert.h",
	"complex.h", "cpio.h", "ctype.h", "devctl.h", "dirent.h", "dlfcn.h",
	"endian.h", "errno.h", "fcntl.h", "fenv.h", "float.h", "fmtmsg.h",
	"fnmatch.h", "ftw.h", "glob.h", "grp.h", "iconv.h", "inttypes.h", "iso646.h",
	"langinfo.h", "libgen.h", "libintl.h", "limits.h", "locale.h", "math.h",
	"monetary.h", "mqueue.h", "ndbm.h", "net/if.h", "netdb.h", "netinet/in.h",
	"netinet/tcp.h", "nl_types.h", "poll.h", "pthread.h", "pwd.h", "regex.h",
	"sched.h", "search.h", "semaphore.h", "setjmp.h", "signal.h", "spawn.h",
	"stdalign.h", "stdarg.h", "stdatomic.h", "stdbool.h", "stddef.h", "stdint.h",
	"stdio.h", "stdlib.h", "stdnoreturn.h", "string.h", "strings.h", "sys.h",
	"sys/ipc.h", "sys/cdefs.h", "sys/mman.h", "sys/msg.h", "sys/resource.h",
	"sys/select.h", "sys/sem.h", "sys/shm.h", "sys/socket.h", "sys/stat.h",
	"sys/statvfs.h", "sys/time.h", "sys/times.h", "sys/types.h", "sys/uio.h",
	"sys/un.h", "sys/utsname.h", "sys/wait.h", "syslog.h", "tar.h", "termios.h",
	"tgmath.h", "threads.h", "time.h", "uchar.h", "unistd.h", "utmpx.h",
	"wchar.h", "wctype.h", "wordexp.h",
}
var cStd = []string{
	"assert.h", "complex.h", "ctype.h", "errno.h",
	"fenv.h", "float.h", "inttypes.h", "iso646.h", "limits.h", "locale.h",
	"math.h", "setjmp.h", "signal.h", "stdalign.h", "stdarg.h", "stdatomic.h",
	"stdbit.h", "stdbool.h", "stdckdint.h", "stddef.h", "stdint.h", "stdio.h",
	"stdlib.h", "stdmchar.h", "stdnoreturn.h", "string.h", "tgmath.h",
	"threads.h", "time.h", "uchar.h", "wchar.h", "wctype.h",
}

var stdHeaders = func() coll.Set[string] {
	hdrs := make(coll.Set[string])
	hdrs.Join(coll.ToSet(posixStd))
	hdrs.Join(coll.ToSet(cStd))
	return hdrs
}()
