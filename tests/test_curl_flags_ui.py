from textual.events import Paste
from posting.__main__ import make_posting
from test_snapshots import use_config, SAMPLE_COLLECTIONS


@use_config("general.yaml")
def test_curl_flags_before_url_paste(snap_compare):
    app = make_posting(collection=SAMPLE_COLLECTIONS)

    async def run_before(pilot):
        await pilot.pause()
        app.screen.url_input.post_message(
            Paste(
                "curl --cookie 'session=example' --location --proxy 'http://proxy.example:8080' -H 'Accept: application/json' 'https://example.com/api'"
            )
        )
        await pilot.pause()
        assert app.screen.url_input.value == "https://example.com/api"
        assert (
            app.screen.build_request_model(app.screen.request_options.to_model())
            .headers[0]
            .value
            == "application/json"
        )
        app.clear_notifications()

    assert snap_compare(app, run_before=run_before, terminal_size=(120, 40))
