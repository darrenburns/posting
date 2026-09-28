"""Syntax highlighting for GraphQL documents.

Textual's `TextArea` highlights using tree-sitter, and there's no GraphQL
grammar among the ones it bundles - so rather than take on a grammar
dependency (which has no wheels for most platforms), Posting lexes GraphQL
itself. GraphQL's lexical grammar is small enough that a scanner does a good
job of it.

The style names below are the ones Textual's built-in syntax themes define,
so highlighting follows whichever theme the user has chosen.
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Sequence

Highlight = tuple[int, int | None, str]
"""A highlight within a line: the start column, end column, and style name."""

COMMENT = "comment"
KEYWORD = "keyword"
OPERATION_NAME = "function"
DIRECTIVE = "function"
FIELD = "json.label"
ARGUMENT = "css.property"
TYPE = "type"
VARIABLE = "type.builtin"
STRING = "string"
NUMBER = "number"
BOOLEAN = "boolean"
CONSTANT = "constant.builtin"
BRACKET = "punctuation.bracket"
DELIMITER = "punctuation.delimiter"
OPERATOR = "operator"

KEYWORDS = frozenset(
    {
        # Executable documents.
        "query",
        "mutation",
        "subscription",
        "fragment",
        "on",
        # Schema definition language, so a pasted schema reads well too.
        "schema",
        "scalar",
        "type",
        "interface",
        "union",
        "enum",
        "input",
        "directive",
        "extend",
        "implements",
        "repeatable",
    }
)

OPERATION_KEYWORDS = frozenset({"query", "mutation", "subscription", "fragment"})

BLOCK_QUOTE = '"""'

TOKENS = re.compile(
    r"""
    (?P<comment>\#[^\n]*)
  | (?P<block_quote>\"\"\")
  | (?P<string>"(?:\\.|[^"\\])*"?)
  | (?P<variable>\$[_A-Za-z][_A-Za-z0-9]*)
  | (?P<directive>@[_A-Za-z][_A-Za-z0-9]*)
  | (?P<number>-?\d+\.\d*(?:[eE][+-]?\d+)?|-?\d+(?:[eE][+-]?\d+)?)
  | (?P<identifier>[_A-Za-z][_A-Za-z0-9]*)
  | (?P<bracket>[{}()\[\]])
  | (?P<delimiter>[:,])
  | (?P<operator>\.\.\.|[!=|&])
    """,
    re.VERBOSE,
)


@dataclass
class _State:
    """What the lexer knows about the document before the current token."""

    previous: str | None = None
    """The last significant token."""

    before_colon: str | None = None
    """The token before the most recent `:`, which says what follows it:
    a type after `$id:`, an argument's value after `role:`."""

    paren_depth: int = 0
    """How many argument lists enclose the cursor - `name:` inside one is an
    argument, and outside one it's an alias."""

    in_block_string: bool = False
    """Whether the previous line ended inside a `\"\"\"` string."""


def _classify_identifier(value: str, state: _State, following: str) -> str:
    """The style for an identifier, based on what surrounds it.

    Args:
        value: The identifier itself.
        state: What came before it.
        following: The rest of the line after it.
    """
    if value in KEYWORDS and state.previous != ":":
        return KEYWORD
    if value in {"true", "false"}:
        return BOOLEAN
    if value == "null":
        return CONSTANT
    if state.previous in OPERATION_KEYWORDS:
        # The name of an operation or fragment, e.g. `query GetUser`.
        return OPERATION_NAME
    if state.previous == "on":
        return TYPE
    if following.lstrip().startswith(":"):
        # An argument, an input object's field, or an alias.
        return ARGUMENT
    if state.previous in {"[", "!"}:
        return TYPE
    if state.previous == "=":
        # A default value, e.g. `$role: Role = ADMIN`.
        return CONSTANT
    if state.previous == ":":
        if (state.before_colon or "").startswith("$"):
            # The type of a variable, e.g. `$id: ID!`.
            return TYPE
        if following.lstrip().startswith("!"):
            # Only a type can be non-null.
            return TYPE
        if value.isupper():
            # An enum value, which are conventionally SCREAMING_CASE.
            return CONSTANT
        if value[:1].isupper():
            return TYPE
        if not state.paren_depth:
            # The field an alias refers to, e.g. `fullName: name`.
            return FIELD
        return CONSTANT
    return FIELD


def _line_highlights(line: str, state: _State) -> list[Highlight]:
    """Highlight a single line, advancing `state` as it goes.

    Args:
        line: The text of the line.
        state: The lexer state, updated in place.

    Returns:
        The line's highlights.
    """
    highlights: list[Highlight] = []
    index = 0

    if state.in_block_string:
        end = line.find(BLOCK_QUOTE)
        if end == -1:
            return [(0, None, STRING)]
        index = end + len(BLOCK_QUOTE)
        highlights.append((0, index, STRING))
        state.in_block_string = False

    while match := TOKENS.search(line, index):
        index = match.end()
        kind = match.lastgroup
        start = match.start()
        value = match.group()

        if kind == "comment":
            highlights.append((start, None, COMMENT))
            break
        elif kind == "block_quote":
            end = line.find(BLOCK_QUOTE, index)
            if end == -1:
                highlights.append((start, None, STRING))
                state.in_block_string = True
                return highlights
            index = end + len(BLOCK_QUOTE)
            highlights.append((start, index, STRING))
            continue
        elif kind == "string":
            highlights.append((start, index, STRING))
        elif kind == "variable":
            highlights.append((start, index, VARIABLE))
        elif kind == "directive":
            highlights.append((start, index, DIRECTIVE))
        elif kind == "number":
            highlights.append((start, index, NUMBER))
        elif kind == "identifier":
            highlights.append(
                (start, index, _classify_identifier(value, state, line[index:]))
            )
        elif kind == "bracket":
            highlights.append((start, index, BRACKET))
            if value == "(":
                state.paren_depth += 1
            elif value == ")":
                state.paren_depth = max(0, state.paren_depth - 1)
        elif kind == "delimiter":
            highlights.append((start, index, DELIMITER))
            if value == ":":
                state.before_colon = state.previous
        else:
            highlights.append((start, index, OPERATOR))

        state.previous = value

    return highlights


def highlight_lines(lines: Sequence[str]) -> dict[int, list[Highlight]]:
    """Highlight a GraphQL document.

    Args:
        lines: The lines of the document, without their line endings.

    Returns:
        The highlights of each line which has any, keyed by line index.
    """
    highlights: dict[int, list[Highlight]] = {}
    state = _State()
    for index, line in enumerate(lines):
        if line_highlights := _line_highlights(line, state):
            highlights[index] = line_highlights
    return highlights
