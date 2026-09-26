import pytest


@pytest.fixture(autouse=True)
def isolated_history(tmp_path, monkeypatch):
    """Never read or write the developer's response history from a test."""
    monkeypatch.setattr("posting.history.data_directory", lambda: tmp_path / "data")
