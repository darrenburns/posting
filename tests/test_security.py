"""Security tests for P0/P1 vulnerability fixes."""

import sys
import textwrap
from pathlib import Path

import pytest
import yaml

from posting.yaml import Loader, load
from posting.collection import (
    Collection,
    RequestModel,
    _sanitize_path_component,
    _unique_safe_name,
)
from posting.importing.postman import process_item, RequestItem
from posting.scripts import execute_script, clear_module_cache


class TestYamlSafeLoader:
    """Verify that the YAML loader rejects unsafe deserialization payloads."""

    def test_rejects_python_object_apply(self, tmp_path: Path) -> None:
        """A !!python/object/apply tag must not execute code."""
        malicious = tmp_path / "evil.posting.yaml"
        malicious.write_text(
            '!!python/object/apply:os.system ["echo PWNED"]'
        )
        with pytest.raises(yaml.constructor.ConstructorError):
            with open(malicious) as f:
                load(f, Loader=Loader)

    def test_rejects_python_object(self, tmp_path: Path) -> None:
        """A !!python/object tag must not instantiate arbitrary classes."""
        malicious = tmp_path / "evil.posting.yaml"
        malicious.write_text(
            "!!python/object:os.system\nargs: ['echo PWNED']"
        )
        with pytest.raises(yaml.constructor.ConstructorError):
            with open(malicious) as f:
                load(f, Loader=Loader)

    def test_rejects_python_module(self, tmp_path: Path) -> None:
        """A !!python/module tag must be rejected."""
        malicious = tmp_path / "evil.posting.yaml"
        malicious.write_text("!!python/module:os")
        with pytest.raises(yaml.constructor.ConstructorError):
            with open(malicious) as f:
                load(f, Loader=Loader)

    def test_loads_safe_yaml(self, tmp_path: Path) -> None:
        """Normal YAML data must still load correctly."""
        safe = tmp_path / "safe.yaml"
        safe.write_text(
            textwrap.dedent("""\
                name: Test Request
                method: GET
                url: https://example.com
                headers:
                  - name: Accept
                    value: application/json
            """)
        )
        with open(safe) as f:
            data = load(f, Loader=Loader)
        assert data["name"] == "Test Request"
        assert data["method"] == "GET"


class TestThemeYamlSafeLoader:
    """Verify theme loading rejects unsafe YAML."""

    def test_rejects_python_object_in_theme(self, tmp_path: Path) -> None:
        malicious_theme = tmp_path / "evil_theme.yaml"
        malicious_theme.write_text(
            '!!python/object/apply:os.system ["echo PWNED"]'
        )
        with pytest.raises(yaml.constructor.ConstructorError):
            yaml.safe_load(malicious_theme.read_text())

    def test_loads_safe_theme(self, tmp_path: Path) -> None:
        safe_theme = tmp_path / "safe_theme.yaml"
        safe_theme.write_text(
            textwrap.dedent("""\
                primary: "#ff0000"
                secondary: "#00ff00"
                background: "#000000"
            """)
        )
        data = yaml.safe_load(safe_theme.read_text())
        assert data["primary"] == "#ff0000"


class TestScriptPathTraversal:
    """Verify that script execution blocks path traversal attempts."""

    def test_rejects_parent_directory_traversal(self, tmp_path: Path) -> None:
        """Scripts using ../ to escape the collection root must be rejected."""
        collection_root = tmp_path / "collection"
        collection_root.mkdir()

        # Create a script outside the collection root
        evil_script = tmp_path / "evil.py"
        evil_script.write_text("def setup(): pass")

        with pytest.raises(FileNotFoundError, match="outside the collection root"):
            execute_script(
                collection_root=collection_root,
                script_path=Path("../evil.py"),
                function_name="setup",
            )

    def test_rejects_absolute_path_traversal(self, tmp_path: Path) -> None:
        """Absolute paths outside the collection must be rejected."""
        collection_root = tmp_path / "collection"
        collection_root.mkdir()

        evil_script = tmp_path / "evil.py"
        evil_script.write_text("def setup(): pass")

        # An absolute path that resolves outside collection_root
        with pytest.raises(FileNotFoundError):
            execute_script(
                collection_root=collection_root,
                script_path=Path(str(evil_script)),
                function_name="setup",
            )

    def test_rejects_symlink_escape(self, tmp_path: Path) -> None:
        """A symlink pointing outside the collection root must be rejected."""
        collection_root = tmp_path / "collection"
        collection_root.mkdir()

        # Create a script outside the collection
        outside_script = tmp_path / "outside.py"
        outside_script.write_text("def setup(): pass")

        # Create a symlink inside the collection pointing outside
        symlink = collection_root / "sneaky.py"
        symlink.symlink_to(outside_script)

        with pytest.raises(FileNotFoundError, match="outside the collection root"):
            execute_script(
                collection_root=collection_root,
                script_path=Path("sneaky.py"),
                function_name="setup",
            )

    def test_allows_valid_script_in_collection(self, tmp_path: Path) -> None:
        """A legitimate script within the collection root must work."""
        clear_module_cache()
        collection_root = tmp_path / "collection"
        collection_root.mkdir()

        valid_script = collection_root / "my_script.py"
        valid_script.write_text("def setup(): return 'ok'")

        fn = execute_script(
            collection_root=collection_root,
            script_path=Path("my_script.py"),
            function_name="setup",
        )
        assert fn is not None
        assert fn() == "ok"

    def test_resolved_collection_root_with_symlink(self, tmp_path: Path) -> None:
        """Collection root containing symlinks must be resolved before check."""
        clear_module_cache()
        real_dir = tmp_path / "real_collection"
        real_dir.mkdir()

        valid_script = real_dir / "script.py"
        valid_script.write_text("def setup(): return 'works'")

        # Symlinked collection root
        symlinked_root = tmp_path / "symlinked_collection"
        symlinked_root.symlink_to(real_dir)

        fn = execute_script(
            collection_root=symlinked_root,
            script_path=Path("script.py"),
            function_name="setup",
        )
        assert fn is not None
        assert fn() == "works"


