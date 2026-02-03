"""
General utility functions, missing both in the native Bazel Starlark and the
Skylib library.
"""

def _enum_type(*values):
    """Creates a simple enum type.

    Args:
        *values: The possible enum values as strings.

    Returns:
        A struct representing the enum type with keys for each value.
    """
    return struct(**{value: value for value in values})

def _infinite_loop(iterations = 1000000):
    """Emulates 'while True' by returning a reasonable long range.

    Starlark does not support 'while' loops, especially 'while True'. To
    mitigate this, this function returns a long list of booleans that can be
    iterated over to simulate an infinite loop. Only the last value is True, so
    the following pattern can be used:

    ```
    for err in utils.infinite_loop():
        if err:
            fail("exceeded maximum iterations")

        # loop body
    ```

    Args:
        iterations: Number of False values to return before the final True.
          (default: 1000000)

    Returns:
        A list of booleans with 'iterations' False values followed by a True.
    """
    return [False] * iterations + [True]

utils = struct(
    enum_type = _enum_type,
    infinite_loop = _infinite_loop,
)
