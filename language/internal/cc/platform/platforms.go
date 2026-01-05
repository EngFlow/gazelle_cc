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

// Package 'platform' defines a normalized representation of operating system
// and architecture combinations used to model target platforms.
//
// It provides:
//   - The Platform type, representing an OS/Arch pair
//   - Parsing utilities for canonicalizing platform strings (e.g., "linux/x86_64")
//   - Aliasing support for common OS/Arch names
//   - Precomputed macro environments (e.g. _WIN32, __linux__) per platform
//
// This information is used for conditional compilation and macro evaluation
// within the Gazelle C++ extension.
package platform

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/EngFlow/gazelle_cc/language/internal/cc/parser"
)

// Pair of OS/Arch combination identifing a given platform
type Platform struct {
	OS   OS
	Arch Arch
}

func (p Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}

// Orders first by OS, then by Arch based on the string ordering
func Compare(a, b Platform) int {
	if d := cmp.Compare(a.OS, b.OS); d != 0 {
		return d
	}
	return cmp.Compare(a.Arch, b.Arch)
}

func Create(os OS, arch Arch) (Platform, error) {
	platform := Platform{
		OS:   dealias(os, osAlias),
		Arch: dealias(arch, archAlias),
	}
	if !slices.Contains(allKnownOs, platform.OS) {
		return platform, fmt.Errorf("unknown OS %v, expected one of known values %v or an alias %v", platform.OS, allKnownOs, osAlias)
	}
	if !slices.Contains(allKnownArch, platform.Arch) {
		return platform, fmt.Errorf("unknown architecture %v, expected one of known values %v or an alias %v", platform.Arch, allKnownArch, archAlias)
	}
	return platform, nil
}

// Operating system string identifier matching constraint value names defined in '@platforms//os'.
// Should match one the values defined in https://github.com/bazelbuild/platforms/blob/1.0.0/os/BUILD
type OS string

const (
	android    OS = "android"
	chromiumos OS = "chromiumos"
	emscripten OS = "emscripten"
	freebsd    OS = "freebsd"
	fuchsia    OS = "fuchsia"
	haiku      OS = "haiku"
	ios        OS = "ios"
	linux      OS = "linux"
	netbsd     OS = "netbsd"
	nixos      OS = "nixos"
	none       OS = "none" // bare-metal
	openbsd    OS = "openbsd"
	osx        OS = "osx"
	qnx        OS = "qnx"
	tvos       OS = "tvos"
	uefi       OS = "uefi"
	visionos   OS = "visionos"
	vxworks    OS = "vxworks"
	wasi       OS = "wasi"
	watchos    OS = "watchos"
	windows    OS = "windows"
)

var osAlias = map[string]OS{
	"macos": osx,
}
var allKnownOs = []OS{
	android, chromiumos, emscripten, freebsd, fuchsia, haiku, ios,
	linux, netbsd, nixos, none, openbsd, osx, qnx, tvos,
	uefi, visionos, vxworks, wasi, watchos, windows,
}

// Architecture string identifier matching constraint value names defined in '@platforms//cpu'.
// Should match one the values defined in https://github.com/bazelbuild/platforms/blob/1.0.0/cpu/BUILD
type Arch string

const (
	all       Arch = "all" // architecture-independent
	aarch32   Arch = "aarch32"
	aarch64   Arch = "aarch64"
	arm64_32  Arch = "arm64_32"
	arm64e    Arch = "arm64e"
	armv6m    Arch = "armv6-m"
	armv7     Arch = "armv7"
	armv7em   Arch = "armv7e-m"
	armv7emf  Arch = "armv7e-mf"
	armv7k    Arch = "armv7k"
	armv7m    Arch = "armv7-m"
	armv8m    Arch = "armv8-m"
	cortexr52 Arch = "cortex-r52"
	cortexr82 Arch = "cortex-r82"
	i386      Arch = "i386"
	mips64    Arch = "mips64"
	ppc       Arch = "ppc"
	ppc32     Arch = "ppc32"
	ppc64le   Arch = "ppc64le"
	riscv32   Arch = "riscv32"
	riscv64   Arch = "riscv64"
	s390x     Arch = "s390x"
	wasm32    Arch = "wasm32"
	wasm64    Arch = "wasm64"
	x86_32    Arch = "x86_32"
	x86_64    Arch = "x86_64"
)

