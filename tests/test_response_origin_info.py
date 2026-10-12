"""Tests for the response origin info line (issue #320)."""

import asyncio
import httpx

from posting.__main__ import make_posting
from posting.widgets.response.response_area import ResponseArea
from test_snapshots import use_config, SAMPLE_COLLECTIONS


def _area(app) -> ResponseArea:
    return app.screen.query_one(ResponseArea)


@use_config("general.yaml")
def test_response_origin_info_shows_request():
    async def check():
        app = make_posting(collection=SAMPLE_COLLECTIONS)
        async with app.run_test(size=(120, 34)) as pilot:
            await pilot.pause()
            await pilot.pause()
            area = _area(app)
            origin_info = area.query_one("#response-origin-info")
            assert origin_info.display is False

            request = httpx.Request("GET", "https://alpha.example.com")
            area.response = httpx.Response(200, request=request)
            await pilot.pause()
            assert origin_info.display is True
            assert "GET https://alpha.example.com" in str(origin_info.render())

    asyncio.run(check())


@use_config("general.yaml")
def test_response_origin_info_updates_per_response():
    async def check():
        app = make_posting(collection=SAMPLE_COLLECTIONS)
        async with app.run_test(size=(120, 34)) as pilot:
            await pilot.pause()
            await pilot.pause()
            area = _area(app)
            origin_info = area.query_one("#response-origin-info")

            area.response = httpx.Response(
                200, request=httpx.Request("GET", "https://alpha.example.com")
            )
            await pilot.pause()
            first = str(origin_info.render())

            area.response = httpx.Response(
                200, request=httpx.Request("GET", "https://beta.example.com")
            )
            await pilot.pause()
            second = str(origin_info.render())

            assert first != second
            assert "beta.example.com" in second

    asyncio.run(check())
