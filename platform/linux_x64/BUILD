load("@rules_cc//cc:defs.bzl", "cc_toolchain", "cc_toolchain_suite")
load(":cc_toolchain_config.bzl", "cc_toolchain_config")

package(default_visibility = ["//visibility:public"])

platform(
    name = "linux_x64",
    constraint_values = [
        "@platforms//os:linux",
        "@platforms//cpu:x86_64",
        "@bazel_tools//tools/cpp:clang",
        "@rules_go//go/toolchain:cgo_off",
    ],
    exec_properties = {
        "container-image": "docker://gcr.io/bazel-public/ubuntu2004-java11@sha256:69a78f121230c6d5cbfe2f4af8ce65481aa3f2acaaaf8e899df335f1ac1b35b5",
        "OSFamily": "Linux",
    },
)

toolchain(
    name = "cc-toolchain",
    exec_compatible_with = [
        "@platforms//os:linux",
        "@platforms//cpu:x86_64",
    ],
    target_compatible_with = [
        "@platforms//os:linux",
        "@platforms//cpu:x86_64",
    ],
    toolchain = ":cc-compiler-k8",
    toolchain_type = "@bazel_tools//tools/cpp:toolchain_type",
)

cc_toolchain_suite(
    name = "toolchain",
    target_compatible_with = [
        "@platforms//os:linux",
    ],
    toolchains = {
        "k8|clang": ":cc-compiler-k8",
        "k8": ":cc-compiler-k8",
    },
)

cc_toolchain(
    name = "cc-compiler-k8",
    all_files = ":compiler_deps",
    ar_files = ":compiler_deps",
    as_files = ":compiler_deps",
    compiler_files = ":compiler_deps",
    dwp_files = ":empty",
    linker_files = ":compiler_deps",
    module_map = ":module.modulemap",
    objcopy_files = ":empty",
    strip_files = ":empty",
    supports_param_files = 1,
    toolchain_config = ":linux_gnu_x86",
    toolchain_identifier = "linux_gnu_x86",
)

cc_toolchain_config(
    name = "linux_gnu_x86",
    abi_libc_version = "glibc_2.19",
    abi_version = "clang",
    compile_flags = [
        "-U_FORTIFY_SOURCE",
        "-fstack-protector",
        "-Wall",
        "-Wthread-safety",
        "-Wself-assign",
        "-fcolor-diagnostics",
        "-fno-omit-frame-pointer",
    ],
    compiler = "clang",
    coverage_compile_flags = ["--coverage"],
    coverage_link_flags = ["--coverage"],
    cpu = "k8",
    cxx_builtin_include_directories = [
        "/usr/local/include",
        "/usr/lib/llvm-10/lib/clang/10.0.0/include",
        "/usr/include/x86_64-linux-gnu",
        "/usr/include",
        "/usr/lib/llvm-10/lib/clang/10.0.0/share",
        "/usr/include/c++/9",
        "/usr/include/x86_64-linux-gnu/c++/9",
        "/usr/include/c++/9/backward",
        "/usr/lib/llvm-10/include/c++/v1",
    ],
    cxx_flags = ["-std=c++0x"],
    dbg_compile_flags = ["-g"],
    host_system_name = "i686-unknown-linux-gnu",
    link_flags = [
        "-fuse-ld=/usr/bin/ld.gold",
        "-Wl,-no-as-needed",
        "-Wl,-z,relro,-z,now",
        "-B/usr/lib/llvm-10/bin",
    ],
    link_libs = [
        "-lstdc++",
        "-lm",
    ],
    opt_compile_flags = [
        "-g0",
        "-O2",
        "-D_FORTIFY_SOURCE=1",
        "-DNDEBUG",
        "-ffunction-sections",
        "-fdata-sections",
    ],
    opt_link_flags = ["-Wl,--gc-sections"],
    supports_start_end_lib = True,
    target_libc = "glibc_2.19",
    target_system_name = "x86_64-unknown-linux-gnu",
    tool_paths = {
        "ar": "/usr/bin/ar",
        "ld": "/usr/bin/ld",
        "llvm-cov": "/usr/bin/llvm-cov",
        "cpp": "/usr/bin/cpp",
        "gcc": "/usr/lib/llvm-10/bin/clang",
        "dwp": "/usr/bin/dwp",
        "gcov": "/usr/bin/gcov",
        "nm": "/usr/bin/nm",
        "objcopy": "/usr/bin/objcopy",
        "objdump": "/usr/bin/objdump",
        "strip": "/usr/bin/strip",
    },
    toolchain_identifier = "linux_gnu_x86",
    unfiltered_compile_flags = [
        "-no-canonical-prefixes",
        "-Wno-builtin-macro-redefined",
        "-D__DATE__=\"redacted\"",
        "-D__TIMESTAMP__=\"redacted\"",
        "-D__TIME__=\"redacted\"",
    ],
)

filegroup(
    name = "compiler_deps",
    srcs = glob(
        ["extra_tools/**"],
        allow_empty = True,
    ) + [":builtin_include_directory_paths"],
)

filegroup(
    name = "empty",
)
