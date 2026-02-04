"""
Internal implementation of a simple parser for BUILD files in Starlark.
"""

load(":lexer.bzl", "token_types", "tokenize")
load(":syntax.bzl", "ast_node", "ast_node_types", "op_precedence")
load(":utils.bzl", "utils")

def _parse_impl(tokens):
    """Parse a list of tokens into an AST.

    Args:
        tokens: A list of tokens to parse, returned by the 'tokenize' function.

    Returns:
        An AST representing the parsed BUILD file. The returned root node is always of type ast_node_types.ROOT.
    """

    # Index of the next token to be processed.
    #
    # This is always a single-element list to allow mutation inside nested
    # functions. Starlark closure is not as handy as in e.g. Go, so this is
    # a workaround.
    index = [0]

    # Explicit stacks to avoid recursion.
    #
    # Recursion is not allowed in Starlark, so we use explicit stacks to manage
    # the parsing process using only ordinary iteration.
    call_stack = []
    result_stack = []

    def has_token(ignore_newline = True):
        """Check if there are more tokens to process.

        Args:
            ignore_newline: If True, skip newline tokens.

        Returns:
            True if there are more tokens, False otherwise.
        """
        if ignore_newline and index[0] < len(tokens) and tokens[index[0]].tokenType == token_types.NEWLINE:
            index[0] += 1
        return index[0] < len(tokens)

    def peek_token(ignore_newline = True):
        """Peek the next token without consuming it.

        Args:
            ignore_newline: If True, skip newline tokens.

        Returns:
            The next token, or None if at the end of the token list.
        """
        return tokens[index[0]] if has_token(ignore_newline) else None

    def consume_token(ignore_newline = True):
        """Consume and return the next token.

        Increments the current token index.

        Args:
            ignore_newline: If True, skip newline tokens.

        Returns:
            The next token or None if at the end of the token list.
        """
        token = peek_token(ignore_newline)
        if token != None:
            index[0] += 1
        return token

    def expect_token_type(expected_type, ignore_newline = True):
        """Consume the next token and ensure it is of the expected type.

        Increments the current token index.

        Args:
            expected_type: The expected token type.
            ignore_newline: If True, skip newline tokens.

        Returns:
            The value of the consumed token or fails if the token type does not
            match.
        """
        token = consume_token(ignore_newline)
        if not token or token.tokenType != expected_type:
            found = token.tokenType if token else "EOF"
            fail("expected %s but got %s" % (expected_type, found))
        return token.value

    def revert_token_index(old_index):
        """Revert the token index to a previous value.

        Args:
            old_index: The previous token index to revert to.
        """
        if old_index > index[0]:
            fail("cannot revert to a future index")

        index[0] = old_index

    def get_operator_precedence(token_type):
        """Get precedence level for an operator token type."""
        if token_type == token_types.OPERATOR_ASSIGN:
            return op_precedence.LOWEST
        elif token_type == token_types.LOGICAL_OR:
            return op_precedence.OR
        elif token_type == token_types.LOGICAL_AND:
            return op_precedence.AND
        elif token_type in [
            token_types.OPERATOR_EQUAL,
            token_types.OPERATOR_NOT_EQUAL,
            token_types.OPERATOR_LESS,
            token_types.OPERATOR_GREATER,
            token_types.OPERATOR_LESS_OR_EQUAL,
            token_types.OPERATOR_GREATER_OR_EQUAL,
        ]:
            return op_precedence.COMPARE
        elif token_type == token_types.IN:
            return op_precedence.COMPARE
        elif token_type == token_types.BITWISE_OR:
            return op_precedence.PIPE
        elif token_type == token_types.BITWISE_XOR:
            return op_precedence.XOR
        elif token_type == token_types.BITWISE_AND:
            return op_precedence.AMPERSAND
        elif token_type in [token_types.BITWISE_SHIFT_LEFT, token_types.BITWISE_SHIFT_RIGHT]:
            return op_precedence.SHIFT
        elif token_type in [token_types.PLUS, token_types.MINUS]:
            return op_precedence.ADD
        elif token_type in [token_types.ASTERISK, token_types.SLASH, token_types.DOUBLE_SLASH, token_types.MOD]:
            return op_precedence.MULTIPLY
        return 0

    def is_binary_operator(token_type):
        """Check if token type is a binary operator."""
        return get_operator_precedence(token_type) > 0

    def parse_atom():
        """Parse atomic expression: string, number, or identifier."""
        token = peek_token()
        if not token:
            fail("unexpected end of input")

        if token.tokenType == token_types.LITERAL_STRING:
            consume_token()
            result_stack.append(ast_node.makeString(value = token.value))
        elif token.tokenType == token_types.LITERAL_NUMBER:
            consume_token()
            result_stack.append(ast_node.makeNumber(value = int(token.value)))
        elif token.tokenType == token_types.IDENT:
            consume_token()
            result_stack.append(ast_node.makeIdent(name = token.value))
        else:
            fail("unexpected token: %s" % token.tokenType)

    def parse_list_elements():
        """Parse list elements and push result onto stack."""
        elements = []
        token = peek_token()

        if token and token.tokenType == token_types.BRACKET_RIGHT:
            expect_token_type(token_types.BRACKET_RIGHT)
            result_stack.append(ast_node.makeList(elements = elements))
            return

        # Parse first element
        call_stack.append(struct(function = parse_list_collect, args = {"elements": elements}))
        call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))

    def parse_list_collect(elements):
        """Collect one list element from result stack."""
        elements.append(result_stack.pop())

        token = peek_token()
        if token and token.tokenType == token_types.COMMA:
            consume_token()
            token = peek_token()

        if token and token.tokenType == token_types.BRACKET_RIGHT:
            expect_token_type(token_types.BRACKET_RIGHT)
            result_stack.append(ast_node.makeList(elements = elements))
        elif token and token.tokenType == token_types.FOR:
            # This is a comprehension, not a regular list
            # Don't consume FOR, let parse_primary_check_comprehension handle it
            result_stack.append(ast_node.makeList(elements = elements))
        else:
            # Parse next element
            call_stack.append(struct(function = parse_list_collect, args = {"elements": elements}))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))

    def parse_list():
        """Parse list literal [...]."""
        expect_token_type(token_types.BRACKET_LEFT)
        parse_list_elements()

    def parse_dict_entries():
        """Parse dict entries and push result onto stack."""
        entries = []
        token = peek_token()

        if token and token.tokenType == token_types.BRACE_RIGHT:
            expect_token_type(token_types.BRACE_RIGHT)
            result_stack.append(ast_node.makeDict(entries = entries))
            return

        # Parse first key
        call_stack.append(struct(function = parse_dict_key_collected, args = {"entries": entries}))
        call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))

    def parse_dict_key_collected(entries):
        """Collect dict key, expect colon, then parse value."""
        key = result_stack.pop()
        expect_token_type(token_types.COLON)

        call_stack.append(struct(function = parse_dict_value_collected, args = {"entries": entries, "key": key}))
        call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))

    def parse_dict_value_collected(entries, key):
        """Collect dict value and create key-value pair."""
        value = result_stack.pop()
        entries.append(ast_node.makeKeyValue(key = key, value = value))

        token = peek_token()
        if token and token.tokenType == token_types.COMMA:
            consume_token()
            token = peek_token()

        if token and token.tokenType == token_types.BRACE_RIGHT:
            expect_token_type(token_types.BRACE_RIGHT)
            result_stack.append(ast_node.makeDict(entries = entries))
        elif token and token.tokenType == token_types.FOR:
            # This is a dict comprehension, not a regular dict
            # Don't consume FOR, let parse_dict_check_comprehension handle it
            result_stack.append(ast_node.makeDict(entries = entries))
        else:
            # Parse next key
            call_stack.append(struct(function = parse_dict_key_collected, args = {"entries": entries}))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))

    def parse_dict():
        """Parse dict literal {...}."""
        expect_token_type(token_types.BRACE_LEFT)
        call_stack.append(struct(function = parse_dict_check_comprehension, args = {}))
        parse_dict_entries()

    def parse_dict_check_comprehension():
        """After parsing dict, check if it's actually a dict comprehension."""

        # If next token is 'for', convert dict to comprehension
        token = peek_token()
        if token and token.tokenType == token_types.FOR:
            dict_node = result_stack.pop()
            if dict_node.nodeType != ast_node_types.DICT or len(dict_node.entries) != 1:
                fail("invalid dict comprehension syntax")

            element = dict_node.entries[0]
            consume_token()  # consume 'for'

            # Parse loop variable(s) - could be a tuple without parentheses
            first_var_token = expect_token_type(token_types.IDENT)
            first_var = ast_node.makeIdent(name = first_var_token)

            # Check if this is a tuple (comma-separated variables)
            token = peek_token()
            if token and token.tokenType == token_types.COMMA:
                # It's a tuple
                consume_token()
                var_elements = [first_var]

                # Parse remaining variables
                for err in utils.infinite_loop():
                    if err:
                        fail("exceeded maximum parsing iterations")

                    var_token = expect_token_type(token_types.IDENT)
                    var_elements.append(ast_node.makeIdent(name = var_token))

                    token = peek_token()
                    if token and token.tokenType == token_types.COMMA:
                        consume_token()
                    else:
                        break

                loop_var = ast_node.makeTuple(elements = var_elements)
            else:
                # Single variable
                loop_var = first_var

            # Expect 'in'
            expect_token_type(token_types.IN)

            # Parse iterable (use OR precedence to skip ternary operator parsing)
            call_stack.append(struct(function = parse_dict_comprehension_iterable_collected, args = {
                "element": element,
                "loop_var": loop_var,
            }))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.OR}))

    def parse_dict_comprehension_iterable_collected(element, loop_var):
        """After collecting iterable for dict comprehension, check for optional 'if' condition."""
        iterable = result_stack.pop()

        # Check for optional 'if' condition
        token = peek_token()
        if token and token.tokenType == token_types.IF:
            consume_token()
            call_stack.append(struct(function = parse_dict_comprehension_condition_collected, args = {
                "element": element,
                "loop_var": loop_var,
                "iterable": iterable,
            }))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.OR}))
        else:
            expect_token_type(token_types.BRACE_RIGHT)
            comprehension = ast_node.makeComprehension(
                element = element,
                loop_var = loop_var,
                iterable = iterable,
                condition = None,
            )
            result_stack.append(ast_node.makeDict(entries = [comprehension]))

    def parse_dict_comprehension_condition_collected(element, loop_var, iterable):
        """Collect dict comprehension condition and close brace."""
        condition = result_stack.pop()
        expect_token_type(token_types.BRACE_RIGHT)
        comprehension = ast_node.makeComprehension(
            element = element,
            loop_var = loop_var,
            iterable = iterable,
            condition = condition,
        )
        result_stack.append(ast_node.makeDict(entries = [comprehension]))

    def parse_primary():
        """Parse primary expression (atom, list, dict, or parenthesis)."""
        token = peek_token()
        if not token:
            fail("unexpected end of input")

        if token.tokenType == token_types.BRACKET_LEFT:
            call_stack.append(struct(function = parse_primary_check_comprehension, args = {}))
            parse_list()
        elif token.tokenType == token_types.BRACE_LEFT:
            parse_dict()
        elif token.tokenType == token_types.PARENTHESIS_LEFT:
            consume_token()

            # Check for empty tuple ()
            next_token = peek_token()
            if next_token and next_token.tokenType == token_types.PARENTHESIS_RIGHT:
                expect_token_type(token_types.PARENTHESIS_RIGHT)
                result_stack.append(ast_node.makeTuple(elements = []))
            else:
                # Parse first element, then check if tuple or parenthesis
                call_stack.append(struct(function = parse_parenthesis_or_tuple_check, args = {}))
                call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))
        elif token.tokenType in [token_types.LOGICAL_NOT, token_types.MINUS, token_types.PLUS, token_types.BITWISE_NOT]:
            # Unary operators
            op_token = consume_token()
            call_stack.append(struct(function = parse_unary_collected, args = {"op": op_token.value}))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.UNARY}))
        else:
            parse_atom()

    def parse_primary_check_comprehension():
        """After parsing list, check if it's actually a comprehension."""

        # If next token is 'for', convert list to comprehension
        token = peek_token()
        if token and token.tokenType == token_types.FOR:
            list_node = result_stack.pop()
            if list_node.nodeType != ast_node_types.LIST or len(list_node.elements) != 1:
                fail("invalid comprehension syntax")

            element = list_node.elements[0]
            consume_token()  # consume 'for'

            # Parse loop variable(s) - could be a tuple without parentheses
            first_var_token = expect_token_type(token_types.IDENT)
            first_var = ast_node.makeIdent(name = first_var_token)

            # Check if this is a tuple (comma-separated variables)
            token = peek_token()
            if token and token.tokenType == token_types.COMMA:
                # It's a tuple
                consume_token()
                var_elements = [first_var]

                # Parse remaining variables
                for err in utils.infinite_loop():
                    if err:
                        fail("exceeded maximum parsing iterations")

                    var_token = expect_token_type(token_types.IDENT)
                    var_elements.append(ast_node.makeIdent(name = var_token))

                    token = peek_token()
                    if token and token.tokenType == token_types.COMMA:
                        consume_token()
                    else:
                        break

                loop_var = ast_node.makeTuple(elements = var_elements)
            else:
                # Single variable
                loop_var = first_var

            # Expect 'in'
            expect_token_type(token_types.IN)

            # Parse iterable (use OR precedence to skip ternary operator parsing)
            call_stack.append(struct(function = parse_comprehension_iterable_collected, args = {
                "element": element,
                "loop_var": loop_var,
            }))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.OR}))

    def parse_comprehension_iterable_collected(element, loop_var):
        """After collecting iterable, check for optional 'if' condition."""
        iterable = result_stack.pop()

        # Check for optional 'if' condition
        token = peek_token()
        if token and token.tokenType == token_types.IF:
            consume_token()
            call_stack.append(struct(function = parse_comprehension_condition_collected, args = {
                "element": element,
                "loop_var": loop_var,
                "iterable": iterable,
            }))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.OR}))
        else:
            expect_token_type(token_types.BRACKET_RIGHT)
            comprehension = ast_node.makeComprehension(
                element = element,
                loop_var = loop_var,
                iterable = iterable,
                condition = None,
            )
            result_stack.append(ast_node.makeList(elements = [comprehension]))

    def parse_comprehension_condition_collected(element, loop_var, iterable):
        """Collect comprehension condition and close bracket."""
        condition = result_stack.pop()
        expect_token_type(token_types.BRACKET_RIGHT)
        comprehension = ast_node.makeComprehension(
            element = element,
            loop_var = loop_var,
            iterable = iterable,
            condition = condition,
        )
        result_stack.append(ast_node.makeList(elements = [comprehension]))

    def parse_parenthesis_or_tuple_check():
        """After parsing first element, check if tuple or parenthesis."""
        token = peek_token()

        if token and token.tokenType == token_types.COMMA:
            # It's a tuple - collect elements
            consume_token()
            elements = [result_stack.pop()]

            # Check if there are more elements
            next_token = peek_token()
            if next_token and next_token.tokenType == token_types.PARENTHESIS_RIGHT:
                # Single element tuple with trailing comma: (x,)
                expect_token_type(token_types.PARENTHESIS_RIGHT)
                result_stack.append(ast_node.makeTuple(elements = elements))
            else:
                # Multi-element tuple
                call_stack.append(struct(function = parse_tuple_collect, args = {"elements": elements}))
                call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))
        elif token and token.tokenType == token_types.PARENTHESIS_RIGHT:
            # Single element without comma - it's a parenthesis
            expect_token_type(token_types.PARENTHESIS_RIGHT)
            expr = result_stack.pop()
            result_stack.append(ast_node.makeParenthesis(expr = expr))
        else:
            fail("expected comma or closing parenthesis")

    def parse_tuple_collect(elements):
        """Collect one tuple element from result stack."""
        elements.append(result_stack.pop())

        token = peek_token()
        if token and token.tokenType == token_types.COMMA:
            consume_token()
            token = peek_token()

        if token and token.tokenType == token_types.PARENTHESIS_RIGHT:
            expect_token_type(token_types.PARENTHESIS_RIGHT)
            result_stack.append(ast_node.makeTuple(elements = elements))
        else:
            # Parse next element
            call_stack.append(struct(function = parse_tuple_collect, args = {"elements": elements}))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))

    def parse_unary_collected(op):
        """Collect operand for unary operator."""
        operand = result_stack.pop()
        result_stack.append(ast_node.makeUnaryOp(op = op, operand = operand))

    def parse_call_args():
        """Parse function call arguments."""
        positional_args = []
        keyword_args = []

        token = peek_token()
        if token and token.tokenType == token_types.PARENTHESIS_RIGHT:
            result_stack.append({"positional_args": positional_args, "keyword_args": keyword_args})
            return

        # Check if first arg is keyword
        saved_index = index[0]
        is_keyword = False
        if token and token.tokenType == token_types.IDENT:
            consume_token()
            next_token = peek_token()
            if next_token and next_token.tokenType == token_types.OPERATOR_ASSIGN:
                is_keyword = True
        revert_token_index(saved_index)

        if is_keyword:
            # Parse keyword argument
            key = expect_token_type(token_types.IDENT)
            expect_token_type(token_types.OPERATOR_ASSIGN)
            call_stack.append(struct(function = parse_call_collect_arg, args = {
                "positional_args": positional_args,
                "keyword_args": keyword_args,
                "pending_key": key,
            }))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))
        else:
            # Parse positional argument
            call_stack.append(struct(function = parse_call_collect_arg, args = {
                "positional_args": positional_args,
                "keyword_args": keyword_args,
                "pending_key": None,
            }))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))

    def parse_call_collect_arg(positional_args, keyword_args, pending_key):
        """Collect one function argument."""
        value = result_stack.pop()

        if pending_key:
            keyword_args.append(ast_node.makeKeyValue(key = pending_key, value = value))
        else:
            positional_args.append(value)

        token = peek_token()
        if token and token.tokenType == token_types.COMMA:
            consume_token()
            token = peek_token()

        if token and token.tokenType == token_types.PARENTHESIS_RIGHT:
            result_stack.append({"positional_args": positional_args, "keyword_args": keyword_args})
        else:
            # Check if next arg is keyword
            saved_index = index[0]
            is_keyword = False
            if token and token.tokenType == token_types.IDENT:
                consume_token()
                next_token = peek_token()
                if next_token and next_token.tokenType == token_types.OPERATOR_ASSIGN:
                    is_keyword = True
            revert_token_index(saved_index)

            if is_keyword:
                key = expect_token_type(token_types.IDENT)
                expect_token_type(token_types.OPERATOR_ASSIGN)
                call_stack.append(struct(function = parse_call_collect_arg, args = {
                    "positional_args": positional_args,
                    "keyword_args": keyword_args,
                    "pending_key": key,
                }))
                call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))
            else:
                call_stack.append(struct(function = parse_call_collect_arg, args = {
                    "positional_args": positional_args,
                    "keyword_args": keyword_args,
                    "pending_key": None,
                }))
                call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))

    def parse_postfix(min_precedence):
        """Check for postfix operators (calls, indexing, etc.)."""
        token = peek_token(ignore_newline = False)

        if token and token.tokenType == token_types.PARENTHESIS_LEFT and op_precedence.CALL >= min_precedence:
            # Function call
            consume_token()
            call_stack.append(struct(function = parse_postfix, args = {"min_precedence": min_precedence}))
            call_stack.append(struct(function = parse_call_finalize, args = {}))
            call_stack.append(struct(function = parse_call_args, args = {}))
        elif token and token.tokenType == token_types.BRACKET_LEFT and op_precedence.CALL >= min_precedence:
            # Index operation
            consume_token()
            call_stack.append(struct(function = parse_postfix, args = {"min_precedence": min_precedence}))
            call_stack.append(struct(function = parse_index_finalize, args = {}))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))
        elif token and token.tokenType == token_types.DOT and op_precedence.CALL >= min_precedence:
            # Attribute access
            consume_token()
            call_stack.append(struct(function = parse_postfix, args = {"min_precedence": min_precedence}))
            call_stack.append(struct(function = parse_attr_finalize, args = {}))
        else:
            # Check for ternary operator
            call_stack.append(struct(function = parse_ternary, args = {"min_precedence": min_precedence}))

    def parse_call_finalize():
        """Finalize function call with collected args."""
        expect_token_type(token_types.PARENTHESIS_RIGHT)
        args_dict = result_stack.pop()
        callable = result_stack.pop()
        result_stack.append(ast_node.makeCall(
            callable = callable,
            positional_args = args_dict["positional_args"],
            keyword_args = args_dict["keyword_args"],
        ))

    def parse_index_finalize():
        """Finalize index operation with collected index expression."""
        expect_token_type(token_types.BRACKET_RIGHT)
        index_expr = result_stack.pop()
        object_expr = result_stack.pop()
        result_stack.append(ast_node.makeIndex(object = object_expr, index = index_expr))

    def parse_attr_finalize():
        """Finalize attribute access operation."""
        attr_name = expect_token_type(token_types.IDENT)
        object_expr = result_stack.pop()
        result_stack.append(ast_node.makeAttr(object = object_expr, attr = attr_name))

    def parse_ternary(min_precedence):
        """Check for ternary conditional operator (x if cond else y)."""
        token = peek_token()

        if token and token.tokenType == token_types.IF and op_precedence.TERNARY >= min_precedence:
            consume_token()  # consume 'if'

            # Parse condition
            call_stack.append(struct(function = parse_ternary_condition_collected, args = {"min_precedence": min_precedence}))
            call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.OR}))  # Higher than TERNARY
        else:
            # Check for binary operators
            call_stack.append(struct(function = parse_binary, args = {"min_precedence": min_precedence}))

    def parse_ternary_condition_collected(min_precedence):
        """Collect condition and expect 'else' keyword."""
        condition = result_stack.pop()

        # Expect 'else' - check if it's a keyword token or identifier
        token = peek_token()
        is_else = False
        if token:
            if hasattr(token_types, "ELSE") and token.tokenType == token_types.ELSE:
                is_else = True
            elif token.tokenType == token_types.IDENT and token.value == "else":
                is_else = True

        if not is_else:
            fail("expected 'else' in ternary expression")
        consume_token()

        # Parse false expression
        call_stack.append(struct(function = parse_ternary_finalize, args = {"condition": condition, "min_precedence": min_precedence}))
        call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.TERNARY}))

    def parse_ternary_finalize(condition, min_precedence):
        """Finalize ternary operation with all three expressions."""
        false_expr = result_stack.pop()
        true_expr = result_stack.pop()
        result_stack.append(ast_node.makeTernaryOp(condition = condition, true_expr = true_expr, false_expr = false_expr))

        # Continue checking for more operators at this precedence level
        call_stack.append(struct(function = parse_ternary, args = {"min_precedence": min_precedence}))

    def parse_binary(min_precedence):
        """Parse binary operators with precedence climbing."""
        token = peek_token()

        if token and is_binary_operator(token.tokenType):
            op_precedence = get_operator_precedence(token.tokenType)
            if op_precedence >= min_precedence:
                op_token = consume_token()

                # For right-associative operators like **, use op_precedence
                # For left-associative, use op_precedence + 1
                next_precedence = op_precedence + 1
                if token.tokenType == token_types.DOUBLE_ASTERISK:
                    next_precedence = op_precedence

                call_stack.append(struct(function = parse_binary_collected, args = {
                    "op": op_token.value,
                    "min_precedence": min_precedence,
                }))
                call_stack.append(struct(function = parse_expression, args = {"min_precedence": next_precedence}))
                return

        # No more operators at this precedence level

    def parse_binary_collected(op, min_precedence):
        """Collect right operand and create binary expression."""
        right = result_stack.pop()
        left = result_stack.pop()
        result_stack.append(ast_node.makeBinaryOp(left = left, op = op, right = right))

        # Continue parsing operators at same precedence
        call_stack.append(struct(function = parse_binary, args = {"min_precedence": min_precedence}))

    def parse_expression(min_precedence):
        """Parse expression with operator precedence."""
        call_stack.append(struct(function = parse_postfix, args = {"min_precedence": min_precedence}))
        call_stack.append(struct(function = parse_primary, args = {}))

    def parse_statement():
        """Parse one statement."""
        call_stack.append(struct(function = parse_expression, args = {"min_precedence": op_precedence.LOWEST}))

    def parse_root_begin():
        """Begin parsing root node."""
        statements = []
        call_stack.append(struct(function = parse_root_end, args = {"statements": statements}))
        call_stack.append(struct(function = parse_root_statements, args = {"statements": statements}))

    def parse_root_statements(statements):
        """Parse all statements in root."""
        if not has_token(ignore_newline = False):
            return

        call_stack.append(struct(function = parse_root_collect_statement, args = {"statements": statements}))
        call_stack.append(struct(function = parse_statement, args = {}))

    def parse_root_collect_statement(statements):
        """Collect one statement and continue parsing."""
        statements.append(result_stack.pop())

        if has_token(ignore_newline = False):
            call_stack.append(struct(function = parse_root_collect_statement, args = {"statements": statements}))
            call_stack.append(struct(function = parse_statement, args = {}))

    def parse_root_end(statements):
        """Finalize root node."""
        result_stack.append(ast_node.makeRoot(statements = statements))

    call_stack.append(struct(function = parse_root_begin, args = {}))
    for err in utils.infinite_loop():
        if err:
            fail("exceeded maximum parsing iterations")

        if len(call_stack) == 0:
            break

        call = call_stack.pop()
        call.function(**call.args)

    if len(result_stack) != 1:
        fail("expected single root node in result stack, got %d items" % len(result_stack))

    return result_stack.pop()

def parse(content):
    """Parse BUILD file content into an AST. Fails fast on syntax errors.

    Args:
        content: The BUILD file content as a string.

    Returns:
        An AST representing the parsed BUILD file.
    """
    tokens = tokenize(content)
    return _parse_impl(tokens)
