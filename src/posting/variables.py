from __future__ import annotations
from contextvars import ContextVar
from functools import lru_cache

import re
import os
from pathlib import Path
from dotenv import dotenv_values
from textual_autocomplete import TargetState


_VARIABLES_PATTERN = re.compile(
    r"\$(?:([a-zA-Z_][a-zA-Z0-9_]*)|{([a-zA-Z_][a-zA-Z0-9_]*)})"
)

_initial_variables: dict[str, str | None] = {}
VARIABLES: ContextVar[dict[str, str | None]] = ContextVar(
    "variables", default=_initial_variables
)


def get_variables() -> dict[str, str | None]:
    return VARIABLES.get()


def load_variables(
    environment_files: tuple[Path, ...], use_host_environment: bool
) -> dict[str, str | None]:
    """Load the variables that are currently available in the environment.

    This will make them available via the `get_variables` function."""

    existing_variables = VARIABLES.get()
    if existing_variables:
        return {key: value for key, value in existing_variables}

    variables: dict[str, str | None] = {
        key: value
        for file in environment_files
        for key, value in dotenv_values(file).items()
    }
    if use_host_environment:
        host_env_variables = {key: value for key, value in os.environ.items()}
        variables = {**variables, **host_env_variables}

    VARIABLES.set(variables)
    return variables


@lru_cache()
def find_variables(template_str: str) -> list[tuple[str, int, int]]:
    return [
        (m.group(1) or m.group(2), m.start(), m.end())
        for m in re.finditer(_VARIABLES_PATTERN, template_str)
    ]


@lru_cache()
def is_cursor_within_variable(cursor: int, text: str) -> bool:
    if not text or cursor < 0 or cursor > len(text):
        return False

    # Check for ${var} syntax
    start = text.rfind("${", 0, cursor)
    if start != -1:
        end = text.find("}", start)
        if end != -1 and start < cursor <= end:
            return True

    # Check for $ followed by { (cursor between $ and {)
    if (
        cursor > 0
        and text[cursor - 1] == "$"
        and cursor < len(text)
        and text[cursor] == "{"
    ):
        return True

    # Check for $var syntax
    last_dollar = text.rfind("$", 0, cursor)
    if last_dollar == -1:
        return False

    # Check if cursor is within a valid variable name
    for i in range(last_dollar + 1, len(text)):
        if i >= cursor:
            return True
        if not (text[i].isalnum() or text[i] == "_"):
            return False

    return True


@lru_cache()
def find_variable_start(cursor: int, text: str) -> int:
    if not text:
        return cursor

    # Check for ${var} syntax
    start = text.rfind("${", 0, cursor)
    if start != -1 and start < cursor <= text.find("}", start):
        return start

    # Check for $ followed by { (cursor between $ and {)
    if (
        cursor > 0
        and text[cursor - 1] == "$"
        and cursor < len(text)
        and text[cursor] == "{"
    ):
        return cursor - 1

    # Check for $var syntax
    for i in range(cursor - 1, -1, -1):
        if text[i] == "$":
            if i + 1 < len(text) and text[i + 1] == "{":
                return i
            if all(c.isalnum() or c == "_" for c in text[i + 1 : cursor]):
                return i

    return cursor  # No valid variable start found


@lru_cache()
def find_variable_end(cursor: int, text: str) -> int:
    if not text:
        return cursor

    # Check for ${var} syntax
    start = text.rfind("${", 0, cursor)
    if start != -1:
        end = text.find("}", start)
        if end != -1 and start < cursor <= end + 1:  # Include the closing brace
            return end + 1

    # Check for $ followed by { (cursor between $ and {)
    if (
        cursor > 0
        and text[cursor - 1] == "$"
        and cursor < len(text)
        and text[cursor] == "{"
    ):
        end = text.find("}", cursor)
        if end != -1:
            return end + 1

    # Check for $var syntax
    for i in range(cursor, len(text)):
        if not (text[i].isalnum() or text[i] == "_"):
            return i

    return len(text)


@lru_cache()
def get_variable_at_cursor(cursor: int, text: str) -> str | None:
    if not is_cursor_within_variable(cursor, text):
        return None

    start = find_variable_start(cursor, text)
    end = find_variable_end(cursor, text)

    return text[start:end]


def extract_variable_name(variable_text: str) -> str:
    """
    Extract the variable name from a variable reference.

    Args:
        variable_text: The text containing the variable reference.

    Returns:
        str: The extracted variable name.

    Examples:
        >>> extract_variable_name("$var")
        'var'
        >>> extract_variable_name("${MY_VAR}")
        'MY_VAR'
    """
    if variable_text.startswith("${") and variable_text.endswith("}"):
        return variable_text[2:-1]
    elif variable_text.startswith("$"):
        return variable_text[1:]
    return variable_text  # Return as-is if it doesn't match expected formats


class SubstitutionError(Exception):
    """Raised when the user refers to a variable that doesn't exist."""
