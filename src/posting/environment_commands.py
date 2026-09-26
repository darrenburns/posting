"""Environment submenu in the command palette."""

from functools import partial
from pathlib import Path
from textual.command import CommandPalette, DiscoveryHit, Hit, Hits, Provider

from posting.environments import switch_environment
from posting.locations import config_directory
from posting.widgets.load_env_file_dialog import (
    is_env_file_candidate,
    show_load_env_file_dialog,
)


class EnvironmentProvider(Provider):
    def choices(self):
        app = self.app
        groups = list(getattr(app, "environment_history", [app.environment_files]))
        roots = {Path.cwd(), app.collection.path, config_directory()}
        roots.update(path.parent for group in groups for path in group)
        candidates = set()
        for root in roots:
            try:
                candidates.update(
                    path.resolve()
                    for path in root.iterdir()
                    if path.is_file() and is_env_file_candidate(path)
                )
            except OSError:
                continue
        for path in sorted(candidates):
            if (path,) not in groups:
                groups.append((path,))
        if () not in groups:
            groups.append(())
        for paths in groups:
            label = " + ".join(path.name for path in paths) or "No environment files"
            active = paths == app.environment_files
            yield (
                label + (" (active)" if active else ""),
                partial(self.select, paths),
                " · ".join(map(str, paths))
                or "Keep only session and configured host variables",
            )
        yield (
            "Load another env file…",
            partial(show_load_env_file_dialog, app),
            "Browse for an environment file",
        )

    def select(self, paths):
        if switch_environment(self.app, paths):
            label = " + ".join(path.name for path in paths) or "No environment files"
            self.app.notify(label, title="Environment switched")

    async def discover(self) -> Hits:
        for name, callback, help_text in self.choices():
            yield DiscoveryHit(name, callback, help=help_text)

    async def search(self, query: str) -> Hits:
        matcher = self.matcher(query)
        for name, callback, help_text in self.choices():
            if score := matcher.match(name):
                yield Hit(score, matcher.highlight(name), callback, help=help_text)


def show_environment_palette(app):
    app.push_screen(
        CommandPalette(
            providers=[EnvironmentProvider], placeholder="Switch environment…"
        )
    )
