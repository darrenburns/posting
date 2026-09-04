"""Context-aware completions for the GraphQL query editor.

Given the text of a GraphQL document and the position of the cursor within it,
`complete` works out what can legally appear there - the fields of the type
being selected, the arguments of a field, the values of an enum, and so on -
and returns the candidates from the schema.

This is deliberately a lightweight scan rather than a full GraphQL parser: the
document being edited is usually incomplete, so we only need to understand
enough of it to know where the cursor is.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from typing import Iterator, Literal

from posting.graphql.schema import Field, OperationType, Schema, Type

CompletionKind = Literal["field", "argument", "enum", "type", "variable", "none"]

IDENTIFIER = re.compile(r"[_A-Za-z][_A-Za-z0-9]*")
OPERATION_KEYWORDS: dict[str, OperationType] = {
    "query": "query",
    "mutation": "mutation",
    "subscription": "subscription",
}
TYPENAME_DESCRIPTION = "The name of the concrete type of this object."


@dataclass(frozen=True)
class Completion:
    """A single thing which could be inserted at the cursor."""

    name: str
    kind: CompletionKind
    type: str = ""
    """The GraphQL type of the field, argument or variable, if it has one."""

    description: str = ""
    deprecated: bool = False


@dataclass(frozen=True)
class Context:
    """What the cursor is currently positioned to complete."""

    kind: CompletionKind
    prefix: str = ""
    """The partially typed word at the cursor."""

    start: int = 0
    """The offset at which `prefix` begins."""

    path: tuple[str, ...] = ()
    """The field names (and inline fragment type conditions) enclosing the cursor."""

    operation: OperationType = "query"
    base_type: str | None = None
    """The type of a fragment definition's type condition, if we're inside one."""

    input_path: tuple[str, ...] = ()
    """The argument, then input object fields, enclosing the cursor inside an
    input object literal, e.g. `createUser(input: {profile: {` gives
    `("input", "profile")`."""

    field_name: str | None = None
    """The field whose arguments are being completed."""

    argument_name: str | None = None
    """The argument whose value is being completed."""

    variables: tuple[str, ...] = ()
    """The variables declared in the document, without their `$` prefix."""


@dataclass
class _Scan:
    """The state we accumulate while scanning the document before the cursor."""

    stack: list[str | None] = field(default_factory=list)
    operation: OperationType = "query"
    last_identifier: str | None = None
    paren_fields: list[str | None] = field(default_factory=list)
    input_objects: list[str | None] = field(default_factory=list)
    """The argument and input object fields enclosing an input object literal."""

    argument_name: str | None = None
    last_token: tuple[str, str] | None = None
    previous_token: tuple[str, str] | None = None
    variables: list[str] = field(default_factory=list)
    unterminated: bool = False
    """True if the text ends inside a string or comment."""

    base_type: str | None = None
    """The type condition of a fragment definition, e.g. `fragment F on User`."""


def _tokens(text: str, scan: _Scan) -> Iterator[tuple[str, str]]:
    """Yield the significant tokens of `text`, skipping whitespace and comments."""
    index = 0
    length = len(text)
    while index < length:
        character = text[index]
        if character in " \t\r\n,﻿":
            index += 1
        elif character == "#":
            newline = text.find("\n", index)
            if newline == -1:
                scan.unterminated = True
                return
            index = newline + 1
        elif character == '"':
            if text.startswith('"""', index):
                end = text.find('"""', index + 3)
                if end == -1:
                    scan.unterminated = True
                    return
                index = end + 3
            else:
                end = index + 1
                while end < length and text[end] != '"':
                    end += 2 if text[end] == "\\" else 1
                if end >= length:
                    scan.unterminated = True
                    return
                index = end + 1
            yield ("string", "")
        elif match := IDENTIFIER.match(text, index):
            index = match.end()
            yield ("identifier", match.group())
        elif text.startswith("...", index):
            index += 3
            yield ("punctuation", "...")
        else:
            index += 1
            yield ("punctuation", character)


