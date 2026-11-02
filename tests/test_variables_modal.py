from posting.widgets.variables_editor import build_rows, is_sensitive


def test_sensitive_names_are_recognised() -> None:
    assert is_sensitive("DP_PASSWORD")
    assert is_sensitive("CLIENT_SECRET")
    assert is_sensitive("TOKEN")
    assert is_sensitive("api_key")
    assert not is_sensitive("HOST")
    assert not is_sensitive("EXPORT_PROFILE")


def test_secrets_are_masked_by_default() -> None:
    rows = build_rows({"CLIENT_SECRET": "hunter2hunter2"}, session_names=set())
    name, value, _ = rows[0]
    assert name == "CLIENT_SECRET"
    assert "hunter2" not in value
    assert set(value) == {"•"}


def test_masking_does_not_leak_secret_length() -> None:
    short = build_rows({"TOKEN": "abc"}, session_names=set())[0][1]
    long = build_rows({"TOKEN": "a" * 500}, session_names=set())[0][1]
    assert short == long == "•" * 12


def test_secrets_shown_when_revealed() -> None:
    rows = build_rows(
        {"CLIENT_SECRET": "hunter2"}, session_names=set(), show_sensitive=True
    )
    assert rows[0][1] == "hunter2"


def test_source_distinguishes_session_from_env_file() -> None:
    rows = build_rows(
        {"HOST": "example.com", "JOB_ID": "abc-123"},
        session_names={"JOB_ID"},
    )
    sources = {name: source for name, _, source in rows}
    assert sources["JOB_ID"] == "session"
    assert sources["HOST"] == "env file"


def test_filter_matches_on_name_case_insensitively() -> None:
    variables = {"HOST": "a", "DP_BASE": "b", "NEXIS_IN": "c"}
    rows = build_rows(variables, session_names=set(), filter_query="nexis")
    assert [name for name, _, _ in rows] == ["NEXIS_IN"]

    assert build_rows(variables, session_names=set(), filter_query="zzz") == []


def test_rows_are_sorted_by_name() -> None:
    rows = build_rows({"ZED": "1", "ALPHA": "2"}, session_names=set())
    assert [name for name, _, _ in rows] == ["ALPHA", "ZED"]


def test_non_string_values_are_rendered() -> None:
    rows = build_rows({"COUNT": 42}, session_names=set())
    assert rows[0][1] == "42"
