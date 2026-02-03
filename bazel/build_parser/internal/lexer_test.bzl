"""
Unit tests for the BUILD file lexer.
"""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load(":lexer.bzl", "make_token", "token_types", "tokenize")

def _simple_call_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    cc_library(
        name = "mylib",
        srcs = ["mylib.cc"],
        hdrs = ["mylib.h"],
    )
    """
    tokens = tokenize(content)

    asserts.equals(env, [
        make_token(tokenType = token_types.IDENT, value = "cc_library"),
        make_token(tokenType = token_types.PARENTHESIS_LEFT, value = "("),
        make_token(tokenType = token_types.IDENT, value = "name"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.LITERAL_STRING, value = "mylib"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.IDENT, value = "srcs"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.BRACKET_LEFT, value = "["),
        make_token(tokenType = token_types.LITERAL_STRING, value = "mylib.cc"),
        make_token(tokenType = token_types.BRACKET_RIGHT, value = "]"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.IDENT, value = "hdrs"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.BRACKET_LEFT, value = "["),
        make_token(tokenType = token_types.LITERAL_STRING, value = "mylib.h"),
        make_token(tokenType = token_types.BRACKET_RIGHT, value = "]"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = ")"),
    ], tokens)

    return unittest.end(env)

def _load_statement_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    load("@rules_cc//cc:defs.bzl", "cc_library", "cc_binary")
    """
    tokens = tokenize(content)

    asserts.equals(env, [
        make_token(tokenType = token_types.IDENT, value = "load"),
        make_token(tokenType = token_types.PARENTHESIS_LEFT, value = "("),
        make_token(tokenType = token_types.LITERAL_STRING, value = "@rules_cc//cc:defs.bzl"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.LITERAL_STRING, value = "cc_library"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.LITERAL_STRING, value = "cc_binary"),
        make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = ")"),
    ], tokens)

    return unittest.end(env)

def _assignment_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    MY_SRCS = ["a.cc", "b.cc"]
    MY_VALUE = 42
    MY_DICT = {"key1": "val1", "key2": "val2"}
    """
    tokens = tokenize(content)

    asserts.equals(env, [
        make_token(tokenType = token_types.IDENT, value = "MY_SRCS"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.BRACKET_LEFT, value = "["),
        make_token(tokenType = token_types.LITERAL_STRING, value = "a.cc"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.LITERAL_STRING, value = "b.cc"),
        make_token(tokenType = token_types.BRACKET_RIGHT, value = "]"),
        make_token(tokenType = token_types.IDENT, value = "MY_VALUE"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.LITERAL_NUMBER, value = "42"),
        make_token(tokenType = token_types.IDENT, value = "MY_DICT"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.BRACE_LEFT, value = "{"),
        make_token(tokenType = token_types.LITERAL_STRING, value = "key1"),
        make_token(tokenType = token_types.COLON, value = ":"),
        make_token(tokenType = token_types.LITERAL_STRING, value = "val1"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.LITERAL_STRING, value = "key2"),
        make_token(tokenType = token_types.COLON, value = ":"),
        make_token(tokenType = token_types.LITERAL_STRING, value = "val2"),
        make_token(tokenType = token_types.BRACE_RIGHT, value = "}"),
    ], tokens)

    return unittest.end(env)

def _comments_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    # This is a comment
    cc_library(  # inline comment
        name = "lib",  # another comment
    )
    # Final comment
    """
    tokens = tokenize(content)

    asserts.equals(env, [
        make_token(tokenType = token_types.IDENT, value = "cc_library"),
        make_token(tokenType = token_types.PARENTHESIS_LEFT, value = "("),
        make_token(tokenType = token_types.IDENT, value = "name"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.LITERAL_STRING, value = "lib"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = ")"),
    ], tokens)

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
    tokens = tokenize(content)

    asserts.equals(env, [
        make_token(tokenType = token_types.IDENT, value = "load"),
        make_token(tokenType = token_types.PARENTHESIS_LEFT, value = "("),
        make_token(tokenType = token_types.LITERAL_STRING, value = "@rules_cc//cc:defs.bzl"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.LITERAL_STRING, value = "cc_library"),
        make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = ")"),
        make_token(tokenType = token_types.IDENT, value = "SRCS"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.BRACKET_LEFT, value = "["),
        make_token(tokenType = token_types.LITERAL_STRING, value = "a.cc"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.LITERAL_STRING, value = "b.cc"),
        make_token(tokenType = token_types.BRACKET_RIGHT, value = "]"),
        make_token(tokenType = token_types.IDENT, value = "cc_library"),
        make_token(tokenType = token_types.PARENTHESIS_LEFT, value = "("),
        make_token(tokenType = token_types.IDENT, value = "name"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.LITERAL_STRING, value = "mylib"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.IDENT, value = "srcs"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.IDENT, value = "SRCS"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = ")"),
        make_token(tokenType = token_types.IDENT, value = "cc_library"),
        make_token(tokenType = token_types.PARENTHESIS_LEFT, value = "("),
        make_token(tokenType = token_types.IDENT, value = "name"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.LITERAL_STRING, value = "other"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.IDENT, value = "srcs"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.BRACKET_LEFT, value = "["),
        make_token(tokenType = token_types.LITERAL_STRING, value = "other.cc"),
        make_token(tokenType = token_types.BRACKET_RIGHT, value = "]"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = ")"),
    ], tokens)

    return unittest.end(env)

