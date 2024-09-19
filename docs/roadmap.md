## About this document

This is a very high level overview of what I'm planning on working on.

It's unrefined and unordered at the moment, but I would like to work with the community to refine and prioritize this list in the future.

If you have any feedback or suggestions, please open a new discussion on GitHub. Opening a discussion helps me understand what's important to the community, as others can upvote it if they're interested.

## The Roadmap

- Pre-request and post-response scripts and assertions. These can be embedded in the YAML or relative paths to files shared alongside the collection (relative to the collection root).
- Parse cURL commands.
- Watching environment files for changes & updating the UI. ✅
- Editing key/value editor rows without having to delete/re-add them.
- Saving recently used environments to a file.
- Saving recently used collections to a file.
- Viewing the currently loaded environment keys/values in a popup.
- Changing the environment at runtime - probably via command palette - push a new command palette screen where you can search for and select one of the previously used environments.
- Variable completion autocompletion TextAreas.
- Variable resolution highlighting in TextAreas.
- Bearer token auth (can be done now by adding header).
- API key auth (can be done now by adding header).
- OAuth2 (need to scope out what's involved here).
- Add "quit" to command palette and footer ✅
- Duplicate request from the tree. ✅
- Quickly duplicate request from the tree ✅
- Colour-coding for request types (i.e. GET is green, POST is blue, etc.) ✅
- Delete request from the tree. ✅
- Add rotating logging
- Inserting into the collection tree in sorted order, not at the bottom ✅
- <s>Improved distribution (move beyond pipx, e.g. `brew`)</s> Cancelled
- Adding test framework.
- More user friendly errors.
- Keymaps.
- I could host themes as YAML files online and offer a `posting themes install foo` which would download and move the file to the users theme directory, which would then be loaded on startup. The app could potentially even have a builtin theme "browser" which lets people preview themes (download the YAML from GitHub into memory to preview it in your app).
- Add contributing guide.  
- External documentation. ✅
- Uploading files.
- Making it clear which HTTP headers are set automatically.  
- Enabling and disabling rows in tables.
- Highlighting variables in tables to show if they've resolved or not.  
- (Maybe) File watchers so that if the request changes on disk then the UI updates to reflect it.
- Custom themes, loaded from theme directory. ✅
- Cookie editor.
- Dynamic in-app help system ✅
- Specify certificate path via config or CLI ✅
- Import from Insomnia and Postman.
- Improving OpenAPI import feature.
- General UX polish and removing footguns: validation, unsurprising navigation, warning when switching request when there are unsaved changes.