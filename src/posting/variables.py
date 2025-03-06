from __future__ import annotations
from functools import lru_cache

import re
import os
from pathlib import Path
from dotenv import dotenv_values


_VARIABLES_PATTERN = re.compile(
    r"(?:(?:^|[^\$])(?:\$\$)*)(\$(?:\{([a-zA-Z_]\w*)\}|([a-zA-Z_]\w*)|$))"
)


class SharedVariables:
    def __init__(self):
        self._variables: dict[str, object] = {}

    def get(self) -> dict[str, object]:
        return self._variables.copy()

    def set(self, variables: dict[str, object]) -> None:
        self._variables = variables

    def update(self, new_variables: dict[str, object]) -> None:
        self._variables.update(new_variables)


VARIABLES = SharedVariables()


def get_variables() -> dict[str, object]:
    return VARIABLES.get()


def load_variables(
    environment_files: tuple[Path, ...],
    use_host_environment: bool,
    avoid_cache: bool = False,
) -> dict[str, object]:
    """Load the variables that are currently available in the environment.

    This will likely involve reading from a set of environment files,
    but it could also involve reading from the host machine's environment
    if `use_host_environment` is True.

    This will make them available via the `get_variables` function.

    Args:
        environment_files: The environment files to load variables from.
        use_host_environment: Whether to use env vars from the host machine.
        avoid_cache: Whether to avoid using cached variables (so do a full lookup).
        overlay_variables: Additional variables to overlay on top of the result.
    """

    existing_variables = get_variables()
    if existing_variables and not avoid_cache:
        return {key: value for key, value in existing_variables.items()}

    variables: dict[str, object] = {
        key: value
        for file in environment_files
        for key, value in dotenv_values(file).items()
    }
    if use_host_environment:
        host_env_variables = {key: value for key, value in os.environ.items()}
        variables = {**variables, **host_env_variables}

    VARIABLES.set(variables)
    return variables


def update_variables(new_variables: dict[str, object]) -> None:
    """Update the current variables with new values.

    This function safely updates the shared variables with new key-value pairs.
    If a key already exists, its value will be updated. If it doesn't exist, it will be added.

    Args:
        new_variables: A dictionary containing the new variables to update or add.
    """
    VARIABLES.update(new_variables)


@lru_cache()
def find_variables(template_str: str) -> list[tuple[str, int, int]]:
    return [
        (m.group(2) or m.group(3), m.start(1), m.end(1))
        for m in re.finditer(_VARIABLES_PATTERN, template_str)
        if m.group(2) or m.group(3)
    ]


@lru_cache()
def variable_range_at_cursor(cursor: int, text: str) -> tuple[int, int] | None:
    if not text or cursor < 0 or cursor > len(text):
        return None

    for match in _VARIABLES_PATTERN.finditer(text):
        start, end = match.span(1)
        if start < cursor and (
            cursor < end or not match.group(2) and cursor == end == len(text)
        ):
            return start, end
    return None


def is_cursor_within_variable(cursor: int, text: str) -> bool:
    return variable_range_at_cursor(cursor, text) is not None


def find_variable_start(cursor: int, text: str) -> int:
    variable_range = variable_range_at_cursor(cursor, text)
    return variable_range[0] if variable_range is not None else cursor


def find_variable_end(cursor: int, text: str) -> int:
    if not text:
        return cursor

    variable_range = variable_range_at_cursor(cursor, text)
    return variable_range[1] if variable_range is not None else len(text)


def get_variable_at_cursor(cursor: int, text: str) -> str | None:
    variable_range = variable_range_at_cursor(cursor, text)
    if variable_range is None:
        return None

    return text[variable_range[0] : variable_range[1]]


@lru_cache()
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