var archAlias = map[string]Arch{
	"arm":   aarch32,
	"arm64": aarch64,
	"amd64": x86_64,
}

var allKnownArch = []Arch{
	aarch32, aarch64, arm64_32, arm64e, armv6m, armv7, armv7em, armv7emf,
	armv7k, armv7m, armv8m, cortexr52, cortexr82, i386, mips64, ppc,
	ppc32, ppc64le, riscv32, riscv64, s390x, wasm32, wasm64, x86_32, x86_64,
}

// Dictionary of well known macro definition for given platforms, initialized in init function
var KnownPlatformEnv = map[Platform]parser.Environment{}

func init() {
	//----------------------------------------------------------------------
	//                                Windows
	//----------------------------------------------------------------------
	windowsArchs := []Arch{i386, x86_32, x86_64, aarch32, aarch64}
	addMacro("_WIN32", osArchPlatforms(windows, windowsArchs))
	addMacro("_WIN64", osArchPlatforms(windows, []Arch{x86_64, aarch64}))
	addMacro("__MINGW32__", osArchPlatform(windows, i386))
	addMacro("__MINGW64__", osArchPlatform(windows, x86_64))
	addMacro("_M_IX86", osArchPlatform(windows, i386))
	addMacro("_M_X64", osArchPlatform(windows, x86_64))
	addMacro("_M_ARM", osArchPlatform(windows, aarch32))
	addMacro("_M_ARM64", osArchPlatform(windows, aarch64))

	//----------------------------------------------------------------------
	//                          Linux / Android family
	//----------------------------------------------------------------------
	linuxArchs := allKnownArch
	addMacros(
		[]string{"linux", "__linux__", "__linux", "__gnu_linux__"},
		osArchPlatforms(linux, linuxArchs),
	)
	addMacro("__NIX__", osArchPlatforms(nixos, linuxArchs))
	addMacro("__NIXOS__", osArchPlatforms(nixos, linuxArchs))

	androidArchs := []Arch{aarch32, aarch64, x86_32, x86_64, riscv64}
	addMacro("__ANDROID__", osArchPlatforms(android, androidArchs))

	chromeArchs := []Arch{x86_64, aarch64, riscv64}
	addMacro("__CHROMEOS__", osArchPlatforms(chromiumos, chromeArchs))

	// Apple does not define unix even though it's unix like os
	unixOS := []OS{linux, android, chromiumos, nixos, freebsd, netbsd, openbsd, haiku, qnx}
	addMacros(
		[]string{"unix", "__unix", "__unix__"},
		platformsMatrix(unixOS, allKnownArch),
	)

	//----------------------------------------------------------------------
	//  WebAssembly (Emscripten & WASI)
	//----------------------------------------------------------------------
	wasmArchs := []Arch{wasm32, wasm64}
	addMacro("__EMSCRIPTEN__", platformsMatrix([]OS{emscripten}, wasmArchs))
	addMacro("__wasi__", platformsMatrix([]OS{wasi}, wasmArchs))
	addMacro("__wasm__", platformsMatrix([]OS{emscripten, wasi}, wasmArchs))
	addMacro("__wasm32__", platformsMatrix([]OS{emscripten, wasi}, []Arch{wasm32}))
	addMacro("__wasm64__", platformsMatrix([]OS{emscripten, wasi}, []Arch{wasm64}))

	//----------------------------------------------------------------------
	//  BSD family
	//----------------------------------------------------------------------
	bsdArchs := []Arch{i386, x86_64, aarch64, riscv64, ppc64le}
	addMacro("__FreeBSD__", platformsMatrix([]OS{freebsd}, bsdArchs))
	addMacro("__NetBSD__", platformsMatrix([]OS{netbsd}, bsdArchs))
	addMacro("__OpenBSD__", platformsMatrix([]OS{openbsd}, bsdArchs))

	//----------------------------------------------------------------------
	//  QNX, Haiku, Fuchsia, VxWorks, UEFI
	//----------------------------------------------------------------------
	qnxArchs := []Arch{aarch32, aarch64, ppc32, ppc64le, x86_32, x86_64}
	addMacro("__QNX__", osArchPlatforms(qnx, qnxArchs))
	addMacro("__QNXNTO__", osArchPlatforms(qnx, qnxArchs))

	haikuArchs := []Arch{x86_32, x86_64}
	addMacro("__HAIKU__", osArchPlatforms(haiku, haikuArchs))

	fuchsiaArchs := []Arch{aarch64, x86_64}
	addMacro("__FUCHSIA__", osArchPlatforms(fuchsia, fuchsiaArchs))
	addMacro("__Fuchsia__", osArchPlatforms(fuchsia, fuchsiaArchs))

	vxworksArchs := []Arch{aarch32, aarch64, ppc32, ppc64le, x86_32, x86_64}
	addMacro("__VXWORKS__", osArchPlatforms(vxworks, vxworksArchs))
	addMacro("__vxworks", osArchPlatforms(vxworks, vxworksArchs))

	uefiArchs := []Arch{aarch32, aarch64, x86_32, x86_64, riscv64}
	addMacro("__UEFI__", osArchPlatforms(uefi, uefiArchs))
	addMacro("__EFI__", osArchPlatforms(uefi, uefiArchs))

	//----------------------------------------------------------------------
	//  Apple family
	//----------------------------------------------------------------------
	// Apple family (modern, so no 32-bit x86 or armv6 any more)
	macArchs := []Arch{x86_64, aarch64, arm64e}
	iosArchs := []Arch{aarch64, arm64e}
	tvosArchs := []Arch{aarch64}
	watchArchs := []Arch{armv7k, arm64_32}
	visionArchs := []Arch{aarch64}
	applePlatforms := slices.Concat(
		osArchPlatforms(osx, macArchs),
		osArchPlatforms(ios, iosArchs),
		osArchPlatforms(tvos, tvosArchs),
		osArchPlatforms(watchos, watchArchs),
		osArchPlatforms(visionos, visionArchs),
	)
	addMacro("__APPLE__", applePlatforms)
	addMacro("__MACH__", applePlatforms)
	addMacro("TARGET_OS_OSX", osArchPlatforms(osx, macArchs))
	addMacro("TARGET_OS_MAC", osArchPlatforms(osx, macArchs))
	addMacro("TARGET_OS_IPHONE", osArchPlatforms(ios, iosArchs))
	addMacro("TARGET_OS_IOS", osArchPlatforms(ios, iosArchs))
	addMacro("TARGET_OS_TV", osArchPlatforms(tvos, tvosArchs))
	addMacro("TARGET_OS_WATCH", osArchPlatforms(watchos, watchArchs))
	addMacro("TARGET_OS_VISION", osArchPlatforms(visionos, visionArchs))

	//----------------------------------------------------------------------
	//  Generic CPU-only macros
	//----------------------------------------------------------------------
	addMacros(
		[]string{"__x86_64__", "__x86_64", "__amd64", "__amd64__"},
		archOsPlatforms(aarch64, allKnownOs),
	)
	addMacros(
		[]string{"__i386__", "__i386"},
		archOsPlatforms(i386, allKnownOs),
	)
	addMacros(
		[]string{"__arm__", "__arm", "__thumb__", "__thumb"},
		archOsPlatforms(aarch32, allKnownOs),
	)
	addMacros(
		[]string{"__aarch64__", "__arm64", "__arm64__"},
		archOsPlatforms(aarch64, allKnownOs),
	)
	addMacros(
		[]string{"__ARM64_32__", "__ARM64_32"},
		osArchPlatform(watchos, arm64_32),
	)
	addMacros(
		[]string{"__arm64e__", "__arm64e"},
		archOsPlatforms(arm64e, []OS{osx, ios}),
	)

	// Fine-grained Arm (mostly bare-metal)
	addMacro("__ARM_ARCH_6M__", osArchPlatform(none, armv6m))
	addMacro("__ARM_ARCH_7__", osArchPlatform(none, armv7))
	addMacro("__ARM_ARCH_7A__", osArchPlatform(none, armv7))
	addMacro("__ARM_ARCH_7M__", osArchPlatform(none, armv7m))
	addMacro("__ARM_ARCH_7EM__", osArchPlatform(none, armv7em))
	addMacro("__ARM_ARCH_8M_BASE__", osArchPlatform(none, armv8m))
	addMacro("__ARM_ARCH_8M_MAIN__", osArchPlatform(none, armv8m))

	//----------------------------------------------------------------------
	//  PowerPC
	//----------------------------------------------------------------------
	powerPCOS := []OS{linux, freebsd, netbsd, openbsd, qnx, vxworks}
	addMacro("__powerpc__", archOsPlatforms(ppc32, powerPCOS))
	addMacro("__PPC__", archOsPlatforms(ppc32, powerPCOS))
	addMacro("__powerpc64__", archOsPlatforms(ppc64le, powerPCOS))
	addMacro("__ppc64__", archOsPlatforms(ppc64le, powerPCOS))

	//----------------------------------------------------------------------
	//  MIPS
	//----------------------------------------------------------------------
	mipsOS := []OS{linux, netbsd, openbsd, qnx, vxworks}
	addMacro("__mips64", archOsPlatforms(mips64, mipsOS))

	//----------------------------------------------------------------------
	//  s390
	//----------------------------------------------------------------------
	addMacro("__s390x__", osArchPlatform(linux, s390x))
	addMacro("__s390__", osArchPlatform(linux, s390x))

	//----------------------------------------------------------------------
	//  RISC-V
	//----------------------------------------------------------------------
	riscvOS := []OS{linux, freebsd, netbsd, openbsd, qnx, vxworks, android, chromiumos, fuchsia, nixos}
	addMacro("__riscv", archOsPlatforms(riscv64, riscvOS))
}