def _triple_quote_string_test_impl(ctx):
    env = unittest.begin(ctx)

    content = '''
    my_string = """This is a
    multi-line string
    with "quotes" and 'single quotes'."""
    '''
    tokens = tokenize(content)

    asserts.equals(env, [
        make_token(tokenType = token_types.IDENT, value = "my_string"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.LITERAL_STRING, value = 'This is a\n    multi-line string\n    with "quotes" and \'single quotes\'.'),
    ], tokens)

    return unittest.end(env)

def _get_item_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    load("@bazel_skylib//lib:paths.bzl", "paths")
    load("//:my_rules.bzl", "magic_rule")

    magic_rule(
        name = "magic",
        data = paths.join("data_dir", "file.txt"),
    )
    """
    tokens = tokenize(content)

    asserts.equals(env, [
        make_token(tokenType = token_types.IDENT, value = "load"),
        make_token(tokenType = token_types.PARENTHESIS_LEFT, value = "("),
        make_token(tokenType = token_types.LITERAL_STRING, value = "@bazel_skylib//lib:paths.bzl"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.LITERAL_STRING, value = "paths"),
        make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = ")"),
        make_token(tokenType = token_types.IDENT, value = "load"),
        make_token(tokenType = token_types.PARENTHESIS_LEFT, value = "("),
        make_token(tokenType = token_types.LITERAL_STRING, value = "//:my_rules.bzl"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.LITERAL_STRING, value = "magic_rule"),
        make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = ")"),
        make_token(tokenType = token_types.IDENT, value = "magic_rule"),
        make_token(tokenType = token_types.PARENTHESIS_LEFT, value = "("),
        make_token(tokenType = token_types.IDENT, value = "name"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.LITERAL_STRING, value = "magic"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.IDENT, value = "data"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.IDENT, value = "paths"),
        make_token(tokenType = token_types.DOT, value = "."),
        make_token(tokenType = token_types.IDENT, value = "join"),
        make_token(tokenType = token_types.PARENTHESIS_LEFT, value = "("),
        make_token(tokenType = token_types.LITERAL_STRING, value = "data_dir"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.LITERAL_STRING, value = "file.txt"),
        make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = ")"),
        make_token(tokenType = token_types.COMMA, value = ","),
        make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = ")"),
    ], tokens)

    return unittest.end(env)

def _escape_characters_in_string_test_impl(ctx):
    env = unittest.begin(ctx)

    content = r'''
    "Line1\nLine2\tTabbed\\Backslash\"DoubleQuote\'SingleQuote"
    '''
    tokens = tokenize(content)

    asserts.equals(env, [
        make_token(tokenType = token_types.LITERAL_STRING, value = 'Line1\nLine2\tTabbed\\Backslash"DoubleQuote\'SingleQuote'),
    ], tokens)

    return unittest.end(env)

def _list_comprehension_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    my_list = [x * 2 for x in range(10) if x % 2 == 0]
    """
    tokens = tokenize(content)

    asserts.equals(env, [
        make_token(tokenType = token_types.IDENT, value = "my_list"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.BRACKET_LEFT, value = "["),
        make_token(tokenType = token_types.IDENT, value = "x"),
        make_token(tokenType = token_types.ASTERISK, value = "*"),
        make_token(tokenType = token_types.LITERAL_NUMBER, value = "2"),
        make_token(tokenType = token_types.FOR, value = "for"),
        make_token(tokenType = token_types.IDENT, value = "x"),
        make_token(tokenType = token_types.IN, value = "in"),
        make_token(tokenType = token_types.IDENT, value = "range"),
        make_token(tokenType = token_types.PARENTHESIS_LEFT, value = "("),
        make_token(tokenType = token_types.LITERAL_NUMBER, value = "10"),
        make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = ")"),
        make_token(tokenType = token_types.IF, value = "if"),
        make_token(tokenType = token_types.IDENT, value = "x"),
        make_token(tokenType = token_types.MOD, value = "%"),
        make_token(tokenType = token_types.LITERAL_NUMBER, value = "2"),
        make_token(tokenType = token_types.OPERATOR_EQUAL, value = "=="),
        make_token(tokenType = token_types.LITERAL_NUMBER, value = "0"),
        make_token(tokenType = token_types.BRACKET_RIGHT, value = "]"),
    ], tokens)

    return unittest.end(env)

