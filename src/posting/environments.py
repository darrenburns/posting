"""Switch loaded dotenv files while retaining session variables."""

from pathlib import Path
from typing import TYPE_CHECKING

from posting.variables import load_variables, update_variables

if TYPE_CHECKING:
    from posting.app import Posting


def switch_environment(app: "Posting", paths: tuple[Path, ...]) -> bool:
    paths = tuple(path.expanduser().resolve() for path in paths)
    for path in paths:
        if not path.is_file():
            app.notify(f"Environment file not found: {path}", severity="error")
            return False
    try:
        load_variables(paths, app.settings.use_host_environment, avoid_cache=True)
    except (OSError, UnicodeError) as error:
        app.notify(f"Could not load environment: {error}", severity="error")
        return False
    previous = app.environment_files
    history = getattr(app, "environment_history", [previous])
    if paths not in history:
        history.append(paths)
    app.environment_history = history
    app.environment_files = paths
    update_variables(app.session_env)
    if getattr(app.settings, "watch_env_files", False):
        app.workers.cancel_group(app, "environment-watcher")
        if paths:
            app.watch_environment_files()
    app.env_changed_signal.publish(None)
    return True
