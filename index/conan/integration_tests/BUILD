load("@rules_go//go:def.bzl", "go_test")
load("//index/internal/tests:indexer_integration_test.bzl", "indexer_integration_test")

# gazelle:exclude testcases

indexer_integration_test(
    name = "integration_test",
    srcs = ["integration_test.go"],
    gazelle_binary_path = "//:gazelle_cc",
    indexer_binary_path = "//index/conan",
)
