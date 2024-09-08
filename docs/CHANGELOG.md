## 1.13.0 [8th September 2024]

### Added

- New `collection_browser.show_on_startup` config to control whether the collection browser is shown on startup.
- Watch for changes to loaded dotenv files and reload UI elements that depend on them when they change.

### Changed

- Upgraded all dependencies
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