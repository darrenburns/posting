import pytest

from posting.session_variables import SessionVariables


@pytest.mark.parametrize("value", [42, None, {"id": 1}, [1, 2]])
def test_undo_preserves_script_value(value):
    variables = SessionVariables()
    variables.values["VALUE"] = value
    variables.set("VALUE", "replacement")

    assert variables.undo() == "VALUE"
    assert variables.values["VALUE"] is value
    assert variables.redo() == "VALUE"
    assert variables.values["VALUE"] == "replacement"


def test_add_revert_and_repeated_undo_redo():
    variables = SessionVariables()
    variables.set("ITEM_ID", "101")
    variables.set("ITEM_ID", "202")
    assert variables.revert("ITEM_ID")
    assert "ITEM_ID" not in variables.values
    assert variables.undo() == "ITEM_ID"
    assert variables.values["ITEM_ID"] == "202"
    assert variables.undo() == "ITEM_ID"
    assert variables.values["ITEM_ID"] == "101"
    assert variables.undo() == "ITEM_ID"
    assert "ITEM_ID" not in variables.values
    for _ in range(3):
        assert variables.redo() == "ITEM_ID"
    assert "ITEM_ID" not in variables.values
    assert variables.redo() is None


def test_new_edit_invalidates_redo_and_history_is_bounded():
    variables = SessionVariables()
    for value in range(60):
        variables.set("COUNT", str(value))
    for _ in range(50):
        assert variables.undo() == "COUNT"
    assert variables.values["COUNT"] == "9"
    assert variables.undo() is None
    variables.set("COUNT", "new")
    assert variables.redo() is None
