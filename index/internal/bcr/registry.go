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

package bcr

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/bazelbuild/bazel-gazelle/label"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/ulikunitz/xz"

	bzl "github.com/EngFlow/gazelle_cc/index/internal/bazel"
	qproto "github.com/EngFlow/gazelle_cc/index/internal/bazel/proto"
	"github.com/EngFlow/gazelle_cc/index/internal/indexer"
	"github.com/EngFlow/gazelle_cc/internal/collections"
)

type BazelRegistry struct {
	Config         BazelRegistryConfig
	RepositoryPath string
	httpClient     http.Client
}

type BazelRegistryConfig struct {
	CacheDir     string
	Verbose      bool
	KeepSources  bool
	RecomputeBad bool
	CacheBad     bool
}

func NewBazelRegistryConfig() BazelRegistryConfig {
	pwd, _ := os.Getwd()
	defaultCache := filepath.Join(pwd, ".cache")
	return BazelRegistryConfig{
		CacheDir: defaultCache,
	}
}

func newBazelRegistryClient(config BazelRegistryConfig, repositoryPath string) BazelRegistry {
	httpTransport := &http.Transport{
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxConnsPerHost:       8,
		MaxIdleConnsPerHost:   8,
	}
	httpClient := http.Client{
		Transport: httpTransport,
		Timeout:   5 * time.Minute, // overall per request
	}
	return BazelRegistry{
		Config:         config,
		RepositoryPath: repositoryPath,
		httpClient:     httpClient,
	}
}

func CheckoutBazelRegistry(config BazelRegistryConfig) (BazelRegistry, error) {
	repoDir := filepath.Join(config.CacheDir, "bazel-central-registry")
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
				return BazelRegistry{}, fmt.Errorf("git refresh failed: %w", err)
			}
		}
		return newBazelRegistryClient(config, repoDir), nil
	}

	if err := os.MkdirAll(config.CacheDir, 0o755); err != nil {
		return BazelRegistry{}, err
	}
	cmd := exec.Command("git", "clone", "https://github.com/bazelbuild/bazel-central-registry", "--depth=1", repoDir)
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return BazelRegistry{}, fmt.Errorf("git clone failed: %w", err)
	}
	return newBazelRegistryClient(config, repoDir), nil
}

func (bcr *BazelRegistry) ResolveModuleInfo(moduleName string, version string) ResolveModuleInfoResult {
	modulesDir := filepath.Join(bcr.RepositoryPath, "modules")

	metaPath := filepath.Join(modulesDir, moduleName, "metadata.json")
	b, err := os.ReadFile(metaPath)
	if err != nil {
		return unresolved(moduleName, "No metadata.json")
	}
	var meta metadataJSON
	if err := json.Unmarshal(b, &meta); err != nil || len(meta.Versions) == 0 {
		return unresolved(moduleName, "Invalid metadata.json")
	}

	if version == "" {
		latest := meta.Versions[len(meta.Versions)-1]
		mv := ModuleVersion{Name: moduleName, Version: latest}
		if _, yanked := meta.YankedVersions[latest]; yanked {
			return unresolvedMV(mv, "latest version is yanked - ignore")
		}
		version = latest
	}
	mv := ModuleVersion{Name: moduleName, Version: version}
	if !slices.Contains(meta.Versions, version) {
		return unresolvedMV(mv, "metadata not found for given version")
	}

	cacheFile := filepath.Join(bcr.Config.CacheDir, "modules", moduleName, version, "module-info.json")
	if cached, err := bcr.tryLoadCached(cacheFile); err == nil {
		if cached.IsResolved() || !bcr.Config.RecomputeBad {
			return cached
		}
	}

	sourcesDir := filepath.Join(modulesDir, moduleName, version)
	srcRootDir, projectRoot, err := bcr.prepareModuleSources(sourcesDir)
	if err != nil {
		rr := unresolvedMV(mv, "Failed to prepare project sources: "+err.Error())
		bcr.saveMaybe(cacheFile, rr)
		return rr
	}

	targets, err := bcr.resolveTargets(projectRoot)
	if !bcr.Config.KeepSources {
		_ = os.RemoveAll(srcRootDir)
	}
	if err != nil {
		rr := unresolvedMV(mv, "Failed to resolve module targets: "+err.Error())
		bcr.saveMaybe(cacheFile, rr)
		return rr
	}

	info := ModuleInfo{Module: mv, Targets: targets}
	rr := ResolveModuleInfoResult{Info: &info}
	bcr.saveMaybe(cacheFile, rr)

	return rr
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
	Name               label.Label   `json:"name"`
	Alias              *label.Label  `json:"alias,omitempty"`
	Hdrs               []label.Label `json:"hdrs"`
	Includes           []string      `json:"includes"`
	StripIncludePrefix *string       `json:"strip_include_prefix,omitempty"`
	IncludePrefix      *string       `json:"include_prefix,omitempty"`
	Deps               []label.Label `json:"deps"`
}

