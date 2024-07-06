from __future__ import annotations
from contextvars import ContextVar

import re
from string import Template
import os
from pathlib import Path
from dotenv import dotenv_values


_VARIABLES_PATTERN = re.compile(
    r"\$(?:\{((?:env:)?[_a-z][_a-z0-9]*)\}|(env:[_a-z][_a-z0-9]*))"
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
        f"env:{key}": value
        for file in environment_files
        for key, value in dotenv_values(file).items()
    }
    if use_host_environment:
        host_env_variables = {f"env:{key}": value for key, value in os.environ.items()}
        variables = {**variables, **host_env_variables}

    VARIABLES.set(variables)
    return variables


class VariablesTemplate(Template):
    delimiter = "$"
    pattern = r"""
    \$(?:
      (?P<escaped>\$) |
      (?:
        \{(?P<braced>(?:env:)?[_a-z][_a-z0-9]*)\} |
        (?P<named>env:[_a-z][_a-z0-9]*)
      )
    )
    """


def find_variables(template_str: str) -> list[tuple[str, int, int]]:
    return [
        (m.group(1) or m.group(2), m.start(), m.end())
        for m in re.finditer(_VARIABLES_PATTERN, template_str)
    ]


class SubstitutionError(Exception):
    """Raised when the user refers to a variable that doesn't exist."""
