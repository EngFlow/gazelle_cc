load("@rules_foreign_cc//foreign_cc:defs.bzl", "cmake")
load("@rules_cc//cc:defs.bzl", "cc_library")

cmake(
    name = "fmt_build",
    lib_source = "@fmt_sources//:all_srcs",
    out_static_libs = ["libfmt.a"],
)
cc_library(
    name = "fmt",
    deps = [":fmt_build"],
    visibility = ["//visibility:public"],
)
