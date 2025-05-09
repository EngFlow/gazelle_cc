load("@rules_cc//cc:defs.bzl", "cc_library")

cc_library(
    name = "foo",
    hdrs = ["foo.h"],
    include_prefix = "lib_a",
    includes = ["third_party"],
    visibility = ["//visibility:public"],
)
