"""
General utility functions, missing both in the native Bazel Starlark and the
Skylib library.
"""

def _enum(*values):
    """Creates a simple enum type.

    Args:
        *values: The possible enum values as strings.

    Returns:
        A struct representing the enum type with keys for each value.
    """
    return struct(**{value: value for value in values})

def _int_enum(*values):
    """Creates a simple integer enum type.

    Constants are mapped to consecutive integers starting from 0, like in C-style enums.

    Args:
        *values: The possible enum values as strings.

    Returns:
        A struct representing the enum type with keys for each value mapped to
        their integer index.
    """
    return struct(**{value: index for index, value in enumerate(values)})

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

def _ref_make(value):
    """Creates a mutable reference wrapper for use in nested functions.

    Starlark does not allow nested functions to modify variables from outer
    scopes. This function wraps a value in a single-element list to enable
    mutation inside nested functions. Starlark closure is not as handy as in
    e.g. Go, so this is a workaround.

    Usage:
        index = utils.ref_make(0)
        def nested():
            utils.ref_set(index, utils.ref_get(index) + 1)

    Args:
        value: The initial value to wrap.

    Returns:
        A struct with a single field '_ref' containing a single-element list.
    """
    return struct(_ref = [value])

def _ref_get(ref):
    """Gets the value of a mutable reference wrapper created by 'ref_make'.

    Args:
        ref: The reference wrapper struct created by 'ref_make'.

    Returns:
        The current value stored in the reference wrapper.
    """
    return ref._ref[0]

def _ref_set(ref, value):
    """Sets the value of a mutable reference wrapper created by 'ref_make'.

    Args:
        ref: The reference wrapper struct created by 'ref_make'.
        value: The new value to set.
    """
    ref._ref[0] = value

utils = struct(
    enum = _enum,
    int_enum = _int_enum,
    infinite_loop = _infinite_loop,
    ref_make = _ref_make,
    ref_get = _ref_get,
    ref_set = _ref_set,
)