class TestPostmanFolderSanitization:
    """Verify that Postman import sanitizes folder names to prevent path traversal."""

    def _make_folder_item(self, name: str) -> RequestItem:
        """Create a minimal Postman folder item."""
        return RequestItem(
            name=name,
            item=[],  # empty folder
            request=None,
        )

    def test_strips_path_traversal(self, tmp_path: Path) -> None:
        """Folder names with ../ must be sanitized in both path and name."""
        parent = Collection(path=tmp_path, name="root")
        item = self._make_folder_item("../../etc")

        process_item(item, parent, tmp_path)

        child = parent.children[0]
        # The path must NOT contain ../
        assert ".." not in str(child.path)
        # The name used for save_to_disk must also be sanitized
        assert ".." not in child.name
        # The path must stay within tmp_path
        assert child.path.resolve().is_relative_to(tmp_path.resolve())

    def test_strips_slashes(self, tmp_path: Path) -> None:
        """Folder names with path separators must be sanitized."""
        parent = Collection(path=tmp_path, name="root")
        item = self._make_folder_item("foo/bar\\baz")

        process_item(item, parent, tmp_path)

        child = parent.children[0]
        assert "/" not in child.path.name
        assert "\\" not in child.path.name

    def test_empty_name_gets_default(self, tmp_path: Path) -> None:
        """A folder name that sanitizes to empty must get a default name."""
        parent = Collection(path=tmp_path, name="root")
        item = self._make_folder_item("///")

        process_item(item, parent, tmp_path)

        child = parent.children[0]
        assert child.path.name == "unnamed"

    def test_preserves_normal_names(self, tmp_path: Path) -> None:
        """Normal folder names must not be altered."""
        parent = Collection(path=tmp_path, name="root")
        item = self._make_folder_item("User Details")

        process_item(item, parent, tmp_path)

        child = parent.children[0]
        assert child.path == tmp_path / "User Details"

    def test_deduplicates_colliding_names(self, tmp_path: Path) -> None:
        """Folders that sanitize to the same name must get unique paths."""
        parent = Collection(path=tmp_path, name="root")
        item1 = self._make_folder_item("foo/bar")
        item2 = self._make_folder_item("foobar")

        process_item(item1, parent, tmp_path)
        process_item(item2, parent, tmp_path)

        names = [c.name for c in parent.children]
        assert len(names) == 2
        assert len(set(names)) == 2  # all unique

    def test_deduplicates_multiple_unnamed(self, tmp_path: Path) -> None:
        """Multiple folders that sanitize to 'unnamed' must get unique paths."""
        parent = Collection(path=tmp_path, name="root")
        item1 = self._make_folder_item("///")
        item2 = self._make_folder_item("***")

        process_item(item1, parent, tmp_path)
        process_item(item2, parent, tmp_path)

        names = [c.name for c in parent.children]
        assert len(names) == 2
        assert names[0] == "unnamed"
        assert names[1] == "unnamed_2"

    def test_preserves_hyphens_and_underscores(self, tmp_path: Path) -> None:
        """Hyphens and underscores are valid in folder names."""
        parent = Collection(path=tmp_path, name="root")
        item = self._make_folder_item("my-api_v2")

        process_item(item, parent, tmp_path)

        child = parent.children[0]
        assert child.path == tmp_path / "my-api_v2"


