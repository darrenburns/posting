from __future__ import annotations

import sys
from pathlib import Path
from types import ModuleType
from typing import TYPE_CHECKING, Callable, Any
import threading

from httpx import Request, Response
from textual.app import App
from textual.notifications import SeverityLevel

if TYPE_CHECKING:
    from posting.app import Posting

# Global cache for loaded modules
_MODULE_CACHE: dict[str, ModuleType] = {}
_CACHE_LOCK = threading.Lock()


# class Context:
#     def __init__(self, app: Posting):
#         self._app: "Posting" = app
#         self.request: Request | None = None
#         self.response: Response | None = None

#     def notify(
#         self,
#         message: str,
#         *,
#         title: str = "",
#         severity: SeverityLevel = "information",
#         timeout: float | None = None,
#     ):
#         self._app.notify(
#             message=message,
#             title=title,
#             severity=severity,
#             timeout=timeout,
#         )


def clear_module_cache():
    """
    Clear the global module cache in a thread-safe manner.
    """
    with _CACHE_LOCK:
        _MODULE_CACHE.clear()


def execute_script(
    collection_root: Path, script_path: Path, function_name: str
) -> Callable[..., Any] | None:
    """
    Execute a Python script from the given path and extract a specified function.
    Uses caching to prevent multiple executions of the same script.

    Args:
        collection_root: Path to the root of the collection.
        script_path: Path to the Python script file, relative to the collection root.
        function_name: Name of the function to extract from the script.

    Returns:
        The extracted function if found, None otherwise.

    Raises:
        FileNotFoundError: If the script file does not exist or is outside the collection.
        Exception: If there's an error during script execution.
    """
    # Ensure the script_path is relative to the collection root
    full_script_path = (collection_root / script_path).resolve()

    # Check if the script is within the collection root
    if not full_script_path.is_relative_to(collection_root):
        raise FileNotFoundError(
            f"Script path {script_path} is outside the collection root"
        )

    if not full_script_path.is_file():
        raise FileNotFoundError(f"Script not found: {full_script_path}")

    script_dir = full_script_path.parent
    module_name = full_script_path.stem
    module_key = str(full_script_path)

    try:
        sys.path.insert(0, str(script_dir))
        module = _import_script_as_module(full_script_path, module_name, module_key)
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


def uncache_module(script_path: str) -> None:
    """
    Clear a specific module from the global module cache.

    Args:
        script_path: Path to the script file.
    """
    module_key = str(script_path)
    with _CACHE_LOCK:
        if module_key in _MODULE_CACHE:
            del _MODULE_CACHE[module_key]
