"""
Based on CMakeLists.txt:
https://github.com/tree-sitter/tree-sitter/blob/d29132512b1fd73eff25099a2f7181a70c07e985/CMakeLists.txt

We want a separate target for the core library only, because it's easier to manage than using embedded sources from
go-tree-sitter repository. See how go-tree-sitter does it:
https://github.com/tree-sitter/go-tree-sitter/blob/adc13ffd8b2c0b01b878fda9f7c422ce0df5fad3/copy.sh
https://github.com/tree-sitter/go-tree-sitter/blob/adc13ffd8b2c0b01b878fda9f7c422ce0df5fad3/tree_sitter.go
"""

load("@rules_cc//cc:defs.bzl", "cc_library")

cc_library(
    name = "tree-sitter",
    srcs = glob(
        include = [
            "lib/src/*.c",
            "lib/src/*.h",
            "lib/src/portable/*.h",
        ],
        exclude = ["lib/src/lib.c"],
    ),
    hdrs = ["lib/include/tree_sitter/api.h"],
    copts = [
        "-Wall",
        "-Wextra",
        "-Wshadow",
        "-Wpedantic",
        "-Werror=incompatible-pointer-types",
        "-std=c11",
        "-fvisibility=hidden",
        "-fPIC",
    ],
    local_defines = [
        "_POSIX_C_SOURCE=200112L",
        "_DEFAULT_SOURCE",
    ],
    strip_include_prefix = "lib/include",
    visibility = ["@com_github_tree_sitter_go_tree_sitter//:__pkg__"],
)