def tokens(text: str) -> Iterator[tuple[str, str]]:
    """The significant tokens of a GraphQL document.

    Whitespace, commas and comments are skipped. A string is yielded as a
    single `("string", "")` token, since only its presence matters here.
    """
    return _tokens(text, _Scan())


def _scan_document(text: str) -> _Scan:
    """Scan the document up to the cursor, tracking where we've ended up."""
    scan = _Scan()
    for token in _tokens(text, scan):
        kind, value = token
        if kind == "identifier":
            if (
                not scan.stack
                and not scan.paren_fields
                and (operation := OPERATION_KEYWORDS.get(value))
            ):
                scan.operation = operation
            if scan.last_token == ("punctuation", "$"):
                scan.variables.append(value)
            else:
                scan.last_identifier = value
        elif kind == "punctuation":
            if value == "{":
                if scan.paren_fields:
                    # An input object literal, e.g. `createUser(input: {`.
                    scan.input_objects.append(scan.argument_name)
                    scan.argument_name = None
                else:
                    if not scan.stack:
                        # The start of a top-level definition. Only a fragment
                        # definition's type condition applies to it, e.g.
                        # `fragment F on User {` - and it must not carry over
                        # into the definitions which follow it.
                        scan.base_type = (
                            scan.last_identifier
                            if scan.previous_token == ("identifier", "on")
                            else None
                        )
                    scan.stack.append(scan.last_identifier)
                scan.last_identifier = None
            elif value == "}":
                if scan.input_objects:
                    scan.input_objects.pop()
                    scan.argument_name = None
                elif scan.stack:
                    scan.stack.pop()
                scan.last_identifier = None
            elif value == "(":
                scan.paren_fields.append(scan.last_identifier)
                scan.last_identifier = None
                scan.argument_name = None
            elif value == ")":
                # Restore the identifier which owns the parentheses, so that
                # `user(id: 1) { ... }` still knows it's selecting on `user`.
                scan.last_identifier = (
                    scan.paren_fields.pop() if scan.paren_fields else None
                )
                scan.argument_name = None
            elif value == ":" and scan.paren_fields:
                scan.argument_name = scan.last_identifier
        scan.previous_token = scan.last_token
        scan.last_token = token
    return scan


def _prefix_at(text: str, cursor: int) -> tuple[str, int]:
    """The partially typed word ending at the cursor, and where it starts."""
    start = cursor
    while start > 0 and (text[start - 1] == "_" or text[start - 1].isalnum()):
        start -= 1
    return text[start:cursor], start


def analyze(text: str, cursor: int) -> Context:
    """Work out what the cursor is positioned to complete.

    Args:
        text: The full text of the GraphQL document being edited.
        cursor: The offset of the cursor within that text.
    """
    cursor = max(0, min(cursor, len(text)))
    prefix, start = _prefix_at(text, cursor)
    scan = _scan_document(text[:start])

    if scan.unterminated:
        # The cursor is inside a string or a comment.
        return Context(kind="none", prefix=prefix, start=start)

    path = tuple(segment for segment in scan.stack[1:] if segment)
    common = {
        "prefix": prefix,
        "start": start,
        "path": path,
        "operation": scan.operation,
        "base_type": scan.base_type,
        "input_path": tuple(name for name in scan.input_objects if name),
        "variables": tuple(dict.fromkeys(scan.variables)),
    }

    if start > 0 and text[start - 1] == "$":
        return Context(kind="variable", **common)

    if scan.paren_fields:
        if not scan.stack:
            # Still in the operation header, e.g. `query Foo($id: ID!)`.
            return Context(kind="none", **common)
        if scan.last_token == ("punctuation", ":"):
            return Context(
                kind="enum",
                field_name=scan.paren_fields[0],
                argument_name=scan.argument_name,
                **common,
            )
        return Context(kind="argument", field_name=scan.paren_fields[0], **common)

    if not scan.stack:
        return Context(kind="none", **common)

    if scan.last_token == ("punctuation", "...") or (
        scan.last_token == ("identifier", "on")
        and scan.previous_token == ("punctuation", "...")
    ):
        return Context(kind="type", **common)

    return Context(kind="field", **common)


