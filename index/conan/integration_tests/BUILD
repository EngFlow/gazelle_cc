load("@rules_go//go:def.bzl", "go_test")

# gazelle:exclude testcases

go_test(
    name = "integration_tests_test",
    timeout = "eternal",
    srcs = ["integration_test.go"],
    data = [":testcases"],
    tags = ["manual"],
    deps = ["//index/internal/tests"],
)
