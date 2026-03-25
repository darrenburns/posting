from pathlib import Path
from types import SimpleNamespace

from textual.widgets import Input
from textual_autocomplete import TargetState

from posting.__main__ import make_posting
from posting.widgets.load_env_file_dialog import (
    EnvFilePathAutoComplete,
    load_env_file,
)
from posting.variables import get_variables, load_variables


TESTS_DIR = Path(__file__).parent
ENV_DIR = TESTS_DIR / "sample-envs"
SAMPLE_COLLECTIONS = TESTS_DIR / "sample-collections"


def test_empty_input_includes_cwd_and_config_env_candidates(
    tmp_path, monkeypatch
) -> None:
    working_directory = tmp_path / "workspace"
    working_directory.mkdir()
    (working_directory / ".env").write_text("FROM_CWD=1\n", encoding="utf-8")
    (working_directory / ".env.dev").write_text("FROM_CWD_DEV=1\n", encoding="utf-8")
    (working_directory / "nested").mkdir()
    (working_directory / "notes.txt").write_text("ignore\n", encoding="utf-8")

    config_home = tmp_path / "xdg-config"
    posting_config = config_home / "posting"
    posting_config.mkdir(parents=True)
    config_env = posting_config / "config.env"
    config_env.write_text("FROM_CONFIG=1\n", encoding="utf-8")

    monkeypatch.setenv("XDG_CONFIG_HOME", str(config_home))

    auto_complete = EnvFilePathAutoComplete(Input(), working_directory=working_directory)

    candidates = auto_complete.get_candidates(TargetState(text="", cursor_position=0))
    values = [candidate.value for candidate in candidates]

    assert ".env" in values
    assert ".env.dev" in values
    assert "nested/" in values
    assert str(config_env.resolve()) in values
    assert "notes.txt" not in values
    assert values.index(".env") < values.index("nested/")
    assert values.index(".env.dev") < values.index("nested/")
    assert values.index(str(config_env.resolve())) < values.index("nested/")


def test_empty_input_candidates_use_relative_and_absolute_values(
    tmp_path, monkeypatch
) -> None:
    working_directory = tmp_path / "workspace"
    working_directory.mkdir()
    (working_directory / ".env").write_text("FROM_CWD=1\n", encoding="utf-8")

    config_home = tmp_path / "xdg-config"
    posting_config = config_home / "posting"
    posting_config.mkdir(parents=True)
    config_env = posting_config / "config.env"
    config_env.write_text("FROM_CONFIG=1\n", encoding="utf-8")

    monkeypatch.setenv("XDG_CONFIG_HOME", str(config_home))

    auto_complete = EnvFilePathAutoComplete(Input(), working_directory=working_directory)

    candidates = auto_complete.get_candidates(TargetState(text="", cursor_position=0))
    values = [candidate.value for candidate in candidates]

    assert ".env" in values
    assert str(config_env.resolve()) in values


def test_relative_input_browses_from_working_directory(tmp_path, monkeypatch) -> None:
    working_directory = tmp_path / "workspace"
    subdirectory = working_directory / "envs"
    working_directory.mkdir()
    subdirectory.mkdir()
    (subdirectory / "dev.env").write_text("NAME=dev\n", encoding="utf-8")
    (subdirectory / "nested").mkdir()

    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "xdg-config"))

    auto_complete = EnvFilePathAutoComplete(Input(), working_directory=working_directory)

    candidates = auto_complete.get_candidates(
        TargetState(text="envs/", cursor_position=len("envs/"))
    )

    values = [candidate.value for candidate in candidates]
    assert "dev.env" in values
    assert "nested/" in values
    assert values.index("dev.env") < values.index("nested/")


def test_absolute_input_browses_from_absolute_path(tmp_path, monkeypatch) -> None:
    absolute_directory = tmp_path / "absolute"
    absolute_directory.mkdir()
    (absolute_directory / "prod.env").write_text("NAME=prod\n", encoding="utf-8")
    (absolute_directory / "skip.txt").write_text("ignore\n", encoding="utf-8")

    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "xdg-config"))

    auto_complete = EnvFilePathAutoComplete(Input(), working_directory=tmp_path)

    candidates = auto_complete.get_candidates(
        TargetState(
            text=f"{absolute_directory}/",
            cursor_position=len(f"{absolute_directory}/"),
        )
    )

    values = [candidate.value for candidate in candidates]
    assert "prod.env" in values
    assert "skip.txt" not in values


