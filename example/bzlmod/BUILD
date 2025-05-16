load("@gazelle//:def.bzl", "gazelle", "gazelle_binary")

gazelle_binary(
    name = "gazelle_cc",
    languages = [
        "@gazelle//language/proto",
        "@gazelle_cc//language/cc",
    ],
)

gazelle(
    name = "gazelle",
    gazelle = ":gazelle_cc",
)

# gazelle:cc_group unit
# gazelle:resolve cc gtest/gtest.h @googletest//:gtest_main

# TODO Workaround to bug in proto gazelle
# gazelle:map_kind cc_proto_library cc_proto_library @com_google_protobuf//bazel:cc_proto_library.bzl
