# BUILD File Parser

A pure Starlark implementation of a BUILD file parser inspired by the Go implementation in [bazelbuild/buildtools](https://github.com/bazelbuild/buildtools).

The parser supports only a simplified grammar subset of Starlark applicable to BUILD files (expressions, function calls, literals, etc.). It does not support function definitions (`def`), loops (`for`, `while`), or other control flow statements typically found in `*.bzl` files.

## Features

- **Pure Starlark Implementation**: Platform-independent and can run anywhere Bazel/Starlark runs
- **Repository Rule Compatible**: Can be used in repository rules for BUILD file manipulation and analysis
- **No External Dependencies**: Self-contained parser with no dependencies on native code or external tools
- **AST Generation**: Produces a structured Abstract Syntax Tree (AST) for BUILD file content
- **Fail-Fast Error Handling**: Expects correct BUILD file syntax and fails immediately on syntax errors

## Usage

```starlark
load("//bazel/build_parser:build_parser.bzl", "parse", "ast_node_types")

def _repository_rule_impl(repository_ctx):
    # Read BUILD file content from the repository
    build_content = repository_ctx.read(repository_ctx.attr.build_file)

    # Example content:
    # cc_library(
    #     name = "mylib",
    #     srcs = ["mylib.cc"],
    #     hdrs = ["mylib.h"],
    # )

    ast = parse(build_content)
    # Process the AST...
```

## AST Node Types

The parser produces an AST where each node has a `nodeType` field. The available node types are defined in `ast_node_types`:

| Node Type       | Description                               | Fields                                                                                                                                                 |
| --------------- | ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `ROOT`          | Root node containing all statements       | `statements` - list of top-level statement nodes                                                                                                       |
| `CALL`          | Function call expression                  | `callable` - function being called<br>`positional_args` - list of positional arguments<br>`keyword_args` - list of keyword arguments (KEY_VALUE nodes) |
| `IDENT`         | Identifier (variable/function name)       | `name` - identifier string                                                                                                                             |
| `STRING`        | String literal                            | `value` - string value                                                                                                                                 |
| `NUMBER`        | Numeric literal                           | `value` - numeric value                                                                                                                                |
| `LIST`          | List literal `[...]`                      | `elements` - list of element nodes                                                                                                                     |
| `DICT`          | Dictionary literal `{...}`                | `entries` - list of KEY_VALUE nodes                                                                                                                    |
| `TUPLE`         | Tuple literal `(...)`                     | `elements` - list of element nodes                                                                                                                     |
| `KEY_VALUE`     | Dictionary entry or keyword argument      | `key` - key expression/string<br>`value` - value expression                                                                                            |
| `BINARY_OP`     | Binary operation (e.g., `+`, `-`, `*`)    | `left` - left operand<br>`op` - operator string<br>`right` - right operand                                                                             |
| `UNARY_OP`      | Unary operation (e.g., `not`, `-`)        | `op` - operator string<br>`operand` - operand expression                                                                                               |
| `TERNARY_OP`    | Conditional expression `x if cond else y` | `condition` - condition expression<br>`true_expr` - expression if true<br>`false_expr` - expression if false                                           |
| `ATTR`          | Attribute access `obj.attr`               | `object` - object expression<br>`attr` - attribute name string                                                                                         |
| `INDEX`         | Index operation `obj[index]`              | `object` - object expression<br>`index` - index expression                                                                                             |
| `PARENTHESIS`   | Parenthesized expression `(expr)`         | `expr` - wrapped expression                                                                                                                            |
| `COMPREHENSION` | List/dict comprehension                   | `element` - element expression<br>`loop_var` - loop variable(s)<br>`iterable` - iterable expression<br>`condition` - optional filter condition         |

## Examples

### Basic Parsing

```starlark
load("//bazel/build_parser:build_parser.bzl", "parse", "ast_node_types")

content = """
cc_library(
    name = "example",
    srcs = ["example.cc"],
)
"""

ast = parse(content)

# ast.nodeType == ast_node_types.ROOT
# ast.statements[0].nodeType == ast_node_types.CALL
# ast.statements[0].callable.name == "cc_library"
```

### Extracting Function Calls

```starlark
def extract_function_names(ast):
    """Extract all function names called in a BUILD file."""
    if ast.nodeType != ast_node_types.ROOT:
        fail("Expected ROOT node")

    names = []
    for stmt in ast.statements:
        if stmt.nodeType == ast_node_types.CALL:
            if stmt.callable.nodeType == ast_node_types.IDENT:
                names.append(stmt.callable.name)

    return names

# Example usage
ast = parse('load("@rules_cc//cc:defs.bzl", "cc_library")\ncc_library(name = "foo")')
names = extract_function_names(ast)  # ["load", "cc_library"]
```

### Finding Target Names

```starlark
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
target_names = find_target_names(ast)  # ["mylib", "mytest"]
```

### Parse expressions with operators

```starlark
ast = parse("x = 1 + 2 * 3")
```

Produces AST (operator precedence respected):

```
ROOT
└── BINARY_OP (=)
    ├── left: IDENT (x)
    └── right: BINARY_OP (+)
        ├── left: NUMBER (1)
        └── right: BINARY_OP (*)
            ├── left: NUMBER (2)
            └── right: NUMBER (3)
```

### Parse list comprehensions

```starlark
ast = parse("srcs = [f + '.cc' for f in files if f != 'main']")
```

Produces AST:

```
ROOT
└── BINARY_OP (=)
    ├── left: IDENT (srcs)
    └── right: LIST
        └── COMPREHENSION
            ├── element: BINARY_OP (+)
            │   ├── left: IDENT (f)
            │   └── right: STRING ('.cc')
            ├── loop_var: IDENT (f)
            ├── iterable: IDENT (files)
            └── condition: BINARY_OP (!=)
                ├── left: IDENT (f)
                └── right: STRING ('main')
```

## Implementation Notes

### Starlark Limitations

Starlark has several limitations compared to full Python that affect parser implementation:

1. **No `while` loops**: Starlark only supports `for` loops with finite iterables
2. **No recursion**: Recursive function calls are not allowed
3. **No mutable closures**: Nested functions cannot modify variables from outer scopes

### Design Patterns

To work around these limitations, the parser uses:

1. **Bounded iteration**: `while True` is emulated through `utils.infinite_loop()`, see `internal/utils.bzl` for details
2. **Explicit call stack**: Instead of recursive descent parsing, we maintain an explicit `call_stack` list that simulates function calls
3. **Mutable references**: Variables that need to be modified in nested functions use `utils.ref_make()` which wraps values in single-element lists to enable mutation (e.g., `index_ref = utils.ref_make(0)`, then access via `utils.ref_get(index_ref)`)

## Limitations

This is a simplified parser that:

- **Grammar subset**: Only supports BUILD file syntax (expressions, literals, function calls). Does not support function definitions (`def`), loops (`for`, `while`), `if` statements, or other control flow found in `.bzl` files
- Does not track source positions or line/column numbers
- Does not preserve or track comments
- Focuses on parsing syntax, not validating Bazel semantics
- May not support every edge case of Starlark syntax
