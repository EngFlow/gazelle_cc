"""
Unit tests for the BUILD file parser.
"""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load(":parser.bzl", "parse")
load(":syntax.bzl", "ast_node")

def _simple_call_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    cc_library(
        name = "mylib",
        srcs = ["mylib.cc"],
        hdrs = ["mylib.h"],
    )
    """

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeCall(
                callable = ast_node.makeIdent(name = "cc_library"),
                positional_args = [],
                keyword_args = [
                    ast_node.makeKeyValue(
                        key = "name",
                        value = ast_node.makeString(value = "mylib"),
                    ),
                    ast_node.makeKeyValue(
                        key = "srcs",
                        value = ast_node.makeList(
                            elements = [ast_node.makeString(value = "mylib.cc")],
                        ),
                    ),
                    ast_node.makeKeyValue(
                        key = "hdrs",
                        value = ast_node.makeList(
                            elements = [ast_node.makeString(value = "mylib.h")],
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeCall(
                callable = ast_node.makeIdent(name = "load"),
                positional_args = [
                    ast_node.makeString(value = "@rules_cc//cc:defs.bzl"),
                    ast_node.makeString(value = "cc_library"),
                    ast_node.makeString(value = "cc_binary"),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeCall(
                callable = ast_node.makeIdent(name = "load"),
                positional_args = [
                    ast_node.makeString(value = "@rules_cc//cc:defs.bzl"),
                    ast_node.makeString(value = "cc_library"),
                ],
                keyword_args = [],
            ),
            ast_node.makeBinaryOp(
                left = ast_node.makeIdent(name = "SRCS"),
                op = "=",
                right = ast_node.makeList(
                    elements = [
                        ast_node.makeString(value = "a.cc"),
                        ast_node.makeString(value = "b.cc"),
                    ],
                ),
            ),
            ast_node.makeCall(
                callable = ast_node.makeIdent(name = "cc_library"),
                positional_args = [],
                keyword_args = [
                    ast_node.makeKeyValue(
                        key = "name",
                        value = ast_node.makeString(value = "mylib"),
                    ),
                    ast_node.makeKeyValue(
                        key = "srcs",
                        value = ast_node.makeIdent(name = "SRCS"),
                    ),
                ],
            ),
            ast_node.makeCall(
                callable = ast_node.makeIdent(name = "cc_library"),
                positional_args = [],
                keyword_args = [
                    ast_node.makeKeyValue(
                        key = "name",
                        value = ast_node.makeString(value = "other"),
                    ),
                    ast_node.makeKeyValue(
                        key = "srcs",
                        value = ast_node.makeList(
                            elements = [ast_node.makeString(value = "other.cc")],
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeBinaryOp(
                left = ast_node.makeBinaryOp(
                    left = ast_node.makeString(value = "a"),
                    op = "+",
                    right = ast_node.makeString(value = "b"),
                ),
                op = "+",
                right = ast_node.makeString(value = "c"),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeCall(
                callable = ast_node.makeCall(
                    callable = ast_node.makeIdent(name = "i_return_a_function"),
                    positional_args = [ast_node.makeIdent(name = "inner_expr")],
                    keyword_args = [],
                ),
                positional_args = [ast_node.makeIdent(name = "outer_expr")],
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeBinaryOp(
                left = ast_node.makeBinaryOp(
                    left = ast_node.makeNumber(value = 1),
                    op = "+",
                    right = ast_node.makeBinaryOp(
                        left = ast_node.makeNumber(value = 2),
                        op = "*",
                        right = ast_node.makeNumber(value = 3),
                    ),
                ),
                op = "-",
                right = ast_node.makeBinaryOp(
                    left = ast_node.makeNumber(value = 4),
                    op = "/",
                    right = ast_node.makeNumber(value = 5),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeCall(
                callable = ast_node.makeIdent(name = "cc_library"),
                positional_args = [],
                keyword_args = [
                    ast_node.makeKeyValue(
                        key = "name",
                        value = ast_node.makeString(value = "my_lib"),
                    ),
                    ast_node.makeKeyValue(
                        key = "srcs",
                        value = ast_node.makeCall(
                            callable = ast_node.makeIdent(name = "glob"),
                            positional_args = [
                                ast_node.makeList(
                                    elements = [
                                        ast_node.makeString(value = "*.c"),
                                        ast_node.makeString(value = "*.h"),
                                    ],
                                ),
                            ],
                            keyword_args = [
                                ast_node.makeKeyValue(
                                    key = "exclude",
                                    value = ast_node.makeList(
                                        elements = [
                                            ast_node.makeString(value = "*_test.cc"),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeCall(
                callable = ast_node.makeIdent(name = "cc_library"),
                positional_args = [],
                keyword_args = [
                    ast_node.makeKeyValue(
                        key = "name",
                        value = ast_node.makeString(value = "my_lib"),
                    ),
                    ast_node.makeKeyValue(
                        key = "deps",
                        value = ast_node.makeBinaryOp(
                            left = ast_node.makeList(
                                elements = [
                                    ast_node.makeString(value = "//shared:api"),
                                ],
                            ),
                            op = "+",
                            right = ast_node.makeCall(
                                callable = ast_node.makeIdent(name = "select"),
                                positional_args = [
                                    ast_node.makeDict(
                                        entries = [
                                            ast_node.makeKeyValue(
                                                key = ast_node.makeString(value = "//platforms/linux_x86"),
                                                value = ast_node.makeList(
                                                    elements = [
                                                        ast_node.makeString(value = "//select:32bits"),
                                                    ],
                                                ),
                                            ),
                                            ast_node.makeKeyValue(
                                                key = ast_node.makeString(value = "@platforms//os:windows"),
                                                value = ast_node.makeList(
                                                    elements = [
                                                        ast_node.makeString(value = "//select:64bits"),
                                                        ast_node.makeString(value = "//select:non_unix"),
                                                        ast_node.makeString(value = "//select:win"),
                                                    ],
                                                ),
                                            ),
                                            ast_node.makeKeyValue(
                                                key = ast_node.makeString(value = "//conditions:default"),
                                                value = ast_node.makeList(
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeBinaryOp(
                left = ast_node.makeIdent(name = "first"),
                op = "=",
                right = ast_node.makeTernaryOp(
                    condition = ast_node.makeBinaryOp(
                        left = ast_node.makeCall(
                            callable = ast_node.makeIdent(name = "len"),
                            positional_args = [ast_node.makeIdent(name = "my_list")],
                            keyword_args = [],
                        ),
                        op = ">",
                        right = ast_node.makeNumber(value = 0),
                    ),
                    true_expr = ast_node.makeIndex(
                        object = ast_node.makeIdent(name = "my_list"),
                        index = ast_node.makeNumber(value = 0),
                    ),
                    false_expr = ast_node.makeIdent(name = "None"),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeBinaryOp(
                left = ast_node.makeIdent(name = "total"),
                op = "=",
                right = ast_node.makeBinaryOp(
                    left = ast_node.makeParenthesis(
                        expr = ast_node.makeBinaryOp(
                            left = ast_node.makeIdent(name = "a"),
                            op = "+",
                            right = ast_node.makeIdent(name = "b"),
                        ),
                    ),
                    op = "*",
                    right = ast_node.makeIdent(name = "c"),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeBinaryOp(
                left = ast_node.makeIdent(name = "r"),
                op = "=",
                right = ast_node.makeBinaryOp(
                    left = ast_node.makeBinaryOp(
                        left = ast_node.makeIdent(name = "a"),
                        op = "&",
                        right = ast_node.makeIdent(name = "b"),
                    ),
                    op = "|",
                    right = ast_node.makeBinaryOp(
                        left = ast_node.makeIdent(name = "c"),
                        op = "^",
                        right = ast_node.makeBinaryOp(
                            left = ast_node.makeBinaryOp(
                                left = ast_node.makeIdent(name = "d"),
                                op = "<<",
                                right = ast_node.makeIdent(name = "e"),
                            ),
                            op = ">>",
                            right = ast_node.makeUnaryOp(
                                op = "~",
                                operand = ast_node.makeUnaryOp(
                                    op = "~",
                                    operand = ast_node.makeIdent(name = "f"),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeBinaryOp(
                left = ast_node.makeIdent(name = "CONSTANT"),
                op = "=",
                right = ast_node.makeAttr(
                    object = ast_node.makeAttr(
                        object = ast_node.makeIdent(name = "my_struct"),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeList(
                elements = [
                    ast_node.makeComprehension(
                        element = ast_node.makeBinaryOp(
                            left = ast_node.makeIdent(name = "x"),
                            op = "*",
                            right = ast_node.makeIdent(name = "x"),
                        ),
                        loop_var = ast_node.makeIdent(name = "x"),
                        iterable = ast_node.makeCall(
                            callable = ast_node.makeIdent(name = "range"),
                            positional_args = [ast_node.makeNumber(value = 10)],
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeList(
                elements = [
                    ast_node.makeComprehension(
                        element = ast_node.makeBinaryOp(
                            left = ast_node.makeIdent(name = "x"),
                            op = "*",
                            right = ast_node.makeIdent(name = "x"),
                        ),
                        loop_var = ast_node.makeIdent(name = "x"),
                        iterable = ast_node.makeCall(
                            callable = ast_node.makeIdent(name = "range"),
                            positional_args = [ast_node.makeNumber(value = 10)],
                            keyword_args = [],
                        ),
                        condition = ast_node.makeBinaryOp(
                            left = ast_node.makeBinaryOp(
                                left = ast_node.makeIdent(name = "x"),
                                op = "%",
                                right = ast_node.makeNumber(value = 2),
                            ),
                            op = "==",
                            right = ast_node.makeNumber(value = 0),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeBinaryOp(
                left = ast_node.makeIdent(name = "my_dict"),
                op = "=",
                right = ast_node.makeDict(
                    entries = [
                        ast_node.makeKeyValue(
                            key = ast_node.makeString(value = "key1"),
                            value = ast_node.makeString(value = "value1"),
                        ),
                        ast_node.makeKeyValue(
                            key = ast_node.makeString(value = "key2"),
                            value = ast_node.makeString(value = "value2"),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeBinaryOp(
                left = ast_node.makeIdent(name = "empty_tuple"),
                op = "=",
                right = ast_node.makeTuple(
                    elements = [],
                ),
            ),
            ast_node.makeBinaryOp(
                left = ast_node.makeIdent(name = "string"),
                op = "=",
                right = ast_node.makeParenthesis(
                    expr = ast_node.makeString(value = "not_a_tuple"),
                ),
            ),
            ast_node.makeBinaryOp(
                left = ast_node.makeIdent(name = "tuple_single_element"),
                op = "=",
                right = ast_node.makeTuple(
                    elements = [
                        ast_node.makeString(value = "i_am_a_tuple"),
                    ],
                ),
            ),
            ast_node.makeBinaryOp(
                left = ast_node.makeIdent(name = "tuple_multiple_elements"),
                op = "=",
                right = ast_node.makeTuple(
                    elements = [
                        ast_node.makeNumber(value = 1),
                        ast_node.makeString(value = "two"),
                        ast_node.makeNumber(value = 3),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeDict(
                entries = [
                    ast_node.makeComprehension(
                        element = ast_node.makeKeyValue(
                            key = ast_node.makeIdent(name = "k"),
                            value = ast_node.makeIdent(name = "v"),
                        ),
                        loop_var = ast_node.makeTuple(
                            elements = [
                                ast_node.makeIdent(name = "k"),
                                ast_node.makeIdent(name = "v"),
                            ],
                        ),
                        iterable = ast_node.makeIdent(name = "iterable"),
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

    expected_ast = ast_node.makeRoot(
        statements = [
            ast_node.makeDict(
                entries = [
                    ast_node.makeComprehension(
                        element = ast_node.makeKeyValue(
                            key = ast_node.makeIdent(name = "k"),
                            value = ast_node.makeIdent(name = "v"),
                        ),
                        loop_var = ast_node.makeTuple(
                            elements = [
                                ast_node.makeIdent(name = "k"),
                                ast_node.makeIdent(name = "v"),
                            ],
                        ),
                        iterable = ast_node.makeIdent(name = "iterable"),
                        condition = ast_node.makeBinaryOp(
                            left = ast_node.makeIdent(name = "k"),
                            op = "!=",
                            right = ast_node.makeString(value = "skip"),
                        ),
                    ),
                ],
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
    )
