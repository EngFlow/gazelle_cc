load("@gazelle//:def.bzl", "gazelle", "gazelle_binary")

gazelle_binary(
    name = "gazelle_cc",
    languages = [
        "@gazelle_cc//language/cc",
    ],
)

# gazelle:cc_group unit
# gazelle:cc_indexfile generated.ccindex
# gazelle:exclude third-party
gazelle(
    name = "gazelle",
    gazelle = ":gazelle_cc",
)
