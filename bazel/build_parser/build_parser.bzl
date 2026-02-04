"""
Simple BUILD file parser implementation in pure Starlark.

Inspired by the Go implementation in https://github.com/bazelbuild/buildtools.

This is a simplified parser that can parse basic BUILD file constructs without
tracking comments or detailed position information.
"""

load("//bazel/build_parser/internal:parser.bzl", _parse = "parse")
load("//bazel/build_parser/internal:syntax.bzl", _ast_node_types = "ast_node_types")

parse = _parse
ast_node_types = _ast_node_types
