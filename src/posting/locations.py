from pathlib import Path

from xdg_base_dirs import xdg_config_home, xdg_data_home


def _posting_directory(root: Path) -> Path:
    directory = root / "posting"
    directory.mkdir(exist_ok=True, parents=True)
    return directory


def data_directory() -> Path:
    """Return (possibly creating) the application data directory."""
    return _posting_directory(xdg_data_home())


def default_collection_directory() -> Path:
    """Return (possibly creating) the default collection directory."""
    return data_directory() / "default"


def config_directory() -> Path:
    """Return (possibly creating) the application config directory."""
    return _posting_directory(xdg_config_home())


def config_file() -> Path:
    return config_directory() / "config.yaml"