class TestSaveToDiskSanitization:
    """Verify that Collection.save_to_disk sanitizes names used as paths."""

    def test_request_name_traversal_blocked(self, tmp_path: Path) -> None:
        """A request with ../ in the name must not escape the output dir."""
        collection = Collection(
            path=tmp_path,
            name="root",
            requests=[
                RequestModel(
                    name="../../etc/evil",
                    method="GET",
                    url="http://example.com",
                ),
            ],
        )
        collection.save_to_disk(tmp_path)

        # Must not write outside tmp_path
        assert not (tmp_path / ".." / ".." / "etc").exists()
        # Should have written a sanitized file inside tmp_path
        written = list(tmp_path.glob("*.posting.yaml"))
        assert len(written) == 1
        assert ".." not in written[0].name

    def test_child_name_traversal_blocked(self, tmp_path: Path) -> None:
        """A child collection with ../ in the name must not escape the output dir."""
        collection = Collection(
            path=tmp_path,
            name="root",
            children=[
                Collection(
                    path=tmp_path / "child",
                    name="../../escape",
                    requests=[
                        RequestModel(
                            name="test",
                            method="GET",
                            url="http://example.com",
                        ),
                    ],
                ),
            ],
        )
        collection.save_to_disk(tmp_path)

        # Must not create directories outside tmp_path
        assert not (tmp_path / ".." / ".." / "escape").exists()
        # Should have created a sanitized subdirectory
        subdirs = [p for p in tmp_path.iterdir() if p.is_dir()]
        assert len(subdirs) == 1
        assert ".." not in subdirs[0].name

    def test_openapi_style_name_sanitized(self, tmp_path: Path) -> None:
        """Names from OpenAPI specs (with slashes) must be sanitized."""
        collection = Collection(
            path=tmp_path,
            name="root",
            requests=[
                RequestModel(
                    name="/api/users/{id}",
                    method="GET",
                    url="http://example.com",
                ),
            ],
        )
        collection.save_to_disk(tmp_path)

        written = list(tmp_path.glob("*.posting.yaml"))
        assert len(written) == 1
        assert "/" not in written[0].name

    def test_duplicate_request_names_deduplicated(self, tmp_path: Path) -> None:
        """Requests whose names sanitize to the same value must not collide."""
        collection = Collection(
            path=tmp_path,
            name="root",
            requests=[
                RequestModel(name="foo/bar", method="GET", url="http://a.com"),
                RequestModel(name="foobar", method="GET", url="http://b.com"),
            ],
        )
        collection.save_to_disk(tmp_path)

        written = list(tmp_path.glob("*.posting.yaml"))
        assert len(written) == 2
        assert len({f.name for f in written}) == 2  # unique filenames


class TestSanitizePathComponent:
    """Unit tests for _sanitize_path_component."""

    def test_strips_traversal(self) -> None:
        assert ".." not in _sanitize_path_component("../../etc")

    def test_strips_slashes(self) -> None:
        result = _sanitize_path_component("foo/bar\\baz")
        assert "/" not in result
        assert "\\" not in result

    def test_empty_returns_unnamed(self) -> None:
        assert _sanitize_path_component("///") == "unnamed"

    def test_preserves_normal_names(self) -> None:
        assert _sanitize_path_component("My API v2") == "My API v2"

    def test_unique_safe_name_deduplicates(self) -> None:
        used: set[str] = set()
        first = _unique_safe_name("test", used)
        second = _unique_safe_name("test", used)
        assert first == "test"
        assert second == "test_2"
        assert len(used) == 2


class TestSysPathIsolation:
    """Verify that script loading doesn't pollute sys.path."""

    def test_sys_path_restored_after_execution(self, tmp_path: Path) -> None:
        """sys.path must not retain the script directory after execution."""
        clear_module_cache()
        collection_root = tmp_path / "collection"
        collection_root.mkdir()

        script = collection_root / "test_path.py"
        script.write_text("def setup(): pass")

        execute_script(
            collection_root=collection_root,
            script_path=Path("test_path.py"),
            function_name="setup",
        )

        # The script directory should not remain in sys.path
        assert str(collection_root) not in sys.path

    def test_sys_path_restored_after_error(self, tmp_path: Path) -> None:
        """sys.path must be restored even if the script raises an error."""
        clear_module_cache()
        collection_root = tmp_path / "collection"
        collection_root.mkdir()

        script = collection_root / "bad_script.py"
        script.write_text("raise RuntimeError('boom')")

        with pytest.raises(RuntimeError, match="boom"):
            execute_script(
                collection_root=collection_root,
                script_path=Path("bad_script.py"),
                function_name="setup",
            )

        assert str(collection_root) not in sys.path
