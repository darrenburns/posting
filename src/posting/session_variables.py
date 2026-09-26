"""Session variable edits and their undo history, independent of the UI."""

from dataclasses import dataclass


@dataclass
class _VariableChange:
    name: str
    had_override: bool
    previous_value: object


class SessionVariables:
    """Own session values and retain the last 50 edits made through the editor.

    Scripts may write to ``values`` directly without creating undo entries.
    The app overlays these values on the environment after editor mutations.
    """

    def __init__(self) -> None:
        self.values: dict[str, object] = {}
        self._history: list[_VariableChange] = []
        self._redo: list[_VariableChange] = []

    def _snapshot(self, name: str) -> _VariableChange:
        return _VariableChange(name, name in self.values, self.values.get(name))

    def _record(self, name: str) -> None:
        self._history.append(self._snapshot(name))
        del self._history[:-50]
        self._redo.clear()

    def set(self, name: str, value: str) -> None:
        self._record(name)
        self.values[name] = value

    def revert(self, name: str) -> bool:
        if name not in self.values:
            return False
        self._record(name)
        del self.values[name]
        return True

    def _restore(self, change: _VariableChange) -> str:
        if change.had_override:
            self.values[change.name] = change.previous_value
        else:
            self.values.pop(change.name, None)
        return change.name

    def undo(self) -> str | None:
        if not self._history:
            return None
        change = self._history.pop()
        self._redo.append(self._snapshot(change.name))
        return self._restore(change)

    def redo(self) -> str | None:
        if not self._redo:
            return None
        change = self._redo.pop()
        self._history.append(self._snapshot(change.name))
        return self._restore(change)
