"""
Additional macros for Gazelle generation tests.
"""

load("@bazel_skylib//lib:paths.bzl", "paths")
load("@gazelle//:def.bzl", "gazelle_generation_test")

def gazelle_generation_test_suite(
        *,
        name,
        gazelle_binary,
        workspace_paths,
        subtest_suffix = "_generation_test",
        size = None,
        **kwargs):
    """
    gazelle_generation_test_suite is a macro that creates a suite of gazelle_generation_test tests.

    Args:
        name: The name of the test suite.
        gazelle_binary: The name of the gazelle binary target.
        workspace_paths: A list of workspace paths to create subtests for.
        subtest_suffix: Suffix to append to each subtest name.
        size: Size attribute to apply to all subtests.
        **kwargs: Attributes that are passed directly to the test suite and all subtests.
    """

    def subtest_name(workspace_path):
        return workspace_path.replace("/", "_") + subtest_suffix

    for workspace_path in workspace_paths:
        gazelle_generation_test(
            name = subtest_name(workspace_path),
            gazelle_binary = gazelle_binary,
            test_data = native.glob([paths.join(workspace_path, "**")]),
            size = size,
            **kwargs
        )

    native.test_suite(
        name = name,
        tests = [":" + subtest_name(workspace_path) for workspace_path in workspace_paths],
        **kwargs
    )