type ModuleInfo struct {
	Module  ModuleVersion  `json:"module"`
	Targets []ModuleTarget `json:"targets"`
}

func (m ModuleInfo) ToIndexerModule() indexer.Module {
	targets := make([]indexer.Target, 0, len(m.Targets))
	for _, target := range m.Targets {
		name := target.Name
		if target.Alias != nil {
			if target.Alias.Name == m.Module.Name || len(target.Alias.String()) < len(name.String()) {
				name = *target.Alias
			}
		}
		var stripIncludePrefix, includePrefix string
		if target.StripIncludePrefix != nil {
			stripIncludePrefix = *target.StripIncludePrefix
		}
		if target.IncludePrefix != nil {
			includePrefix = *target.IncludePrefix
		}
		targets = append(targets, indexer.Target{
			Name:               name,
			Hdrs:               collections.ToSet(target.Hdrs),
			Includes:           collections.ToSet(target.Includes),
			StripIncludePrefix: stripIncludePrefix,
			IncludePrefix:      includePrefix,
			Deps:               collections.ToSet(target.Deps),
		})
	}
	return indexer.Module{
		Repository: m.Module.Name,
		Targets:    targets,
	}
}

type ResolveModuleInfoResult struct {
	Info       *ModuleInfo `json:"info,omitempty"`
	Unresolved *struct {
		Module ModuleVersion `json:"module"`
		Reason string        `json:"reason"`
	} `json:"unresolved,omitempty"`
}

func (r ResolveModuleInfoResult) IsResolved() bool   { return r.Info != nil }
func (r ResolveModuleInfoResult) IsUnresolved() bool { return r.Unresolved != nil }

func normalizeRelativePath(s string) string { return strings.TrimLeft(strings.TrimSpace(s), "/") }

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

func unresolved(name, reason string) ResolveModuleInfoResult {
	return unresolvedMV(ModuleVersion{Name: name, Version: ""}, reason)
}
func unresolvedMV(mv ModuleVersion, reason string) ResolveModuleInfoResult {
	return ResolveModuleInfoResult{Unresolved: &struct {
		Module ModuleVersion `json:"module"`
		Reason string        `json:"reason"`
	}{Module: mv, Reason: reason}}
}

func (_ *BazelRegistry) tryLoadCached(path string) (ResolveModuleInfoResult, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ResolveModuleInfoResult{}, err
	}
	var rr ResolveModuleInfoResult
	if err := json.Unmarshal(b, &rr); err != nil {
		return ResolveModuleInfoResult{}, err
	}
	return rr, nil
}

func (bcr *BazelRegistry) saveMaybe(path string, rr ResolveModuleInfoResult) {
	if rr.IsUnresolved() && !bcr.Config.CacheBad {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, mustWriteJSON(rr), 0o644)
}

// =====================================================================================
// Sources: download / extract / patch
// =====================================================================================

