"""
Additional macros for Gazelle generation tests.
"""

load("@gazelle//:def.bzl", "gazelle_generation_test")

def gazelle_generation_tests(
        name,
        gazelle_binary,
        test_data_map,
        size = None,
        visibility = None,
        tags = None
):
    """
    gazelle_generation_tests is a macro that creates a suite of gazelle_generation_test tests.

    Args:
        name: The name of the test suite.
        gazelle_binary: The name of the gazelle binary target.
        test_data_map: A map from subtest names to the test data passed to each single gazelle_generation_test.
        size: Size attribute to apply to all subtests.
        visibility: Visibility attribute to apply to all subtests.
        tags: Tags to apply to the test suite and all subtests.
    """
    for subtest_name, test_data in test_data_map.items():
        gazelle_generation_test(
            name = subtest_name,
            gazelle_binary = gazelle_binary,
            test_data = test_data,
            size = size,
            visibility = visibility,
            tags = tags,
        )

    native.test_suite(
        name = name,
        tests = [":" + subtest_name for subtest_name in test_data_map.keys()],
        visibility = visibility,
        tags = tags,
    )
