"""
Additional macros for Gazelle generation tests.
"""

load("@gazelle//:def.bzl", "gazelle_generation_test")

def gazelle_generation_test_suite(*, name, gazelle_binary, test_data_map, size = None, **kwargs):
    """
    gazelle_generation_test_suite is a macro that creates a suite of gazelle_generation_test tests.

    Args:
        name: The name of the test suite.
        gazelle_binary: The name of the gazelle binary target.
        test_data_map: A map from subtest names to the test data passed to each single gazelle_generation_test.
        size: Size attribute to apply to all subtests.
        **kwargs: Attributes that are passed directly to the test suite and all subtests.
    """
    for subtest_name, test_data in test_data_map.items():
        gazelle_generation_test(
            name = subtest_name,
            gazelle_binary = gazelle_binary,
            test_data = test_data,
            size = size,
            **kwargs
        )

    native.test_suite(
        name = name,
        tests = [":" + subtest_name for subtest_name in test_data_map.keys()],
        **kwargs
    )
