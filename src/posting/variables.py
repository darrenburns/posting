from __future__ import annotations
from functools import lru_cache

import re
import os
from pathlib import Path
from dotenv import dotenv_values


_VARIABLES_PATTERN = re.compile(
    r"\$(?:([a-zA-Z_][a-zA-Z0-9_]*)|{([a-zA-Z_][a-zA-Z0-9_]*)})"
)


class SharedVariables:
    def __init__(self, current_env: str = "default.env") -> None:
        self._current_env = current_env
        self._variables: dict[str, object] = {}

    def get(self, env: str | None = None) -> dict[str, dict[str, object]]:
        if env:
            return self._variables.get(env, {}).copy()
        return self._variables.get(self._current_env, {}).copy()

    def set(self, variables: dict[str, object], env: str | None = None) -> None:
        if env:
            self._variables[env] = variables
        else:
            self._variables[self._current_env] = variables

    def update(self, new_variables: dict[str, object], 
               env: str | None = None ) -> None:
        if env:
            self._variables[env].update(new_variables)
        else:
            self._variables[self._current_env].update(new_variables)
        
    def remove(self, key: str, env: str | None = None) -> None:
        if env:
            self._variables[env].pop(key, None)
        else:
            self._variables[self._current_env].pop(key, None)
            
    def get_envs(self) -> list[str]:
        return list(self._variables.keys())

    def set_current_env(self, env: str) -> None:
        self._current_env = env
    
    def get_current_env(self) -> str:
        return self._current_env

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

    envs: dict[str, dict[str, object]] = {}
    for file in environment_files:
        variables = envs.setdefault(file.name, {})
        for key, value in dotenv_values(file).items():
            variables[key] = value
    
    
    if use_host_environment:
        variables = envs.setdefault("host_environment", {})
        host_env_variables = {key: value for key, value in os.environ.items()}
        variables.update({**variables, **host_env_variables})

    for env, variables in envs.items():
        VARIABLES.set(variables, env=env)
        
    VARIABLES.set_current_env(environment_files[0].name)
        
    return get_variables()


def update_variables(new_variables: dict[str, object]) -> None:
    """Update the current variables with new values.

    This function safely updates the shared variables with new key-value pairs.
    If a key already exists, its value will be updated. If it doesn't exist, it will be added.

    Args:
        new_variables: A dictionary containing the new variables to update or add.
    """
    VARIABLES.update(new_variables)
    
def remove_variable(key: str) -> None:
    """Remove a variable from the shared variables.

    Args:
        key: The key of the variable to remove.
    """
    VARIABLES.remove(key)

def get_environments() -> list[str]:
    return list(VARIABLES.get_envs())

def set_current_environment(env: str) -> None:
    VARIABLES.set_current_env(env)
    
def get_current_environment() -> str:
    return VARIABLES._current_env

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
