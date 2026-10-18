from pathlib import Path

import pytest

from posting.posting3_notice import MARKER_NAME


@pytest.fixture(autouse=True)
def isolated_history(tmp_path, monkeypatch):
    """Never read or write the developer's response history from a test."""
    monkeypatch.setattr("posting.history.data_directory", lambda: tmp_path / "data")


@pytest.fixture(autouse=True)
def posting3_notice_marker(tmp_path, monkeypatch) -> Path:
    """Keep the one-time Posting 3 notice out of tests unless one removes this marker."""
    data = tmp_path / "data"
    data.mkdir(exist_ok=True)
    marker = data / MARKER_NAME
    marker.touch()
    monkeypatch.setattr("posting.posting3_notice.data_directory", lambda: data)
    return marker
