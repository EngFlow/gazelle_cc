load("@gazelle//:def.bzl", "gazelle", "gazelle_binary")

gazelle_binary(
    name = "gazelle_cc",
    languages = [
        "@gazelle_cc//language/cc",
    ],
    visibility = ["//visibility:public"],
)

# gazelle:cc_group unit
# gazelle:cc_indexfile generated.ccindex
# gazelle:exclude third_party
gazelle(
    name = "gazelle",
    gazelle = ":gazelle_cc",
)
