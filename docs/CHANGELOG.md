## 2.1.1 [12th November 2024]

### Fixed

- Fix collection browser message not being visible when it's empty.

### Changed

- Improved message in empty collection browser, indicating keybind for how to toggle the collection browser.

## 2.1.0 [11th November 2024]

### Added

- Import curl command by pasting it into the URL bar.

### Changed

- Collection browser width now adjusts based on content, so it doesn't waste a lot of space on larger screens.
- Info tab now shows "Request not saved to disk" if a request has not been saved to disk, rather than "None" or nothing at all.

## 2.0.0 [18th October 2024]

### Added

- **Scripting**: Run Python scripts before and after sending requests. Scripts can be used to perform setup, set variables, modify requests, and more.
    - Define "setup", "pre-request" and "post-request" Python functions and attach them to requests.
    - Posting will automatically reload these functions when they change, meaning you can edit them in an external editor while Posting is running.
    - Scripts can be used to directly manipulate the request, set variables which are used in the request (e.g. set a `$token` variable which is used in the request URL).
    - Output from scripts is captured and displayed in the "Scripts" tab.
- **Keymaps**: Change the default keybindings for any of Posting's "global" actions (e.g. sending request, opening jump mode, etc.) by editing `keymap` section of your `config.yaml` file.
- Added `heading.hostname` config to allow customisation of the hostname in the header. This field supports Rich markup. You may wish to use this to apply highlighting when `posting` is running on a production system vs a development environment, for example.
- Added `focus.on_request_open` config to automatically shift focus when a request is opened via the collection browser. For example, you might prefer to have focus jump to the "Body" tab when a request is opened.
- More detail and screenshots added to several sections of the guide.
    - Much more detail added to the "Getting Started" section.
    - Collections guide updated to explain more about the collection browser.
    - Guide for Keymaps added.
    - Guide for Scripting added.
    - Guide for External Tools added (integrating with vim, less, fx, etc.)
- `alt`+`enter` can now be used to send a request (in addition to the existing `ctrl+j` binding).
- Tooltips added to more actions in the app footer. These appear on mouse hover.

### Changed

- Automatically apply `content-type` header based on the body type selected in the UI.
- Updated to Textual 0.83.0
- Various refinements to autocompletion, upgrading to textual-autocomplete 3.0.0a12.
- Dependency specifications loosened on several dependencies.
- Recommended installation method changed from rye to uv.

### Fixed

- Fixed double rendering in "jump mode" overlay.
- Fixed sidebar not working on mobile on https://posting.sh
- Fixed autocompletion appearing when on 1 item in the list and the "search string" is equal to that item.

## 1.13.0 [8th September 2024]

### Added

- New `collection_browser.show_on_startup` config to control whether the collection browser is shown on startup.
- Watch for changes to loaded dotenv files and reload UI elements that depend on them when they change.

### Changed

- Upgraded all dependencies.
- Remove `pydantic-settings` crash workaround on empty config files.
- Renaming `App.maximized` as it now clashes with a Textual concept.
- Removed "using default collection" message from startup.

### Fixed

- Fixed crash while rendering error message on timeout.

## 1.12.3 [4th September 2024]

### Fixed

- Fixed crash when no clipboard tool installed

## 1.12.2 [4th September 2024]

*This release was yanked. It's functionally identical to 1.12.1.*

## 1.12.1 [21st August 2024]

### Fixed

- Fix "invalid escape sequence" warnings on Python 3.12+
- Fix buttons in request deletion confirmation modal not being usable with the enter key.

## 1.12.0 [17th August 2024]

### Added

- Colour-coding of methods in the collection browser.
- Added FAQ to website.

## 1.11.0 [15th August 2024]

### Added

- This file, `CHANGELOG.md`.
- Launch docs website.
- Duplicate request (with new request popup) under cursor in tree with ++d++.
- "Quick" duplicate request (without new request popup, request name is auto-generated) under cursor in tree with ++shift+d++.
- Delete request (with confirmation modal) under cursor in tree with ++backspace++.
- "Quick" delete request (without confirmation modal) under cursor in tree with ++shift+backspace++.
- "Quit Posting" added to command palette.
- Move the sidebar to the right or left using `collection_browser.position: 'right' | 'left'` config.
- Refinements to "galaxy" theme.
- "galaxy" theme is now default.
- Help text added to "empty state" in the collection browser.
- Extend info in the "Collection Browser" help modal.
- Visual indicator (a red bar on the left) on Input fields that contain invalid values.
- Toast message now appears when trying to submit the 'new request' modal with invalid values.
- Public roadmap (initial brain-dump version).

### Fixed
- Ensure the location of the request on disk in the `Info` tab wraps instead of clipping out of view.
- Inserting requests in sorted position on creation.
- Prevent creating requests with no name.
- Prevent writing paths in the file-name field in the new request modal.
- Prevent specifying paths outside of the open collection dir in the directory field in the new request modal.
- Fix variables not being substituted into several fields, including auth.

### Changed

- Upgrade to Textual version 0.76.0
- Change logic to render bindings in help modal to reflect new Textual API.
- Sort order of requests in the tree improved.

---

!!! note
    Changes prior to 1.11.0 are not documented here.
    Please see the [Releases page](https://github.com/darrenburns/posting/releases) on GitHub for information on changes prior to 1.11.0.