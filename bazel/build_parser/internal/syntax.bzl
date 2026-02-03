"""
List of supported AST node types and constructors for them.
"""

load(":utils.bzl", "utils")

ast_node_types = utils.enum_type(
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

def _makeAttr(*, object, attr):
    """Attribute access: object.attr."""
    return struct(
        nodeType = ast_node_types.ATTR,
        object = object,
        attr = attr,
    )

def _makeBinaryOp(*, left, op, right):
    """Binary expression: left op right."""
    return struct(
        nodeType = ast_node_types.BINARY_OP,
        left = left,
        op = op,
        right = right,
    )

def _makeCall(*, callable, positional_args, keyword_args):
    """Function call expression: func(args, key=value)."""
    return struct(
        nodeType = ast_node_types.CALL,
        callable = callable,
        positional_args = positional_args,
        keyword_args = keyword_args,
    )

def _makeComprehension(*, element, loop_var, iterable, condition = None):
    """List comprehension: [element for loop_var in iterable if condition]."""
    return struct(
        nodeType = ast_node_types.COMPREHENSION,
        element = element,
        loop_var = loop_var,
        iterable = iterable,
        condition = condition,
    )

def _makeDict(*, entries):
    """Dictionary literal: {...}."""
    return struct(
        nodeType = ast_node_types.DICT,
        entries = entries,
    )

def _makeIdent(*, name):
    """Identifier."""
    return struct(
        nodeType = ast_node_types.IDENT,
        name = name,
    )

def _makeIndex(*, object, index):
    """Index expression: object[index]."""
    return struct(
        nodeType = ast_node_types.INDEX,
        object = object,
        index = index,
    )

def _makeKeyValue(*, key, value):
    """Dictionary entry: 'key: value' or keyword argument 'key=value'."""
    return struct(
        nodeType = ast_node_types.KEY_VALUE,
        key = key,
        value = value,
    )

def _makeList(*, elements):
    """List literal: [...]."""
    return struct(
        nodeType = ast_node_types.LIST,
        elements = elements,
    )

def _makeNumber(*, value):
    """Number literal."""
    return struct(
        nodeType = ast_node_types.NUMBER,
        value = value,
    )

def _makeParenthesis(*, expr):
    """Parenthesized expression: (expr)."""
    return struct(
        nodeType = ast_node_types.PARENTHESIS,
        expr = expr,
    )

def _makeRoot(*, statements):
    """Root node containing all statements."""
    return struct(
        nodeType = ast_node_types.ROOT,
        statements = statements,
    )

def _makeString(*, value):
    """String literal."""
    return struct(
        nodeType = ast_node_types.STRING,
        value = value,
    )

def _makeTernaryOp(*, condition, true_expr, false_expr):
    """Ternary/conditional expression: true_expr if condition else false_expr."""
    return struct(
        nodeType = ast_node_types.TERNARY_OP,
        condition = condition,
        true_expr = true_expr,
        false_expr = false_expr,
    )

def _makeTuple(*, elements):
    """Tuple literal: (...)."""
    return struct(
        nodeType = ast_node_types.TUPLE,
        elements = elements,
    )

def _makeUnaryOp(*, op, operand):
    """Unary expression: op operand (e.g., not x, -x)."""
    return struct(
        nodeType = ast_node_types.UNARY_OP,
        op = op,
        operand = operand,
    )

ast_node = struct(
    makeAttr = _makeAttr,
    makeBinaryOp = _makeBinaryOp,
    makeCall = _makeCall,
    makeComprehension = _makeComprehension,
    makeDict = _makeDict,
    makeIdent = _makeIdent,
    makeIndex = _makeIndex,
    makeKeyValue = _makeKeyValue,
    makeList = _makeList,
    makeNumber = _makeNumber,
    makeParenthesis = _makeParenthesis,
    makeRoot = _makeRoot,
    makeString = _makeString,
    makeTernaryOp = _makeTernaryOp,
    makeTuple = _makeTuple,
    makeUnaryOp = _makeUnaryOp,
)

# Operator precedence levels (higher = tighter binding)
op_precedence = struct(
    LOWEST = 1,
    TERNARY = 2,  # if/else
    OR = 3,  # or
    AND = 4,  # and
    NOT = 5,  # not
    COMPARE = 6,  # ==, !=, <, >, <=, >=, in, not in
    PIPE = 7,  # |
    XOR = 8,  # ^
    AMPERSAND = 9,  # &
    SHIFT = 10,  # <<, >>
    ADD = 11,  # +, -
    MULTIPLY = 12,  # *, /, //, %
    UNARY = 13,  # +x, -x, ~x
    CALL = 14,  # f(), x[i], x.attr
)
