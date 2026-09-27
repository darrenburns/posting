"""Tests that the YAML loader used for collection files rejects dangerous tags.

These tests verify the hardening fix for CWE-502: deserializing untrusted
.posting.yaml files with PyYAML's unsafe loader allowed arbitrary code
execution via !!python/object/apply tags. The loader must now use safe_load
so that crafted collection files raise ConstructorError instead of executing
commands.
"""

from __future__ import annotations

import textwrap
from pathlib import Path

import pytest
import yaml

from posting.collection import load_request_from_yaml
from posting.yaml import Loader


def test_exported_loader_is_safe_loader():
    """The Loader exported from posting.yaml must be a safe constructor."""
    safe_loaders = (yaml.CSafeLoader, yaml.SafeLoader)
    assert issubclass(Loader, yaml.SafeLoader), (
        f"Loader must be a SafeLoader subclass, got {Loader!r}"
    )


def test_safe_load_rejects_python_object_apply_tag():
    """yaml.safe_load must reject !!python/object/apply tags."""
    malicious_yaml = "name: !!python/object/apply:os.system ['echo pwned']\n"
    with pytest.raises(yaml.constructor.ConstructorError):
        yaml.safe_load(malicious_yaml)


def test_safe_load_rejects_python_name_tag():
    """yaml.safe_load must reject !!python/name tags that resolve to callables."""
    malicious_yaml = "name: !!python/name:os.system\n"
    with pytest.raises(yaml.constructor.ConstructorError):
        yaml.safe_load(malicious_yaml)


def test_load_request_from_yaml_rejects_malicious_collection_file(tmp_path: Path):
    """Loading a crafted .posting.yaml must raise ConstructorError, not execute code."""
    malicious_file = tmp_path / "evil.posting.yaml"
    malicious_file.write_text(
        textwrap.dedent(
            """\
            name: !!python/object/apply:os.system ['echo pwned']
            method: GET
            url: https://example.com
            """
        ),
        encoding="utf-8",
    )
    with pytest.raises(yaml.constructor.ConstructorError):
        load_request_from_yaml(str(malicious_file))


def test_load_request_from_yaml_still_loads_benign_file(tmp_path: Path):
    """Normal .posting.yaml files must still load correctly after the fix."""
    benign_file = tmp_path / "normal.posting.yaml"
    benign_file.write_text(
        textwrap.dedent(
            """\
            name: my-request
            method: GET
            url: https://example.com/api
            """
        ),
        encoding="utf-8",
    )
    model = load_request_from_yaml(str(benign_file))
    assert model.name == "my-request"
    assert model.method == "GET"
