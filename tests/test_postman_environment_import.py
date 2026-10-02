import json
from pathlib import Path

import pytest
from click.testing import CliRunner
from dotenv import dotenv_values

from posting.__main__ import cli
from posting.variables import SharedVariables, load_variables


def import_environment(tmp_path, spec, output=None):
    source = tmp_path / "source.json"
    source.write_text(json.dumps(spec), encoding="utf-8")
    args = ["import", "--type", "postman-env", str(source)]
    if output is not None:
        args.extend(["--output", str(output)])
    return CliRunner().invoke(cli, args)


@pytest.mark.parametrize("existing_directory", [False, True])
def test_import_environment_round_trip(tmp_path, monkeypatch, existing_directory):
    monkeypatch.setattr("posting.variables.VARIABLES", SharedVariables())
    output = tmp_path / "nested" / "output"
    if existing_directory:
        output.mkdir(parents=True)
    values = {
        "baseUrl": "https://example.com",
        "api-key": "abc #secret",
        "multiline": "first\nINJECTED=second\r\nlast",
        "quoted": "\"literal\" 'quote' \\n \\t \\path",
        "unicode": "café 🐍",
        "empty": "",
        "null": None,
    }
    result = import_environment(tmp_path, {
        "name": "Production Env",
        "values": [
            {"key": key, "value": value} for key, value in values.items()
        ] + [{"key": "disabled", "value": "ignored", "enabled": False}],
    }, output)
    assert result.exit_code == 0, result.output
    env_file = output / "Production Env.env"
    assert load_variables((env_file,), use_host_environment=False, avoid_cache=True) == {
        "BASE_URL": "https://example.com",
        "API_KEY": "abc #secret",
        "MULTILINE": "first\nINJECTED=second\r\nlast",
        "QUOTED": "\"literal\" 'quote' \\n \\t \\path",
        "UNICODE": "café 🐍",
        "EMPTY": "",
        "NULL": "",
    }


def test_import_empty_environment_defaults_to_current_directory(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    result = import_environment(tmp_path, {"values": []})
    assert result.exit_code == 0, result.output
    assert dotenv_values(tmp_path / "environment.env") == {}


@pytest.mark.parametrize("spec", [
    {},
    {"info": {"name": "Collection"}, "item": []},
    {"values": None},
    {"values": {}},
    {"values": [{"value": "secret"}]},
    {"values": [{"key": "bad\nKEY", "value": "secret"}]},
    {"values": [{"key": "a=b", "value": "secret"}]},
    {"values": [{"key": "1key", "value": "secret"}]},
    {"values": [{"key": "key", "value": {}, "enabled": True}]},
    {"values": [{"key": "key", "enabled": "invalid"}]},
    {"name": "../escaped", "values": []},
    {"name": "/absolute", "values": []},
    {"name": "..\\escaped", "values": []},
    {"name": "nul\0name", "values": []},
    {"name": "", "values": []},
    {"name": None, "values": []},
])
def test_reject_invalid_environment_without_writing(tmp_path, spec):
    output = tmp_path / "output"
    result = import_environment(tmp_path, spec, output)
    assert result.exit_code == 1
    assert "Could not import the Postman environment" in result.output
    assert "secret" not in result.output
    assert not output.exists()
    assert list(tmp_path.iterdir()) == [tmp_path / "source.json"]


def test_reject_invalid_json(tmp_path):
    source = tmp_path / "source.json"
    source.write_text("not json")
    output = tmp_path / "output"
    result = CliRunner().invoke(cli, [
        "import", "--type", "postman-env", str(source), "-o", str(output)
    ])
    assert result.exit_code == 1
    assert not output.exists()


def test_import_sample_environment(tmp_path):
    source = Path("tests/sample-importable-collections/sample.postman_environment.json")
    expected = Path("tests/sample-importable-collections/sample_output.env")
    result = import_environment(tmp_path, json.loads(source.read_text()), tmp_path)
    assert result.exit_code == 0, result.output
    assert dotenv_values(tmp_path / "Sample Local Env.env") == dotenv_values(expected)


@pytest.mark.parametrize("values", [
    [{"key": "first", "value": "trail\\"}, {"key": "second", "value": "next"}],
    [{"key": "api-key", "value": "a"}, {"key": "apiKey", "value": "b"}],
    [{"key": "literal", "value": "${POSTING_IMPORT_TEST_MISSING}"}],
])
def test_reject_lossy_environment_without_writing(tmp_path, monkeypatch, values):
    monkeypatch.delenv("POSTING_IMPORT_TEST_MISSING", raising=False)
    output = tmp_path / "output"
    result = import_environment(tmp_path, {"values": values}, output)
    assert result.exit_code == 1
    assert "Values must round-trip" in result.output
    assert not output.exists()
