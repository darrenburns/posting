from pathlib import Path
import runpy

import pytest
import yaml

import posting.collection as collection
import posting.yaml as posting_yaml
from posting.collection import Collection, RequestBody, RequestModel, load_request_from_yaml


@pytest.fixture(params=["installed", "python"])
def loader(request, monkeypatch):
    if request.param == "python":
        monkeypatch.delattr(yaml, "CSafeLoader", raising=False)
        monkeypatch.delattr(yaml, "CLoader", raising=False)
    namespace = runpy.run_path(posting_yaml.__file__)
    monkeypatch.setattr(collection, "Loader", namespace["Loader"])
    return namespace["Loader"]


@pytest.mark.parametrize(
    "payload",
    [
        "!!python/object/apply:builtins.str [constructed]",
        "!!python/object/new:builtins.str [constructed]",
        "!!python/name:builtins.str",
        "!!python/module:builtins",
    ],
)
def test_rejects_python_tags(payload, loader, tmp_path: Path):
    with pytest.raises(yaml.constructor.ConstructorError):
        posting_yaml.load(payload, Loader=loader)

    path = tmp_path / "unsafe.posting.yaml"
    path.write_text(f"name: {payload}\nurl: https://example.com\n", encoding="utf-8")
    with pytest.raises(yaml.constructor.ConstructorError):
        load_request_from_yaml(str(path))


def test_collection_scan_does_not_execute_yaml(loader, monkeypatch, tmp_path: Path):
    calls = []
    monkeypatch.setattr("os.system", lambda command: calls.append(command) or 0)
    (tmp_path / "unsafe.posting.yaml").write_text(
        "name: !!python/object/apply:os.system ['unexpected command']\n",
        encoding="utf-8",
    )
    RequestModel(name="safe", url="https://example.com").save_to_disk(
        tmp_path / "safe.posting.yaml"
    )

    result = Collection.from_directory(str(tmp_path))

    assert calls == []
    assert [request.name for request in result.requests] == ["safe"]


def test_request_roundtrip_preserves_multiline_format(loader, tmp_path: Path):
    request = RequestModel(
        name="multiline",
        method="POST",
        url="https://example.com/api",
        body=RequestBody(content="first line\nsecond line"),
    )
    path = tmp_path / "normal.posting.yaml"
    request.save_to_disk(path)

    assert "content: |-\n    first line\n    second line\n" in path.read_text(
        encoding="utf-8"
    )
    restored = load_request_from_yaml(str(path))
    assert restored.model_dump(exclude={"path"}) == request.model_dump(exclude={"path"})
    assert restored.path == path
    assert posting_yaml.dump(
        {"text": "first line\nsecond line"}, Dumper=posting_yaml.Dumper
    ) == "text: |-\n  first line\n  second line\n"


def test_theme_loader_rejects_python_tags(tmp_path: Path):
    from posting.themes import InvalidThemeError, load_user_theme

    path = tmp_path / "unsafe.yaml"
    path.write_text(
        "name: unsafe\nprimary: '#ffffff'\nsecondary: !!python/name:os.system\n",
        encoding="utf-8",
    )

    with pytest.raises(InvalidThemeError, match="Could not parse theme file"):
        load_user_theme(path)