def resolve_type(schema: Schema, context: Context) -> Type | None:
    """The type whose fields are being selected at the cursor."""
    if context.base_type and (base := schema.type_named(context.base_type)):
        # A fragment definition, e.g. `fragment Fields on User { ... }`.
        current: Type | None = base
    else:
        current = schema.root_type(context.operation)

    for segment in context.path:
        if current is None:
            return None
        if field_ := current.fields.get(segment):
            current = schema.type_named(field_.named_type)
        else:
            # Not a field, so it's most likely an inline fragment's type
            # condition, e.g. `... on User { ... }`.
            current = schema.type_named(segment)
    return current


def _field_completions(type_: Type) -> list[Completion]:
    completions = [
        Completion(
            name=field_.name,
            kind="field",
            type=field_.type,
            description=field_.description,
            deprecated=field_.deprecated,
        )
        for field_ in type_.fields.values()
        if not field_.name.startswith("__")
    ]
    completions.sort(key=lambda completion: completion.deprecated)
    if type_.is_selectable:
        completions.append(
            Completion(
                name="__typename",
                kind="field",
                type="String!",
                description=TYPENAME_DESCRIPTION,
            )
        )
    return completions


def _resolve_input_type(
    schema: Schema, field_: Field, input_path: tuple[str, ...]
) -> Type | None:
    """The input object type whose fields are being completed, if any.

    Args:
        field_: The field whose arguments are being supplied.
        input_path: The argument name, followed by any input object fields
            which the cursor is nested inside.
    """
    if not input_path:
        return None
    argument = field_.argument(input_path[0])
    current = schema.type_named(argument.named_type) if argument else None
    for segment in input_path[1:]:
        if current is None:
            return None
        input_field = current.input_fields.get(segment)
        current = schema.type_named(input_field.named_type) if input_field else None
    return current


def _input_field_completions(input_type: Type) -> list[Completion]:
    return [
        Completion(
            name=input_field.name,
            kind="argument",
            type=input_field.type,
            description=input_field.description,
        )
        for input_field in input_type.input_fields.values()
    ]


def complete(schema: Schema, text: str, cursor: int) -> list[Completion]:
    """The completions available at the cursor.

    Args:
        schema: The schema of the endpoint the query will be sent to.
        text: The full text of the GraphQL document being edited.
        cursor: The offset of the cursor within that text.

    Returns:
        The candidates which could be inserted at the cursor, unfiltered - it's
        up to the caller to match them against `analyze(text, cursor).prefix`.
    """
    context = analyze(text, cursor)
    if context.kind == "none":
        return []

    if context.kind == "variable":
        return [
            Completion(name=name, kind="variable") for name in sorted(context.variables)
        ]

    current_type = resolve_type(schema, context)
    if current_type is None:
        return []

    if context.kind == "field":
        return _field_completions(current_type)

    if context.kind == "type":
        names = current_type.possible_types or schema.object_type_names()
        return [
            Completion(
                name=name,
                kind="type",
                description=type_.description
                if (type_ := schema.type_named(name))
                else "",
            )
            for name in names
        ]

    field_ = current_type.fields.get(context.field_name) if context.field_name else None
    if field_ is None:
        return []

    input_type = _resolve_input_type(schema, field_, context.input_path)
    if context.input_path and input_type is None:
        return []

    if context.kind == "argument":
        if input_type is not None:
            return _input_field_completions(input_type)
        return [
            Completion(
                name=argument.name,
                kind="argument",
                type=argument.type,
                description=argument.description,
            )
            for argument in field_.arguments
        ]

    # A value in argument position, e.g. an enum value or an input object.
    if input_type is not None:
        value = input_type.input_fields.get(context.argument_name or "")
    else:
        value = (
            field_.argument(context.argument_name) if context.argument_name else None
        )
    value_type = schema.type_named(value.named_type) if value else None
    if value_type is None:
        return []
    if value_type.kind == "ENUM":
        return [
            Completion(
                name=enum_value.name,
                kind="enum",
                description=enum_value.description,
                deprecated=enum_value.deprecated,
            )
            for enum_value in value_type.enum_values
        ]
    if value_type.kind == "INPUT_OBJECT":
        return _input_field_completions(value_type)
    return []