def _bitwise_operators_test_impl(ctx):
    env = unittest.begin(ctx)

    content = """
    result = a & b | c ^ d << e >> ~~f
    """
    tokens = tokenize(content)

    asserts.equals(env, [
        make_token(tokenType = token_types.IDENT, value = "result"),
        make_token(tokenType = token_types.OPERATOR_ASSIGN, value = "="),
        make_token(tokenType = token_types.IDENT, value = "a"),
        make_token(tokenType = token_types.BITWISE_AND, value = "&"),
        make_token(tokenType = token_types.IDENT, value = "b"),
        make_token(tokenType = token_types.BITWISE_OR, value = "|"),
        make_token(tokenType = token_types.IDENT, value = "c"),
        make_token(tokenType = token_types.BITWISE_XOR, value = "^"),
        make_token(tokenType = token_types.IDENT, value = "d"),
        make_token(tokenType = token_types.BITWISE_SHIFT_LEFT, value = "<<"),
        make_token(tokenType = token_types.IDENT, value = "e"),
        make_token(tokenType = token_types.BITWISE_SHIFT_RIGHT, value = ">>"),
        make_token(tokenType = token_types.BITWISE_NOT, value = "~"),
        make_token(tokenType = token_types.BITWISE_NOT, value = "~"),
        make_token(tokenType = token_types.IDENT, value = "f"),
    ], tokens)

    return unittest.end(env)

simple_call_test = unittest.make(_simple_call_test_impl)
load_statement_test = unittest.make(_load_statement_test_impl)
assignment_test = unittest.make(_assignment_test_impl)
comments_test = unittest.make(_comments_test_impl)
multiple_statements_test = unittest.make(_multiple_statements_test_impl)
triple_quote_string_test = unittest.make(_triple_quote_string_test_impl)
get_item_test = unittest.make(_get_item_test_impl)
escape_characters_in_string_test = unittest.make(_escape_characters_in_string_test_impl)
list_comprehension_test = unittest.make(_list_comprehension_test_impl)
bitwise_operators_test = unittest.make(_bitwise_operators_test_impl)

def lexer_test_suite(name):
    unittest.suite(
        name,
        simple_call_test,
        load_statement_test,
        assignment_test,
        comments_test,
        multiple_statements_test,
        triple_quote_string_test,
        get_item_test,
        escape_characters_in_string_test,
        list_comprehension_test,
        bitwise_operators_test,
    )
