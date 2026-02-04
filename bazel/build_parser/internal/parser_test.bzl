"""
Unit tests for the BUILD file parser.
"""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load(":parser.bzl", "parse")
load(":syntax.bzl", "ast_node", "ast_node_types")

def _simple_call_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    cc_library(
        name = "mylib",
        srcs = ["mylib.cc"],
        hdrs = ["mylib.h"],
    )
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_call(
                callable = ast_node.make_ident(name = "cc_library"),
                positional_args = [],
                keyword_args = [
                    ast_node.make_key_value(
                        key = "name",
                        value = ast_node.make_string(value = "mylib"),
                    ),
                    ast_node.make_key_value(
                        key = "srcs",
                        value = ast_node.make_list(
                            elements = [ast_node.make_string(value = "mylib.cc")],
                        ),
                    ),
                    ast_node.make_key_value(
                        key = "hdrs",
                        value = ast_node.make_list(
                            elements = [ast_node.make_string(value = "mylib.h")],
                        ),
                    ),
                ],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _load_statement_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    load("@rules_cc//cc:defs.bzl", "cc_library", "cc_binary")
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_call(
                callable = ast_node.make_ident(name = "load"),
                positional_args = [
                    ast_node.make_string(value = "@rules_cc//cc:defs.bzl"),
                    ast_node.make_string(value = "cc_library"),
                    ast_node.make_string(value = "cc_binary"),
                ],
                keyword_args = [],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _multiple_statements_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    load("@rules_cc//cc:defs.bzl", "cc_library")

    SRCS = ["a.cc", "b.cc"]

    cc_library(
        name = "mylib",
        srcs = SRCS,
    )

    cc_library(
        name = "other",
        srcs = ["other.cc"],
    )
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_call(
                callable = ast_node.make_ident(name = "load"),
                positional_args = [
                    ast_node.make_string(value = "@rules_cc//cc:defs.bzl"),
                    ast_node.make_string(value = "cc_library"),
                ],
                keyword_args = [],
            ),
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "SRCS"),
                op = "=",
                right = ast_node.make_list(
                    elements = [
                        ast_node.make_string(value = "a.cc"),
                        ast_node.make_string(value = "b.cc"),
                    ],
                ),
            ),
            ast_node.make_call(
                callable = ast_node.make_ident(name = "cc_library"),
                positional_args = [],
                keyword_args = [
                    ast_node.make_key_value(
                        key = "name",
                        value = ast_node.make_string(value = "mylib"),
                    ),
                    ast_node.make_key_value(
                        key = "srcs",
                        value = ast_node.make_ident(name = "SRCS"),
                    ),
                ],
            ),
            ast_node.make_call(
                callable = ast_node.make_ident(name = "cc_library"),
                positional_args = [],
                keyword_args = [
                    ast_node.make_key_value(
                        key = "name",
                        value = ast_node.make_string(value = "other"),
                    ),
                    ast_node.make_key_value(
                        key = "srcs",
                        value = ast_node.make_list(
                            elements = [ast_node.make_string(value = "other.cc")],
                        ),
                    ),
                ],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _string_concatenation_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    "a" + "b" + "c"
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_binary_op(
                    left = ast_node.make_string(value = "a"),
                    op = "+",
                    right = ast_node.make_string(value = "b"),
                ),
                op = "+",
                right = ast_node.make_string(value = "c"),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _higher_order_function_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    i_return_a_function(inner_expr)(outer_expr)
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_call(
                callable = ast_node.make_call(
                    callable = ast_node.make_ident(name = "i_return_a_function"),
                    positional_args = [ast_node.make_ident(name = "inner_expr")],
                    keyword_args = [],
                ),
                positional_args = [ast_node.make_ident(name = "outer_expr")],
                keyword_args = [],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _binary_operators_priorities_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    1 + 2 * 3 - 4 / 5
    """

    # Should be parsed as:
    #       (-)
    #      /   \
    #    (+)   (/)
    #   /  \   /  \
    # (1)  (*) (4) (5)
    #      / \
    #    (2) (3)

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_binary_op(
                    left = ast_node.make_number(value = "1"),
                    op = "+",
                    right = ast_node.make_binary_op(
                        left = ast_node.make_number(value = "2"),
                        op = "*",
                        right = ast_node.make_number(value = "3"),
                    ),
                ),
                op = "-",
                right = ast_node.make_binary_op(
                    left = ast_node.make_number(value = "4"),
                    op = "/",
                    right = ast_node.make_number(value = "5"),
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _glob_expression_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    cc_library(
        name = "my_lib",
        srcs = glob(["*.c", "*.h"], exclude = ["*_test.cc"]),
    )
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_call(
                callable = ast_node.make_ident(name = "cc_library"),
                positional_args = [],
                keyword_args = [
                    ast_node.make_key_value(
                        key = "name",
                        value = ast_node.make_string(value = "my_lib"),
                    ),
                    ast_node.make_key_value(
                        key = "srcs",
                        value = ast_node.make_call(
                            callable = ast_node.make_ident(name = "glob"),
                            positional_args = [
                                ast_node.make_list(
                                    elements = [
                                        ast_node.make_string(value = "*.c"),
                                        ast_node.make_string(value = "*.h"),
                                    ],
                                ),
                            ],
                            keyword_args = [
                                ast_node.make_key_value(
                                    key = "exclude",
                                    value = ast_node.make_list(
                                        elements = [
                                            ast_node.make_string(value = "*_test.cc"),
                                        ],
                                    ),
                                ),
                            ],
                        ),
                    ),
                ],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _select_expression_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    cc_library(
        name = "my_lib",
        deps = [
            "//shared:api",
        ] + select({
            "//platforms/linux_x86": [
                "//select:32bits",
            ],
            "@platforms//os:windows": [
                "//select:64bits",
                "//select:non_unix",
                "//select:win",
            ],
            "//conditions:default": [],
        }),
    )
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_call(
                callable = ast_node.make_ident(name = "cc_library"),
                positional_args = [],
                keyword_args = [
                    ast_node.make_key_value(
                        key = "name",
                        value = ast_node.make_string(value = "my_lib"),
                    ),
                    ast_node.make_key_value(
                        key = "deps",
                        value = ast_node.make_binary_op(
                            left = ast_node.make_list(
                                elements = [
                                    ast_node.make_string(value = "//shared:api"),
                                ],
                            ),
                            op = "+",
                            right = ast_node.make_call(
                                callable = ast_node.make_ident(name = "select"),
                                positional_args = [
                                    ast_node.make_dict(
                                        entries = [
                                            ast_node.make_key_value(
                                                key = ast_node.make_string(value = "//platforms/linux_x86"),
                                                value = ast_node.make_list(
                                                    elements = [
                                                        ast_node.make_string(value = "//select:32bits"),
                                                    ],
                                                ),
                                            ),
                                            ast_node.make_key_value(
                                                key = ast_node.make_string(value = "@platforms//os:windows"),
                                                value = ast_node.make_list(
                                                    elements = [
                                                        ast_node.make_string(value = "//select:64bits"),
                                                        ast_node.make_string(value = "//select:non_unix"),
                                                        ast_node.make_string(value = "//select:win"),
                                                    ],
                                                ),
                                            ),
                                            ast_node.make_key_value(
                                                key = ast_node.make_string(value = "//conditions:default"),
                                                value = ast_node.make_list(
                                                    elements = [],
                                                ),
                                            ),
                                        ],
                                    ),
                                ],
                                keyword_args = [],
                            ),
                        ),
                    ),
                ],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _ternary_expression_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    first = my_list[0] if len(my_list) > 0 else None
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "first"),
                op = "=",
                right = ast_node.make_ternary_op(
                    condition = ast_node.make_binary_op(
                        left = ast_node.make_call(
                            callable = ast_node.make_ident(name = "len"),
                            positional_args = [ast_node.make_ident(name = "my_list")],
                            keyword_args = [],
                        ),
                        op = ">",
                        right = ast_node.make_number(value = "0"),
                    ),
                    true_expr = ast_node.make_index(
                        object = ast_node.make_ident(name = "my_list"),
                        index = ast_node.make_number(value = "0"),
                    ),
                    false_expr = ast_node.make_ident(name = "None"),
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _parenthesis_expression_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    total = (a + b) * c
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "total"),
                op = "=",
                right = ast_node.make_binary_op(
                    left = ast_node.make_parenthesis(
                        expr = ast_node.make_binary_op(
                            left = ast_node.make_ident(name = "a"),
                            op = "+",
                            right = ast_node.make_ident(name = "b"),
                        ),
                    ),
                    op = "*",
                    right = ast_node.make_ident(name = "c"),
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _bitwise_operators_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    r = a & b | c ^ d << e >> ~~f
    """

    # Should be parsed as:
    #
    # r = ((a & b) | (c ^ ((d << e) >> (~(~f)))))
    #
    #   (=)
    #   / \
    # (r) (|)
    #     /   \
    #   (&)   (^)
    #   / \     \
    # (a) (b)   (>>)
    #           /   \
    #         (<<)   (~)
    #         /  \     \
    #       (d)  (e)   (~)
    #                   \
    #                   (f)

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "r"),
                op = "=",
                right = ast_node.make_binary_op(
                    left = ast_node.make_binary_op(
                        left = ast_node.make_ident(name = "a"),
                        op = "&",
                        right = ast_node.make_ident(name = "b"),
                    ),
                    op = "|",
                    right = ast_node.make_binary_op(
                        left = ast_node.make_ident(name = "c"),
                        op = "^",
                        right = ast_node.make_binary_op(
                            left = ast_node.make_binary_op(
                                left = ast_node.make_ident(name = "d"),
                                op = "<<",
                                right = ast_node.make_ident(name = "e"),
                            ),
                            op = ">>",
                            right = ast_node.make_unary_op(
                                op = "~",
                                operand = ast_node.make_unary_op(
                                    op = "~",
                                    operand = ast_node.make_ident(name = "f"),
                                ),
                            ),
                        ),
                    ),
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _get_attribute_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    CONSTANT = my_struct.attribute.sub_attribute
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "CONSTANT"),
                op = "=",
                right = ast_node.make_attr(
                    object = ast_node.make_attr(
                        object = ast_node.make_ident(name = "my_struct"),
                        attr = "attribute",
                    ),
                    attr = "sub_attribute",
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _list_comprehension_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    [x * x for x in range(10)]
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_list(
                elements = [
                    ast_node.make_comprehension(
                        element = ast_node.make_binary_op(
                            left = ast_node.make_ident(name = "x"),
                            op = "*",
                            right = ast_node.make_ident(name = "x"),
                        ),
                        loop_var = ast_node.make_ident(name = "x"),
                        iterable = ast_node.make_call(
                            callable = ast_node.make_ident(name = "range"),
                            positional_args = [ast_node.make_number(value = "10")],
                            keyword_args = [],
                        ),
                        condition = None,
                    ),
                ],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _list_comprehension_filtered_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    [x * x for x in range(10) if x % 2 == 0]
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_list(
                elements = [
                    ast_node.make_comprehension(
                        element = ast_node.make_binary_op(
                            left = ast_node.make_ident(name = "x"),
                            op = "*",
                            right = ast_node.make_ident(name = "x"),
                        ),
                        loop_var = ast_node.make_ident(name = "x"),
                        iterable = ast_node.make_call(
                            callable = ast_node.make_ident(name = "range"),
                            positional_args = [ast_node.make_number(value = "10")],
                            keyword_args = [],
                        ),
                        condition = ast_node.make_binary_op(
                            left = ast_node.make_binary_op(
                                left = ast_node.make_ident(name = "x"),
                                op = "%",
                                right = ast_node.make_number(value = "2"),
                            ),
                            op = "==",
                            right = ast_node.make_number(value = "0"),
                        ),
                    ),
                ],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _dict_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    my_dict = {
        "key1": "value1",
        "key2": "value2",
    }
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "my_dict"),
                op = "=",
                right = ast_node.make_dict(
                    entries = [
                        ast_node.make_key_value(
                            key = ast_node.make_string(value = "key1"),
                            value = ast_node.make_string(value = "value1"),
                        ),
                        ast_node.make_key_value(
                            key = ast_node.make_string(value = "key2"),
                            value = ast_node.make_string(value = "value2"),
                        ),
                    ],
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _tuple_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    empty_tuple = ()
    string = ("not_a_tuple")
    tuple_single_element = ("i_am_a_tuple",)
    tuple_multiple_elements = (1, "two", 3)
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "empty_tuple"),
                op = "=",
                right = ast_node.make_tuple(
                    elements = [],
                ),
            ),
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "string"),
                op = "=",
                right = ast_node.make_parenthesis(
                    expr = ast_node.make_string(value = "not_a_tuple"),
                ),
            ),
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "tuple_single_element"),
                op = "=",
                right = ast_node.make_tuple(
                    elements = [
                        ast_node.make_string(value = "i_am_a_tuple"),
                    ],
                ),
            ),
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "tuple_multiple_elements"),
                op = "=",
                right = ast_node.make_tuple(
                    elements = [
                        ast_node.make_number(value = "1"),
                        ast_node.make_string(value = "two"),
                        ast_node.make_number(value = "3"),
                    ],
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _dict_comprehension_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    {k: v for k, v in iterable}
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_dict(
                entries = [
                    ast_node.make_comprehension(
                        element = ast_node.make_key_value(
                            key = ast_node.make_ident(name = "k"),
                            value = ast_node.make_ident(name = "v"),
                        ),
                        loop_var = ast_node.make_tuple(
                            elements = [
                                ast_node.make_ident(name = "k"),
                                ast_node.make_ident(name = "v"),
                            ],
                        ),
                        iterable = ast_node.make_ident(name = "iterable"),
                        condition = None,
                    ),
                ],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _dict_comprehension_filtered_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    {k: v for k, v in iterable if k != "skip"}
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_dict(
                entries = [
                    ast_node.make_comprehension(
                        element = ast_node.make_key_value(
                            key = ast_node.make_ident(name = "k"),
                            value = ast_node.make_ident(name = "v"),
                        ),
                        loop_var = ast_node.make_tuple(
                            elements = [
                                ast_node.make_ident(name = "k"),
                                ast_node.make_ident(name = "v"),
                            ],
                        ),
                        iterable = ast_node.make_ident(name = "iterable"),
                        condition = ast_node.make_binary_op(
                            left = ast_node.make_ident(name = "k"),
                            op = "!=",
                            right = ast_node.make_string(value = "skip"),
                        ),
                    ),
                ],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _newline_statement_separator_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    "we"
    "are"
    "separated"

    ("so")
    ("as")
    ("we")
    """

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_string(value = "we"),
            ast_node.make_string(value = "are"),
            ast_node.make_string(value = "separated"),
            ast_node.make_parenthesis(
                expr = ast_node.make_string(value = "so"),
            ),
            ast_node.make_parenthesis(
                expr = ast_node.make_string(value = "as"),
            ),
            ast_node.make_parenthesis(
                expr = ast_node.make_string(value = "we"),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _readme_example_1_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    cc_library(
        name = "example",
        srcs = ["example.cc"],
    )
    """

    ast = parse(content)

    asserts.equals(env, ast_node_types.ROOT, ast.nodeType)
    asserts.equals(env, ast_node_types.CALL, ast.statements[0].nodeType)
    asserts.equals(env, "cc_library", ast.statements[0].callable.name)

    return unittest.end(env)

def _readme_example_2_test_impl(ctx):
    env = unittest.begin(ctx)

    def extract_function_names(ast):
        """Extract all function names called in a BUILD file."""
        asserts.equals(env, ast_node_types.ROOT, ast.nodeType)

        names = []
        for stmt in ast.statements:
            if stmt.nodeType == ast_node_types.CALL:
                if stmt.callable.nodeType == ast_node_types.IDENT:
                    names.append(stmt.callable.name)

        return names

    # Example usage
    ast = parse('load("@rules_cc//cc:defs.bzl", "cc_library")\ncc_library(name = "foo")')
    names = extract_function_names(ast)
    asserts.equals(env, ["load", "cc_library"], names)

    return unittest.end(env)

def _readme_example_3_test_impl(ctx):
    env = unittest.begin(ctx)

    def find_target_names(ast):
        """Find all 'name' attributes in function calls."""
        names = []

        for stmt in ast.statements:
            if stmt.nodeType == ast_node_types.CALL:
                for kwarg in stmt.keyword_args:
                    if kwarg.key == "name" and kwarg.value.nodeType == ast_node_types.STRING:
                        names.append(kwarg.value.value)

        return names

    ast = parse('cc_library(name = "mylib")\ncc_test(name = "mytest")')
    target_names = find_target_names(ast)
    asserts.equals(env, ["mylib", "mytest"], target_names)

    return unittest.end(env)

def _readme_example_4_test_impl(ctx):
    env = unittest.begin(ctx)

    content = "x = 1 + 2 * 3"

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "x"),
                op = "=",
                right = ast_node.make_binary_op(
                    left = ast_node.make_number(value = "1"),
                    op = "+",
                    right = ast_node.make_binary_op(
                        left = ast_node.make_number(value = "2"),
                        op = "*",
                        right = ast_node.make_number(value = "3"),
                    ),
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _readme_example_5_test_impl(ctx):
    env = unittest.begin(ctx)

    content = "srcs = [f + '.cc' for f in files if f != 'main']"

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "srcs"),
                op = "=",
                right = ast_node.make_list(
                    elements = [
                        ast_node.make_comprehension(
                            element = ast_node.make_binary_op(
                                left = ast_node.make_ident(name = "f"),
                                op = "+",
                                right = ast_node.make_string(value = ".cc"),
                            ),
                            loop_var = ast_node.make_ident(name = "f"),
                            iterable = ast_node.make_ident(name = "files"),
                            condition = ast_node.make_binary_op(
                                left = ast_node.make_ident(name = "f"),
                                op = "!=",
                                right = ast_node.make_string(value = "main"),
                            ),
                        ),
                    ],
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _buildtools_testdata_001_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """cc_test(name="bar",size="small",srcs=["b.cc","a.cc","c.cc"],deps=["//base",":foo","//util:map-util"], data = [ "datum" ], datum = [ "data", ])"""

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_call(
                callable = ast_node.make_ident(name = "cc_test"),
                positional_args = [],
                keyword_args = [
                    ast_node.make_key_value(
                        key = "name",
                        value = ast_node.make_string(value = "bar"),
                    ),
                    ast_node.make_key_value(
                        key = "size",
                        value = ast_node.make_string(value = "small"),
                    ),
                    ast_node.make_key_value(
                        key = "srcs",
                        value = ast_node.make_list(
                            elements = [
                                ast_node.make_string(value = "b.cc"),
                                ast_node.make_string(value = "a.cc"),
                                ast_node.make_string(value = "c.cc"),
                            ],
                        ),
                    ),
                    ast_node.make_key_value(
                        key = "deps",
                        value = ast_node.make_list(
                            elements = [
                                ast_node.make_string(value = "//base"),
                                ast_node.make_string(value = ":foo"),
                                ast_node.make_string(value = "//util:map-util"),
                            ],
                        ),
                    ),
                    ast_node.make_key_value(
                        key = "data",
                        value = ast_node.make_list(
                            elements = [
                                ast_node.make_string(value = "datum"),
                            ],
                        ),
                    ),
                    ast_node.make_key_value(
                        key = "datum",
                        value = ast_node.make_list(
                            elements = [
                                ast_node.make_string(value = "data"),
                            ],
                        ),
                    ),
                ],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _buildtools_testdata_002_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """cc_test ( name = 'b\\"ar\\'"' , srcs = [ 'a.cc' , "b.cc" , "c.cc" ] , size = "small" , deps = [ "//base" , ":foo", "//util:map-util", ] )"""

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_call(
                callable = ast_node.make_ident(name = "cc_test"),
                positional_args = [],
                keyword_args = [
                    ast_node.make_key_value(
                        key = "name",
                        value = ast_node.make_string(value = 'b\"ar\'"'),
                    ),
                    ast_node.make_key_value(
                        key = "srcs",
                        value = ast_node.make_list(
                            elements = [
                                ast_node.make_string(value = "a.cc"),
                                ast_node.make_string(value = "b.cc"),
                                ast_node.make_string(value = "c.cc"),
                            ],
                        ),
                    ),
                    ast_node.make_key_value(
                        key = "size",
                        value = ast_node.make_string(value = "small"),
                    ),
                    ast_node.make_key_value(
                        key = "deps",
                        value = ast_node.make_list(
                            elements = [
                                ast_node.make_string(value = "//base"),
                                ast_node.make_string(value = ":foo"),
                                ast_node.make_string(value = "//util:map-util"),
                            ],
                        ),
                    ),
                ],
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _buildtools_testdata_003_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """numbers = [
  0,
  11,
  123.456,
  123.,
  .456,
  1.23e45,
  -1,
  +1,
  0.0,
  -0.0,
  1.0,
  -1.0,
  +1.0,
  1e6,
  -1e6,
  -1.23e-45,
  3.539537889086625e+24,
  3.539537889086625E+24,
  3.539537889086625e00024000,
  3.539537889086625e+00024000,
  3539537889086624823140625,
  0x123,
  0xE45,
  0xe45,
]"""

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "numbers"),
                op = "=",
                right = ast_node.make_list(
                    elements = [
                        ast_node.make_number(value = "0"),
                        ast_node.make_number(value = "11"),
                        ast_node.make_number(value = "123.456"),
                        ast_node.make_number(value = "123."),
                        ast_node.make_number(value = ".456"),
                        ast_node.make_number(value = "1.23e45"),
                        ast_node.make_unary_op(
                            op = "-",
                            operand = ast_node.make_number(value = "1"),
                        ),
                        ast_node.make_unary_op(
                            op = "+",
                            operand = ast_node.make_number(value = "1"),
                        ),
                        ast_node.make_number(value = "0.0"),
                        ast_node.make_unary_op(
                            op = "-",
                            operand = ast_node.make_number(value = "0.0"),
                        ),
                        ast_node.make_number(value = "1.0"),
                        ast_node.make_unary_op(
                            op = "-",
                            operand = ast_node.make_number(value = "1.0"),
                        ),
                        ast_node.make_unary_op(
                            op = "+",
                            operand = ast_node.make_number(value = "1.0"),
                        ),
                        ast_node.make_number(value = "1e6"),
                        ast_node.make_unary_op(
                            op = "-",
                            operand = ast_node.make_number(value = "1e6"),
                        ),
                        ast_node.make_unary_op(
                            op = "-",
                            operand = ast_node.make_number(value = "1.23e-45"),
                        ),
                        ast_node.make_number(value = "3.539537889086625e+24"),
                        ast_node.make_number(value = "3.539537889086625E+24"),
                        ast_node.make_number(value = "3.539537889086625e00024000"),
                        ast_node.make_number(value = "3.539537889086625e+00024000"),
                        ast_node.make_number(value = "3539537889086624823140625"),
                        ast_node.make_number(value = "0x123"),
                        ast_node.make_number(value = "0xE45"),
                        ast_node.make_number(value = "0xe45"),
                    ],
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _buildtools_testdata_004_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """JAVA_FILES = [ "Foo.java", "Bar.java",
               "Baz.java", "Quux.java"
             ]"""

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "JAVA_FILES"),
                op = "=",
                right = ast_node.make_list(
                    elements = [
                        ast_node.make_string(value = "Foo.java"),
                        ast_node.make_string(value = "Bar.java"),
                        ast_node.make_string(value = "Baz.java"),
                        ast_node.make_string(value = "Quux.java"),
                    ],
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

def _buildtools_testdata_005_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """JAVA_FILES = [
    # Comment regarding Foo.java
    "Foo.java",
    "Bar.java",
    "Baz.java",  # Comment regarding Baz.java
    "Quux.java"
]"""

    expected_ast = ast_node.make_root(
        statements = [
            ast_node.make_binary_op(
                left = ast_node.make_ident(name = "JAVA_FILES"),
                op = "=",
                right = ast_node.make_list(
                    elements = [
                        ast_node.make_string(value = "Foo.java"),
                        ast_node.make_string(value = "Bar.java"),
                        ast_node.make_string(value = "Baz.java"),
                        ast_node.make_string(value = "Quux.java"),
                    ],
                ),
            ),
        ],
    )

    actual_ast = parse(content)
    asserts.equals(env, expected_ast, actual_ast)

    return unittest.end(env)

simple_call_test = unittest.make(_simple_call_test_impl)
load_statement_test = unittest.make(_load_statement_test_impl)
multiple_statements_test = unittest.make(_multiple_statements_test_impl)
string_concatenation_test = unittest.make(_string_concatenation_test_impl)
higher_order_function_test = unittest.make(_higher_order_function_test_impl)
binary_operators_priorities_test = unittest.make(_binary_operators_priorities_test_impl)
glob_expression_test = unittest.make(_glob_expression_test_impl)
select_expression_test = unittest.make(_select_expression_test_impl)
ternary_expression_test = unittest.make(_ternary_expression_test_impl)
parenthesis_expression_test = unittest.make(_parenthesis_expression_test_impl)
bitwise_operators_test = unittest.make(_bitwise_operators_test_impl)
get_attribute_test = unittest.make(_get_attribute_test_impl)
list_comprehension_test = unittest.make(_list_comprehension_test_impl)
list_comprehension_filtered_test = unittest.make(_list_comprehension_filtered_test_impl)
dict_test = unittest.make(_dict_test_impl)
tuple_test = unittest.make(_tuple_test_impl)
dict_comprehension_test = unittest.make(_dict_comprehension_test_impl)
dict_comprehension_filtered_test = unittest.make(_dict_comprehension_filtered_test_impl)
newline_statement_separator_test = unittest.make(_newline_statement_separator_test_impl)
readme_example_1_test = unittest.make(_readme_example_1_test_impl)
readme_example_2_test = unittest.make(_readme_example_2_test_impl)
readme_example_3_test = unittest.make(_readme_example_3_test_impl)
readme_example_4_test = unittest.make(_readme_example_4_test_impl)
readme_example_5_test = unittest.make(_readme_example_5_test_impl)
buildtools_testdata_001_test = unittest.make(_buildtools_testdata_001_test_impl)
buildtools_testdata_002_test = unittest.make(_buildtools_testdata_002_test_impl)
buildtools_testdata_003_test = unittest.make(_buildtools_testdata_003_test_impl)
buildtools_testdata_004_test = unittest.make(_buildtools_testdata_004_test_impl)
buildtools_testdata_005_test = unittest.make(_buildtools_testdata_005_test_impl)

def parser_test_suite(name):
    unittest.suite(
        name,
        simple_call_test,
        load_statement_test,
        multiple_statements_test,
        string_concatenation_test,
        higher_order_function_test,
        binary_operators_priorities_test,
        glob_expression_test,
        select_expression_test,
        ternary_expression_test,
        parenthesis_expression_test,
        bitwise_operators_test,
        get_attribute_test,
        list_comprehension_test,
        list_comprehension_filtered_test,
        dict_test,
        tuple_test,
        dict_comprehension_test,
        dict_comprehension_filtered_test,
        newline_statement_separator_test,

        # Examples from README.md
        readme_example_1_test,
        readme_example_2_test,
        readme_example_3_test,
        readme_example_4_test,
        readme_example_5_test,

        # Include some examples from https://github.com/bazelbuild/buildtools/tree/main/build/testdata
        buildtools_testdata_001_test,
        buildtools_testdata_002_test,
        buildtools_testdata_003_test,
        buildtools_testdata_004_test,
        buildtools_testdata_005_test,
    )
