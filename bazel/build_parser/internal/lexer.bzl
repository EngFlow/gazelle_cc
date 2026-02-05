"""
Internal implementation of a simple lexer for BUILD files in Starlark.
"""

load(":utils.bzl", "utils")

token_types = utils.enum(
    "ASTERISK",
    "BITWISE_AND",
    "BITWISE_NOT",
    "BITWISE_OR",
    "BITWISE_SHIFT_LEFT",
    "BITWISE_SHIFT_RIGHT",
    "BITWISE_XOR",
    "BRACE_LEFT",
    "BRACE_RIGHT",
    "BRACKET_LEFT",
    "BRACKET_RIGHT",
    "COLON",
    "COMMA",
    "DOT",
    "DOUBLE_ASTERISK",
    "DOUBLE_SLASH",
    "ELSE",
    "FOR",
    "IDENT",
    "IF",
    "IN",
    "LITERAL_NUMBER",
    "LITERAL_STRING",
    "LOGICAL_AND",
    "LOGICAL_NOT",
    "LOGICAL_OR",
    "MINUS",
    "MOD",
    "NEWLINE",
    "OPERATOR_ASSIGN",
    "OPERATOR_EQUAL",
    "OPERATOR_GREATER_OR_EQUAL",
    "OPERATOR_GREATER",
    "OPERATOR_LESS_OR_EQUAL",
    "OPERATOR_LESS",
    "OPERATOR_NOT_EQUAL",
    "PARENTHESIS_LEFT",
    "PARENTHESIS_RIGHT",
    "PLUS",
    "SLASH",
    "UNASSIGNED",
)

def make_token(*, tokenType, value):
    """Create a single token.

    Args:
        tokenType: The type of the token, one of token_types constants.
        value: The value or lexeme of the token.

    Returns:
        A struct representing a token with keys: 'tokenType' and 'value'.
    """
    return struct(
        tokenType = tokenType,
        value = value,
    )

