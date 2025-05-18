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
