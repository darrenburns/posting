import pytest
from posting.collection import RequestBody, RequestModel
from posting.variables import SubstitutionError


def test_literal_body_report_reproduces():
    request = RequestModel(
        url="https://example.com",
        body=RequestBody(content='{"$schema":"example","price":"$5"}'),
    )
    with pytest.raises(SubstitutionError):
        request.apply_template({})


def test_disable_body_substitution_preserves_dollars():
    from posting.collection import Options

    request = RequestModel(
        url="${BASE}/data",
        body=RequestBody(content='{"$schema":"example","price":"$5","escape":"$$"}'),
        options=Options(substitute_body_variables=False),
    )
    original = request.body.content
    request.apply_template({"BASE": "https://example.com"})
    assert request.url == "https://example.com/data"
    assert request.body.content == original


def test_form_body_can_be_literal_and_option_roundtrips(tmp_path):
    from posting.collection import Options, FormItem

    request = RequestModel(
        url="https://example.com",
        body=RequestBody(form_data=[FormItem(name="$filter", value="${literal} $$")]),
        options=Options(substitute_body_variables=False),
    )
    request.apply_template({})
    assert request.body.form_data[0].name == "$filter"
    assert request.body.form_data[0].value == "${literal} $$"
    path = tmp_path / "literal.posting.yaml"
    request.save_to_disk(path)
    from posting.yaml import load, Loader

    reloaded = RequestModel.model_validate(load(path.read_text(), Loader=Loader))
    assert reloaded.options.substitute_body_variables is False


from test_snapshots import use_config


@use_config("general.yaml")
def test_literal_body_option(snap_compare, tmp_path):
    from textual.widgets import Checkbox, TabbedContent
    from posting.__main__ import make_posting

    app = make_posting(collection=tmp_path)

    async def run_before(pilot):
        await pilot.pause()
        screen = app.screen
        screen.query_one(
            "RequestEditorTabbedContent", TabbedContent
        ).active = "options-pane"
        checkbox = screen.query_one("#substitute-body-variables", Checkbox)
        checkbox.focus()
        await pilot.press("space")
        assert not screen.request_options.to_model().substitute_body_variables
        screen.collection_browser.border_subtitle = "collection"

    assert snap_compare(app, run_before=run_before, terminal_size=(120, 40))