// addMacro adds a single macro to every platform in the list.
func addMacroValue(name string, value int, platforms []Platform) {
	for _, platform := range platforms {
		macros, exists := KnownPlatformEnv[platform]
		if !exists {
			macros = make(parser.Environment, 8)
			KnownPlatformEnv[platform] = macros
		}
		macros[name] = value
	}
}

func addMacro(name string, platforms []Platform) {
	// `#define NAME`` is assumed equal to `#define NAME 1`
	addMacroValue(name, 1, platforms)
}

func addMacros(macro []string, platforms []Platform) {
	for _, name := range macro {
		addMacro(name, platforms)
	}
}

func osArchPlatform(os OS, arch Arch) []Platform {
	return []Platform{{os, arch}}
}
func osArchPlatforms(os OS, arch []Arch) []Platform {
	return append(platformsMatrix([]OS{os}, arch), Platform{OS: os})
}

func archOsPlatforms(arch Arch, os []OS) []Platform {
	return append(platformsMatrix(os, []Arch{arch}), Platform{Arch: arch})
}

func platformsMatrix(os []OS, arch []Arch) []Platform {
	result := []Platform{}
	for _, os := range os {
		for _, arch := range arch {
			result = append(result, Platform{OS: os, Arch: arch})
		}
	}
	return result
}

func dealias[T ~string](value T, aliases map[string]T) T {
	if dealiased, exists := aliases[string(value)]; exists {
		return dealiased
	}
	return T(value)
}
