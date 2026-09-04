"""Finding the operations in a GraphQL document.

A document may hold more than one operation, in which case the request has to
say which of them to run. These functions find the operations so that Posting
can ask.
"""

from __future__ import annotations

from dataclasses import dataclass

from posting.graphql.completion import OPERATION_KEYWORDS, tokens
from posting.graphql.schema import OperationType

FRAGMENT_KEYWORD = "fragment"


@dataclass(frozen=True)
class Operation:
    """An operation defined in a GraphQL document."""

    kind: OperationType = "query"
    name: str = ""
    """The operation's name, which is empty if it's anonymous."""

    def __str__(self) -> str:
        return f"{self.kind} {self.name}" if self.name else self.kind


def find_operations(document: str) -> list[Operation]:
    """The operations defined in a GraphQL document, in the order written.

    Fragment definitions aren't operations, so they're skipped.

    Args:
        document: The text of the document.
    """
    operations: list[Operation] = []
    depth = 0
    paren_depth = 0
    kind: OperationType | None = None
    name = ""
    in_fragment = False

    for token_kind, value in tokens(document):
        if token_kind == "identifier":
            if depth or paren_depth:
                continue
            if operation := OPERATION_KEYWORDS.get(value):
                kind, name = operation, ""
            elif value == FRAGMENT_KEYWORD:
                in_fragment = True
            elif kind is not None and not name:
                # The name follows the operation's keyword.
                name = value
        elif token_kind == "punctuation":
            if value == "{":
                if depth == 0 and not in_fragment:
                    operations.append(Operation(kind=kind or "query", name=name))
                depth += 1
            elif value == "}":
                depth = max(0, depth - 1)
                if depth == 0:
                    kind, name, in_fragment = None, "", False
            elif value == "(":
                paren_depth += 1
            elif value == ")":
                paren_depth = max(0, paren_depth - 1)

    return operations


def operations_to_choose(document: str, operation_name: str = "") -> list[Operation]:
    """The operations to choose between before sending, if a choice is needed.

    A server needs to be told which operation to run when a document defines
    more than one. No choice is needed for a single operation, nor when the
    request already names one of them.

    Args:
        document: The text of the document.
        operation_name: The operation the request is currently set to send.

    Returns:
        The operations to choose between, or an empty list if the request can
        be sent as it is.
    """
    operations = find_operations(document)
    if len(operations) < 2:
        return []

    named = [operation for operation in operations if operation.name]
    if not named:
        # Anonymous operations can't be chosen by name.
        return []

    chosen = operation_name.strip()
    if chosen and any(operation.name == chosen for operation in named):
        return []
    return named
