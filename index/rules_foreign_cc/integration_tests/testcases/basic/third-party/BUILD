load("@rules_foreign_cc//foreign_cc:defs.bzl", "cmake")
load("@rules_cc//cc:defs.bzl", "cc_library")

cmake(
    name = "fmt_build",
    lib_source = "@fmt_sources//:all_srcs",
    out_static_libs = ["libfmt.a"],
    # fmt's own tests fail to compile against recent libc++ (e.g. macOS 27 SDK).
    cache_entries = {
        "FMT_DOC": "OFF",
        "FMT_TEST": "OFF",
    },
)
cc_library(
    name = "fmt",
    deps = [":fmt_build"],
    visibility = ["//visibility:public"],
)
