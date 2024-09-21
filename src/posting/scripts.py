from __future__ import annotations

import sys
from pathlib import Path
from types import ModuleType
from typing import Callable, Any
import threading

# Global cache for loaded modules
_MODULE_CACHE: dict[str, ModuleType] = {}
_CACHE_LOCK = threading.Lock()


def clear_module_cache():
    """
    Clear the global module cache in a thread-safe manner.
    """
    with _CACHE_LOCK:
        _MODULE_CACHE.clear()


def execute_script(script_path: Path, function_name: str) -> Callable[..., Any] | None:
    """
    Execute a Python script from the given path and extract a specified function.
    Uses caching to prevent multiple executions of the same script.

    Args:
        script_path: Path to the Python script file.
        function_name: Name of the function to extract from the script.

    Returns:
        The extracted function if found, None otherwise.

    Raises:
        FileNotFoundError: If the script file does not exist.
        Exception: If there's an error during script execution.
    """
    if not script_path.is_file():
        raise FileNotFoundError(f"Script not found: {script_path}")

    script_dir = script_path.parent.resolve()
    module_name = script_path.stem
    module_key = str(script_path.resolve())

    try:
        sys.path.insert(0, str(script_dir))
        module = _import_script_as_module(script_path, module_name, module_key)
        return _validate_function(getattr(module, function_name, None))
    finally:
        sys.path.remove(str(script_dir))


def _import_script_as_module(
    script_path: Path, module_name: str, module_key: str
) -> ModuleType:
    """Import the script file as a module, using cache if available."""
    with _CACHE_LOCK:
        if module_key in _MODULE_CACHE:
            return _MODULE_CACHE[module_key]

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
    with _CACHE_LOCK:
        _MODULE_CACHE[module_key] = module

    return module


def _validate_function(func: Any) -> Callable[..., Any] | None:
    """
    Validate that the provided object is a callable function.

    Args:
        func: The object to validate.

    Returns:
        The function if it's callable, None otherwise.
    """
    return func if callable(func) else None
