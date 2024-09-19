"""Deals with running scripts that can be saved alongside a collection.

These could be pre-request, post-response.
"""

from __future__ import annotations

import sys
from pathlib import Path
from types import ModuleType
from typing import Callable, NamedTuple

import httpx


class ScriptFunctions(NamedTuple):
    on_request: Callable[[httpx.Request], None] | None
    on_response: Callable[[httpx.Response], None] | None


def execute_script(script_path: Path) -> ScriptFunctions:
    """
    Execute a Python script from the given path and extract on_request and on_response functions.

    Args:
        script_path: Path to the Python script.

    Returns:
        ScriptFunctions: A named tuple containing the extracted functions.

    Raises:
        FileNotFoundError: If the script file does not exist.
        Exception: If there's an error during script execution.
    """
    if not script_path.is_file():
        raise FileNotFoundError(f"Script not found: {script_path}")

    script_dir = script_path.parent.resolve()
    module_name = script_path.stem

    try:
        sys.path.insert(0, str(script_dir))
        module = _import_script_as_module(script_path, module_name)
        on_request = getattr(module, "on_request", None)
        on_response = getattr(module, "on_response", None)
        return ScriptFunctions(on_request=on_request, on_response=on_response)
    finally:
        sys.path.remove(str(script_dir))


def _import_script_as_module(script_path: Path, module_name: str) -> ModuleType:
    """
    Import the script file as a module.

    Args:
        script_path (Path): Path to the script file.
        module_name (str): Name to use for the module.

    Returns:
        ModuleType: The imported module.

    Raises:
        ImportError: If there's an error importing the module.
    """
    import importlib.util

    spec = importlib.util.spec_from_file_location(module_name, script_path)
    if spec is None:
        raise ImportError(
            f"Could not load spec for module {module_name} from {script_path}"
        )

    module = importlib.util.module_from_spec(spec)
    if spec.loader is None:
        raise ImportError(f"Could not load module {module_name} from {script_path}")

    spec.loader.exec_module(module)
    return module


# Usage example
if __name__ == "__main__":
    script_path = Path("path/to/your/script.py")
