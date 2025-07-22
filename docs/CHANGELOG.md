## 2.7.1 [22nd July 2025]

### Fixed

- Pin tree-sitter to <0.25.0 to fix crash due to breaking API change.

## 2.7.0 [19th April 2025]

### Added

- Collections can now be imported from Postman [(#106)](https://github.com/darrenburns/posting/pull/106)
- `posting.env` files in the current directory will automatically be loaded if no `--env` options are provided [(#249)](https://github.com/darrenburns/posting/pull/249)
- Generate JSON body with default values after importing OpenAPI specs [(#247)](https://github.com/darrenburns/posting/pull/247)
- Use tags to separate sub-collections in OpenAPI specs [(#247)](https://github.com/darrenburns/posting/pull/247)
- curl import now handles various different data options [(#252)](https://github.com/darrenburns/posting/pull/252)
- Added `-c` shorthand alias for `--collection` option [(#250)](https://github.com/darrenburns/posting/pull/250)
- Added `posting sponsors` command, which lists people who have supported Posting via GitHub Sponsors (specific sponsor tiers only). [(#253)](https://github.com/darrenburns/posting/pull/253)

### Changed

- A double-click rather than a single-click is now required to enter edit mode inside data tables (e.g. headers, query params, etc.) [(#256)](https://github.com/darrenburns/posting/pull/256)
- In the body editor, the `Form data` menu item now shows `(x-www-form-urlencoded)` after the label, to suggest the type of data being sent. [(#259)](https://github.com/darrenburns/posting/pull/259)

### Fixed

- Exported curl command uses `-d` for form data now, rather than `-F`. The result is exported commands with form data will now use `application/x-www-form-urlencoded` instead of `multipart/form-data` (matching Posting's behaviour). [(#252)](https://github.com/darrenburns/posting/pull/252)
- Fix attempting to add a protocol before applying variables in the URL bar [(#248)](https://github.com/darrenburns/posting/pull/248)
- Fix script path with custom function [(#254)](https://github.com/darrenburns/posting/pull/254)
- Fixed Posting's default `User-Agent` header not being used [(#259)](https://github.com/darrenburns/posting/pull/259)

## 2.6.0 [29th March 2025]

### Added

- Added `spacing: <compact|standard>` config to allow for a more compact UI (default: `standard`).
- Added ability to edit headers, form data, and query params without deleting and recreating them.
    - Press `enter` or click a row to enter edit mode. The row will be highlighted, and focus will move to the key/name input.
        Submitted changes will overwrite the existing row rather than adding a new one.
    - Press `v` to enter edit mode and immediately focus on the value input.
    - Press `escape` to cancel the edit.
    - Press `enter` to save the changes and exit edit mode.
    - The background colour behind the input will update to indicate edit mode is active.
- Added autocompletion for header values based on the header name.
    - For example, if the header is `Content-Type`, when typing the value, you'll be able to quickly autocomplete from a list of common content types.
- Press `/` to open the request search palette while the collection browser is focused.
    - `ctrl+shift+p` remains available as a global shortcut for this.
- Added `help: Open web docs` command to the command palette.
- Basic Vim motions to script output log (`hjkl`).
- Added response status code label to the URL bar.
- Contributing guide added to the GitHub repo (`CONTRIBUTING.md`).

### Changed

- Rewrite of the autocompletion system used to autocomplete header names, values, URLs, and variables.
- Automatically prepend `http://` protocol if no protocol is specified in the URL bar.
- Debounce jump overlay recomposition - if you resize while the jump overlay is open, it'll wait a short period before recomputing.
- Jump mode now lives on the main screen, rather than globally. This makes more sense as it's only available on the main screen. The only user-facing impact should be that the position of the keybinding in the footer and keybindings panel may change.
- Jump mode interaction with Tabs now uses the Tabs API, rather than simulating a Click.
- Status code label now displayed in the URL bar beside the trace markers.
- Request description area at the bottom of the collection browser has new design.
- When there are no response cookies, the cookies section will now display a message to the user.
- Updated to Textual 3.0.0.
- Various updates to the https://posting.sh homepage.

### Fixed

- Fixed scrolling in response headers and cookies tabs using keyboard.
- Fixed crash when immediately pressing enter after loading the UI when the `on_startup` config is set to `url` (this was due to lazy loading of the UI, and attempting to send a request before the UI was fully loaded).
- Fixed accepting a completion via enter in the header editor also adding the header.
    - Now, the first enter press will accept the completion, and the second enter press will add the header.
- Fixed Textual markup not being escaped key value tables.
- Fixed request description attempting to parse Textual markup [(#243)](https://github.com/darrenburns/posting/pull/243)

## 2.5.4 [13th March 2025]

### Fixed

- Prioritise user-defined `User-Agent` header over Posting's default.

## 2.5.3 [13th March 2025]

### Changed

- Lazily load content of tabs which are hidden on startup (100ms saved at startup).
- Only import openapi-pydantic when importing OpenAPI specs via `posting import` (63ms saved at startup).
- Pin httpx and patch out httpx._main to prevent slow import (20ms saved at startup).
- Defer import of watchfiles until app is running (6ms saved at startup).
- Defer `HelpScreen` import until it's used (10ms saved at startup).

## 2.5.2 [8th March 2025]

### Added

- Validation and corresponding UI feedback in New Request modal.

### Fixed

- Fixed crash when attempting to delete a request that doesn't exist on disk.
- Fixed being able to create requests with empty names.

## 2.5.1 [7th March 2025]

### Fixed

- Fixed importing of `max-time` from curl commands.

### Changed

- When importing a request from curl, request metadata (name, description) will not be overwritten.

## 2.5.0 [7th March 2025]

### Added

- Added bearer token auth support in the `Auth` tab.
- Added support for importing securitySchemes in OpenAPI specs.

### Changed

- OpenAPI specs are now parsed using an external library (`openapi-pydantic`).

## 2.4.1 [6th March 2025]

### Added

- Added command palette option to export the request as a curl command *without* running setup scripts.
- Added new documentation on working with the response in Posting and via external pagers/editors.
- Added new documentation on saving requests into folder structures from within Posting.

### Fixed

- Fixed crash when toggling rows via clicking the checkbox
- Fixed erroneous trailing ellipsis in `Info` tab.
- Fixed error messages in toasts on read and write timeouts.

### Changed

- Variables will be substituted into exported curl commands.
    - Undefined variables will be left as is (e.g. `$foo` will be left as `$foo` in the curl command)
- Setup scripts will now run by default when exporting to curl.
- Do not focus the URL bar when an error occurs on sending a request.
- Small visual refinement in `Scripts` tab.

## 2.4.0 [2nd March 2025]

### Added

- Added ability to toggle rows in tables on and off (press `space` or click the checkbox to toggle).
- Added "Export to curl" option in the command palette, to copy the request to your clipboard as a curl command.
- Added `curl_export_extra_args` config to allow for adding extra arguments to the curl command copied to your clipboard.
- Allow for customisation of "open in editor" and "open in pager" keys (`open-in-editor` and `open-in-pager` in the keymap).
- Added ability to quickly search for request by name and jump to it (press `ctrl+shift+p` to open the search popup).
- Added configurable keybinding `search-requests` (default: `ctrl+shift+p`).
- A few more screenshots were added to the "Navigation" guide.
- Added new headers to autocompletion: `Accept-Charset`, `DNT`, `Upgrade`, `Sec-Fetch-Site`, `Sec-Fetch-Mode`, `Sec-Fetch-User`, `Sec-Fetch-Dest`, and `Service-Worker-Navigation-Preload`.
- Removed some headers from autocompletion (due to being deprecated or response-only headers).

### Changed

- Upgraded Textual from version 0.86.0 to 2.1.1.

### Fixed

- Fixed variable preview not being shown below URL bar when cursor is over a variable.
- Fixed `ctrl+?` keybinding not opening contextual help on some terminals.

## 2.3.1 [1st March 2025]

### Changed

- Renamed "Change theme" to "Preview theme" in command palette, and update description to not imply the change persists across sessions (use the config file for persistent changes).

### Fixed

- Fixed crash when invalid syntax theme is specified. Posting now exits cleanly with an error message.
- Fixed toast message on copying text referring to "Response text" regardless of what text was copied.
- Fixed error handling and messaging when themes contain invalid syntax, invalid values. Includes batching errors and displaying multiple in one message.
- Fixed animation level config no longer being respected.
- Fixed missing `get_variable` method in scripting API that was described in docs but not implemented.

## 2.3.0 [19th November 2024]

### Added

- Editing a theme on disk will result in the UI refreshing in real-time to reflect changes.

## 2.2.0 [17th November 2024]

### Added

- Added 15 new themes (4 specific to Posting, 11 inherited from Textual's new theme system).
- Themes are now in submenu of command palette.
- Keybinding assistant can now be displayed as a sidebar, teaching you keybindings as you go.
- New tooltips when hovering over collection browser keybinds in the app footer.

### Changed 

- Syntax highlighting colours now derive automatically from the current theme.
- URL bar highlighting now derives automatically from the current theme.
- Method colour-coding in the collection browser is now derived automatically from the current theme.
- Jump mode UI has been refined to be more readable.
- Various refinements to existing themes.
- Options and descriptions in command palette reworded and reordered for clarity.
- Updated to Textual 0.86.1.

### Fixed

- Fixed error notification not rendering correctly when HTTP request times out.

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
