"""
List of supported AST node types and constructors for them.
"""

load(":utils.bzl", "utils")

ast_node_types = utils.enum(
    "ATTR",
    "BINARY_OP",
    "CALL",
    "COMPREHENSION",
    "DICT",
    "IDENT",
    "INDEX",
    "KEY_VALUE",
    "LIST",
    "NUMBER",
    "PARENTHESIS",
    "ROOT",
    "STRING",
    "TERNARY_OP",
    "TUPLE",
    "UNARY_OP",
)

def _make_attr(*, object, attr):
    """Attribute access: object.attr."""
    return struct(
        nodeType = ast_node_types.ATTR,
        object = object,
        attr = attr,
    )

def _make_binary_op(*, left, op, right):
    """Binary expression: left op right."""
    return struct(
        nodeType = ast_node_types.BINARY_OP,
        left = left,
        op = op,
        right = right,
    )

def _make_call(*, callable, positional_args, keyword_args):
    """Function call expression: func(args, key=value)."""
    return struct(
        nodeType = ast_node_types.CALL,
        callable = callable,
        positional_args = positional_args,
        keyword_args = keyword_args,
    )

def _make_comprehension(*, element, loop_var, iterable, condition = None):
    """List comprehension: [element for loop_var in iterable if condition]."""
    return struct(
        nodeType = ast_node_types.COMPREHENSION,
        element = element,
        loop_var = loop_var,
        iterable = iterable,
        condition = condition,
    )

def _make_dict(*, entries):
    """Dictionary literal: {...}."""
    return struct(
        nodeType = ast_node_types.DICT,
        entries = entries,
    )

def _make_ident(*, name):
    """Identifier."""
    return struct(
        nodeType = ast_node_types.IDENT,
        name = name,
    )

def _make_index(*, object, index):
    """Index expression: object[index]."""
    return struct(
        nodeType = ast_node_types.INDEX,
        object = object,
        index = index,
    )

def _make_key_value(*, key, value):
    """Dictionary entry: 'key: value' or keyword argument 'key=value'."""
    return struct(
        nodeType = ast_node_types.KEY_VALUE,
        key = key,
        value = value,
    )

def _make_list(*, elements):
    """List literal: [...]."""
    return struct(
        nodeType = ast_node_types.LIST,
        elements = elements,
    )

def _make_number(*, value):
    """Number literal."""
    return struct(
        nodeType = ast_node_types.NUMBER,
        value = value,
    )

def _make_parenthesis(*, expr):
    """Parenthesized expression: (expr)."""
    return struct(
        nodeType = ast_node_types.PARENTHESIS,
        expr = expr,
    )

def _make_root(*, statements):
    """Root node containing all statements."""
    return struct(
        nodeType = ast_node_types.ROOT,
        statements = statements,
    )

def _make_string(*, value):
    """String literal."""
    return struct(
        nodeType = ast_node_types.STRING,
        value = value,
    )

def _make_ternary_op(*, condition, true_expr, false_expr):
    """Ternary/conditional expression: true_expr if condition else false_expr."""
    return struct(
        nodeType = ast_node_types.TERNARY_OP,
        condition = condition,
        true_expr = true_expr,
        false_expr = false_expr,
    )

def _make_tuple(*, elements):
    """Tuple literal: (...)."""
    return struct(
        nodeType = ast_node_types.TUPLE,
        elements = elements,
    )

def _make_unary_op(*, op, operand):
    """Unary expression: op operand (e.g., not x, -x)."""
    return struct(
        nodeType = ast_node_types.UNARY_OP,
        op = op,
        operand = operand,
    )

ast_node = struct(
    make_attr = _make_attr,
    make_binary_op = _make_binary_op,
    make_call = _make_call,
    make_comprehension = _make_comprehension,
    make_dict = _make_dict,
    make_ident = _make_ident,
    make_index = _make_index,
    make_key_value = _make_key_value,
    make_list = _make_list,
    make_number = _make_number,
    make_parenthesis = _make_parenthesis,
    make_root = _make_root,
    make_string = _make_string,
    make_ternary_op = _make_ternary_op,
    make_tuple = _make_tuple,
    make_unary_op = _make_unary_op,
)

# Operator precedence levels (higher = tighter binding)
op_precedence = utils.int_enum(
    "LOWEST",
    "ASSIGN",  # =
    "TERNARY",  # if/else
    "OR",  # or
    "AND",  # and
    "NOT",  # not
    "COMPARE",  # ==, !=, <, >, <=, >=, in, not in
    "PIPE",  # |
    "XOR",  # ^
    "AMPERSAND",  # &
    "SHIFT",  # <<, >>
    "ADD",  # +, -
    "MULTIPLY",  # *, /, //, %
    "UNARY",  # +x, -x, ~x
    "CALL",  # f(), x[i], x.attr
)
