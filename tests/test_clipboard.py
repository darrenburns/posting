from unittest.mock import Mock

import pyperclip

from posting.widgets.text_area import ReadOnlyTextArea


def make_text_area(text: str = "hello") -> Mock:
    text_area = Mock()
    text_area.visual_mode = False
    text_area.selected_text = ""
    text_area.text = text
    return text_area


def test_copy_uses_native_clipboard_when_available(monkeypatch) -> None:
    text_area = make_text_area()
    native_copy = Mock()
    monkeypatch.setattr(pyperclip, "copy", native_copy)

    ReadOnlyTextArea.action_copy_to_clipboard(text_area)

    native_copy.assert_called_once_with("hello")
    text_area.app.copy_to_clipboard.assert_not_called()
    text_area.notify.assert_called_once_with(
        "Copied (5 characters).", title="Text copied"
    )


def test_copy_falls_back_to_textual_clipboard(monkeypatch) -> None:
    text_area = make_text_area("full response")
    text_area.selected_text = "remote clipboard"

    def unavailable(_: str) -> None:
        raise pyperclip.PyperclipException("no native clipboard")

    monkeypatch.setattr(pyperclip, "copy", unavailable)

    ReadOnlyTextArea.action_copy_to_clipboard(text_area)

    text_area.app.copy_to_clipboard.assert_called_once_with("remote clipboard")
    text_area.notify.assert_called_once_with(
        "Copied 16 characters.", title="Selection copied"
    )