def test_tilde_input_browses_from_home(tmp_path, monkeypatch) -> None:
    home_directory = tmp_path / "home"
    home_directory.mkdir()
    (home_directory / ".env.home").write_text("HOME_ENV=1\n", encoding="utf-8")
    (home_directory / "docs").mkdir()

    monkeypatch.setenv("HOME", str(home_directory))
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path / "xdg-config"))

    auto_complete = EnvFilePathAutoComplete(Input(), working_directory=tmp_path)

    candidates = auto_complete.get_candidates(
        TargetState(text="~/", cursor_position=len("~/"))
    )

    values = [candidate.value for candidate in candidates]
    assert ".env.home" in values
    assert "docs/" in values


def test_load_env_file_rejects_missing_paths(tmp_path) -> None:
    notifications: list[tuple[str, str | None]] = []
    published: list[None] = []
    app = SimpleNamespace(
        environment_files=(),
        settings=SimpleNamespace(use_host_environment=False),
        session_env={},
        env_changed_signal=SimpleNamespace(publish=published.append),
        notify=lambda message, severity=None: notifications.append((message, severity)),
    )

    loaded = load_env_file(
        app,
        "missing.env",
        working_directory=tmp_path,
    )

    assert loaded is False
    assert app.environment_files == ()
    assert published == []
    assert notifications == [
        (f"Environment file not found: {tmp_path / 'missing.env'}", "error")
    ]


def test_load_env_file_rejects_directories(tmp_path) -> None:
    notifications: list[tuple[str, str | None]] = []
    published: list[None] = []
    app = SimpleNamespace(
        environment_files=(),
        settings=SimpleNamespace(use_host_environment=False),
        session_env={},
        env_changed_signal=SimpleNamespace(publish=published.append),
        notify=lambda message, severity=None: notifications.append((message, severity)),
    )

    loaded = load_env_file(app, ".", working_directory=tmp_path)

    assert loaded is False
    assert app.environment_files == ()
    assert published == []
    assert notifications == [
        (f"Environment path is not a file: {tmp_path}", "error")
    ]


def test_load_env_file_updates_app_state_and_variables(tmp_path) -> None:
    env_file = tmp_path / "loaded.env"
    env_file.write_text("FROM_FILE=1\n", encoding="utf-8")

    notifications: list[tuple[str, str | None]] = []
    published: list[None] = []
    app = SimpleNamespace(
        environment_files=(),
        settings=SimpleNamespace(use_host_environment=False),
        session_env={"FROM_SESSION": "2"},
        env_changed_signal=SimpleNamespace(publish=published.append),
        notify=lambda message, severity=None: notifications.append((message, severity)),
    )

    loaded = load_env_file(app, "loaded.env", working_directory=tmp_path)

    assert loaded is True
    assert app.environment_files == (env_file.resolve(),)
    assert published == [None]
    assert notifications == [(f"Loaded environment from: {env_file.resolve()}", None)]
    assert get_variables()["FROM_FILE"] == "1"
    assert get_variables()["FROM_SESSION"] == "2"


def test_make_posting_reloads_env_variables_even_when_cache_is_populated(tmp_path) -> None:
    other_env = tmp_path / "other.env"
    other_env.write_text("OTHER=1\n", encoding="utf-8")
    load_variables((other_env,), use_host_environment=False, avoid_cache=True)

    env_path = str((ENV_DIR / "sample_base.env").resolve())
    make_posting(
        collection=SAMPLE_COLLECTIONS / "jsonplaceholder" / "todos",
        env=(env_path,),
    )

    variables = get_variables()
    assert variables["POST_ID"] == "1"
    assert variables["USER_ID"] == "2"
    assert "OTHER" not in variables
