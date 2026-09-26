import asyncio
from pathlib import Path

import pytest
from textual.command import Command, CommandInput, CommandList

from posting.__main__ import make_posting
from posting.method_styles import get_method_style


@pytest.mark.parametrize(
    ("query", "expected"),
    [
        ("bold", []),
        ("method-color", []),
        ("delete", ["DELETE  delete a post"]),
        ("get all", ["GET     get all"]),
        ("GET", ["GET     get all", "GET     get random user"]),
        ("draft", ["POST    report [draft]"]),
        ("POST", ["POST    report [draft]", "DELETE  delete a post"]),
    ],
)
def test_request_search_matches_visible_text(tmp_path, monkeypatch, query, expected):
    monkeypatch.setenv(
        "POSTING_CONFIG_FILE",
        str(Path(__file__).parent / "sample-configs" / "general.yaml"),
    )
    for filename, method, name in [
        ("get", "GET", "get all"),
        ("get-random", "GET", "get random user"),
        ("delete", "DELETE", "delete a post"),
        ("report", "POST", "report [draft]"),
    ]:
        (tmp_path / f"{filename}.posting.yaml").write_text(
            f"name: {name}\nmethod: {method}\nurl: https://example.com/{filename}\n"
        )

    async def check_search():
        app = make_posting(collection=tmp_path, env=())
        async with app.run_test() as pilot:
            await pilot.press("ctrl+shift+p")
            search_query = (
                get_method_style(app.theme_variables, "GET").lstrip("#")
                if query == "method-color"
                else query
            )
            app.screen.query_one(CommandInput).value = search_query
            await pilot.pause()
            await app.workers.wait_for_complete()
            await pilot.pause()
            results = app.screen.query_one(CommandList)
            hits = [
                option.hit
                for index in range(results.option_count)
                if isinstance(option := results.get_option_at_index(index), Command)
            ]
            assert sorted(hit.text for hit in hits) == sorted(expected)
            for hit in hits:
                assert hit.prompt.plain == hit.text

            if query == "get all":
                display = hits[0].prompt
                match_style = app.screen.get_visual_style(
                    "command-palette--highlight", partial=True
                )
                highlighted = {
                    offset
                    for span in display.spans
                    if span.style == match_style
                    for offset in range(span.start, span.end)
                }
                assert highlighted == {8, 9, 10, 12, 13, 14}
                method_style = get_method_style(app.theme_variables, "GET")
                assert any(
                    span.start == 0
                    and span.end == 7
                    and span.style == f"{method_style} bold"
                    for span in display.spans
                )
                await pilot.press("down", "enter")
                await pilot.pause()
                assert app.screen.url_input.value == "https://example.com/get"
                assert app.screen.collection_tree.cursor_node.data.name == "get all"

    asyncio.run(check_search())