def tokenize(content):
    """Tokenize BUILD file content into a list of tokens.

    Args:
      content: The BUILD file content as a string to be tokenized.

    Returns:
      A list of token structs, each containing 'tokenType' and 'value' fields.
    """
    tokens = []
    n = len(content)
    skip_until = 0  # Position to skip until (exclusive)

    for i in range(n):
        c = content[i]

        # Skip positions we've already processed
        if i < skip_until:
            continue

        # Skip whitespace (but handle newlines specially)
        if c in " \t\r":
            continue

        if c == "\n":
            # Join consecutive newlines
            skip_until = n  # Default to EOF
            for j in range(i + 1, n):
                if content[j] != "\n":
                    skip_until = j
                    break
            tokens.append(make_token(tokenType = token_types.NEWLINE, value = "\n" * (skip_until - i)))
            continue

        if c == "#":
            # Find end of line
            skip_until = n  # Default to end of file if no newline found
            for j in range(i + 1, n):
                if content[j] == "\n":
                    skip_until = j
                    break
            continue

        # String literals
        if c == '"' or c == "'":
            quote = c
            start = i + 1

            # Handle triple quotes
            if i + 2 < n and content[i + 1:i + 3] == quote + quote:
                quote = quote + quote + quote
                start = i + 3

            value = ""
            for j in range(start, n):
                # Skip positions we've already processed
                if j < skip_until:
                    continue

                # Is end of string?
                if content[j:j + len(quote)] == quote:
                    skip_until = j + len(quote)
                    break

                # Handle escape sequences
                if content[j] == "\\" and j + 1 < n:
                    next_char = content[j + 1]
                    if next_char == "n":
                        value += "\n"
                    elif next_char == "t":
                        value += "\t"
                    elif next_char == "r":
                        value += "\r"
                    elif next_char == "\\":
                        value += "\\"
                    elif next_char == '"':
                        value += '"'
                    elif next_char == "'":
                        value += "'"
                    else:
                        # Other characters after backslash: just include them
                        value += next_char

                    skip_until = j + 2

                else:
                    value += content[j]

            tokens.append(make_token(tokenType = token_types.LITERAL_STRING, value = value))
            continue

        # Numbers (including floats, scientific notation, and hex)
        # Note: +/- signs are handled by the parser as unary operators
        if c.isdigit() or (c == "." and i + 1 < n and content[i + 1].isdigit()):
            start = i
            end = i
            has_dot = False

            # Handle leading dot for floats like .456
            if content[end] == ".":
                has_dot = True
                end += 1

            # Check for hex numbers (0x...)
            if end < n and content[end] == "0" and end + 1 < n and (content[end + 1] == "x" or content[end + 1] == "X"):
                end += 2
                for j in range(end, n):
                    if content[j].isdigit() or content[j] in "abcdefABCDEF":
                        end = j + 1
                    else:
                        break
                tokens.append(make_token(tokenType = token_types.LITERAL_NUMBER, value = content[start:end]))
                skip_until = end
                continue

            # Parse integer or decimal part
            for j in range(end, n):
                if content[j].isdigit():
                    end = j + 1
                elif content[j] == "." and not has_dot:
                    has_dot = True
                    end = j + 1
                else:
                    break

            # Check for scientific notation (e or E)
            if end < n and (content[end] == "e" or content[end] == "E"):
                end += 1

                # Optional sign after e
                if end < n and (content[end] == "+" or content[end] == "-"):
                    end += 1

                # Exponent digits
                for j in range(end, n):
                    if content[j].isdigit():
                        end = j + 1
                    else:
                        break

            tokens.append(make_token(tokenType = token_types.LITERAL_NUMBER, value = content[start:end]))
            skip_until = end
            continue

        # Identifiers and keywords
        if c.isalpha() or c == "_":
            start = i
            end = i

            for j in range(i, n):
                if content[j].isalnum() or content[j] == "_":
                    end = j + 1
                else:
                    break

            word = content[start:end]

            # Check for keywords
            token_type = token_types.IDENT
            if word == "if":
                token_type = token_types.IF
            elif word == "else":
                token_type = token_types.ELSE
            elif word == "for":
                token_type = token_types.FOR
            elif word == "in":
                token_type = token_types.IN
            elif word == "and":
                token_type = token_types.LOGICAL_AND
            elif word == "or":
                token_type = token_types.LOGICAL_OR
            elif word == "not":
                token_type = token_types.LOGICAL_NOT

            tokens.append(make_token(tokenType = token_type, value = word))
            skip_until = end
            continue

        # Double character tokens
        if i + 1 < n:
            two_char = content[i:i + 2]
            if two_char == "==":
                tokens.append(make_token(tokenType = token_types.OPERATOR_EQUAL, value = two_char))
                skip_until = i + 2
                continue
            elif two_char == "!=":
                tokens.append(make_token(tokenType = token_types.OPERATOR_NOT_EQUAL, value = two_char))
                skip_until = i + 2
                continue
            elif two_char == "<=":
                tokens.append(make_token(tokenType = token_types.OPERATOR_LESS_OR_EQUAL, value = two_char))
                skip_until = i + 2
                continue
            elif two_char == ">=":
                tokens.append(make_token(tokenType = token_types.OPERATOR_GREATER_OR_EQUAL, value = two_char))
                skip_until = i + 2
                continue
            elif two_char == "//":
                tokens.append(make_token(tokenType = token_types.DOUBLE_SLASH, value = two_char))
                skip_until = i + 2
                continue
            elif two_char == "**":
                tokens.append(make_token(tokenType = token_types.DOUBLE_ASTERISK, value = two_char))
                skip_until = i + 2
                continue
            elif two_char == "<<":
                tokens.append(make_token(tokenType = token_types.BITWISE_SHIFT_LEFT, value = two_char))
                skip_until = i + 2
                continue
            elif two_char == ">>":
                tokens.append(make_token(tokenType = token_types.BITWISE_SHIFT_RIGHT, value = two_char))
                skip_until = i + 2
                continue

        # Single character tokens
        if c == "(":
            tokens.append(make_token(tokenType = token_types.PARENTHESIS_LEFT, value = c))
        elif c == ")":
            tokens.append(make_token(tokenType = token_types.PARENTHESIS_RIGHT, value = c))
        elif c == "[":
            tokens.append(make_token(tokenType = token_types.BRACKET_LEFT, value = c))
        elif c == "]":
            tokens.append(make_token(tokenType = token_types.BRACKET_RIGHT, value = c))
        elif c == "{":
            tokens.append(make_token(tokenType = token_types.BRACE_LEFT, value = c))
        elif c == "}":
            tokens.append(make_token(tokenType = token_types.BRACE_RIGHT, value = c))
        elif c == ",":
            tokens.append(make_token(tokenType = token_types.COMMA, value = c))
        elif c == ".":
            tokens.append(make_token(tokenType = token_types.DOT, value = c))
        elif c == ":":
            tokens.append(make_token(tokenType = token_types.COLON, value = c))
        elif c == "=":
            tokens.append(make_token(tokenType = token_types.OPERATOR_ASSIGN, value = c))
        elif c == "+":
            tokens.append(make_token(tokenType = token_types.PLUS, value = c))
        elif c == "-":
            tokens.append(make_token(tokenType = token_types.MINUS, value = c))
        elif c == "*":
            tokens.append(make_token(tokenType = token_types.ASTERISK, value = c))
        elif c == "/":
            tokens.append(make_token(tokenType = token_types.SLASH, value = c))
        elif c == "%":
            tokens.append(make_token(tokenType = token_types.MOD, value = c))
        elif c == "<":
            tokens.append(make_token(tokenType = token_types.OPERATOR_LESS, value = c))
        elif c == ">":
            tokens.append(make_token(tokenType = token_types.OPERATOR_GREATER, value = c))
        elif c == "&":
            tokens.append(make_token(tokenType = token_types.BITWISE_AND, value = c))
        elif c == "|":
            tokens.append(make_token(tokenType = token_types.BITWISE_OR, value = c))
        elif c == "^":
            tokens.append(make_token(tokenType = token_types.BITWISE_XOR, value = c))
        elif c == "~":
            tokens.append(make_token(tokenType = token_types.BITWISE_NOT, value = c))
        else:
            # Fallback type for unknown characters
            tokens.append(make_token(tokenType = token_types.UNASSIGNED, value = c))

    return tokens