func (bcr *BazelRegistry) prepareModuleSources(moduleVersionDir string) (sourcesDir, projectRoot string, err error) {
	rel, err := filepath.Rel(filepath.Join(moduleVersionDir, "..", ".."), moduleVersionDir)
	if err != nil {
		return "", "", err
	}
	targetDir := filepath.Join(bcr.Config.CacheDir, "modules", filepath.FromSlash(rel), "sources")
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

	archivePath, err := bcr.downloadWithRetries(src.URL)
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

func (bcr *BazelRegistry) downloadWithRetries(url string) (string, error) {
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
		resp, err := bcr.httpClient.Get(url)
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
func (_ *BazelRegistry) resolveTargets(projectRoot string) ([]ModuleTarget, error) {
	// Find nested repositories, these might need to be excluded
	innerModules, _ := doublestar.FilepathGlob(projectRoot + "/*/**/{MODULE,MODULE.bazel,WORKSPACE,WORKSPACE.bazel}")
	excludeConditions := collections.MapSlice(innerModules, func(modulePath string) string {
		relPath := strings.TrimPrefix(modulePath, projectRoot)
		relPath = strings.TrimPrefix(relPath, "/")
		relDirectory := filepath.Dir(relPath)
		return fmt.Sprintf("except //%v/...", filepath.ToSlash(relDirectory))
	})

	// Single query composing the same selector set as before.
	query := `(kind("cc_.*library|alias", //...:*) intersect attr(visibility, //visibility:public, //...:*)) union kind("expand_template|filegroup", //...:*) ` + strings.Join(excludeConditions, " ")
	result, err := bzl.ConfiguredQuery(projectRoot, query, bzl.QueryConfig{KeepGoing: true})
	if err != nil {
		log.Printf("query failed: %v, query:%v", err, query)
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
		ruleName, ok := parseLabel(rule.GetName())
		if !ok || shouldExcludeTarget(ruleName) {
			continue
		}
		// labels + expand filegroups/expand_template
		resolveSourceFiles := func(attribute string) []label.Label {
			sources := []label.Label{}
			for _, s := range getLabelListAttr(t, attribute) {
				if fg, ok := filegroups[s]; ok {
					for _, f := range fg {
						sources = append(sources, f.Rel(ruleName.Repo, ruleName.Pkg))
					}
					continue
				}
				if out, ok := expandTemplates[s]; ok {
					sources = append(sources, out.Rel(ruleName.Repo, ruleName.Pkg))
					continue
				}
				sources = append(sources, s.Rel(ruleName.Repo, ruleName.Pkg))
			}
			return sources
		}
		hdrs := resolveSourceFiles("hdrs")
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
			deps[i] = deps[i].Rel(ruleName.Repo, ruleName.Pkg)
		}
		// alias (if any) pointing to this rule
		var alias *label.Label
		if a, ok := aliases[ruleName]; ok {
			alias = &a
		}

		targets = append(targets, ModuleTarget{
			Name: ruleName, Alias: alias,
			Hdrs:     hdrs,
			Includes: includes, StripIncludePrefix: strip, IncludePrefix: pref,
			Deps: deps,
		})
	}
	return targets, nil
}

// shouldExcludeTarget determines if the given target (label) is possibly internal.
func shouldExcludeTarget(label label.Label) bool {
	// Check target's path segments: if any segment (split on non-word characters and filtered to letters)
	for _, segment := range strings.Split(label.Pkg, string(filepath.Separator)) {
		switch segment {
		case "thirdparty", "third-party", "third_party", "3rd_party", "deps", "tests", "internal", "impl", "test":
			return true
		}
	}
	return false
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
// Utils & constants
// =====================================================================================

func mustRead(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}

func mustWriteJSON(v any) []byte {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return b
}

func isMacOS() bool { return strings.Contains(strings.ToLower(runtime.GOOS), "darwin") }
