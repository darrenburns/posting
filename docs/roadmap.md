## About this document

This is a very high level overview of what I'm planning on working on.

It's unrefined and unordered at the moment, but I would like to work with the community to refine and prioritize this list in the future.

If you have any feedback or suggestions, please open a new discussion on GitHub. Opening a discussion helps me understand what's important to the community, as others can upvote it if they're interested.

## The Roadmap

- Keymaps. ✅
- Pre-request and post-response scripts. ✅
- Parse cURL commands. ✅
- Watching environment files for changes & updating the UI. ✅
- Editing key/value editor rows without having to delete/re-add them.
- Realtime - WebSocket and SSE.
- Quickly open MDN links for headers.
- Templates. Create a `_template.posting.yaml` file (perhaps a checkbox in the new request modal for this). Any requests created in a collection will be based off of the nearest template (looking upwards to the collection root). Note that this is not "inheritance" - it's a means of quickly pre-filling values in requests based on a template request.
- Saving recently used environments to a file.
- Saving recently used collections to a file.
- Viewing the currently loaded environment keys/values in a popup.
- Changing the environment at runtime - probably via command palette - push a new command palette screen where you can search for and select one of the previously used environments.
- Variable completion autocompletion TextAreas.
- Variable resolution highlighting in TextAreas.
- Bearer token auth ✅
- API key auth (can be done now by adding header).
- OAuth2 (need to scope out what's involved here).
- Add "quit" to command palette and footer ✅
- Duplicate request from the tree. ✅
- Quickly duplicate request from the tree ✅
- Colour-coding for request types (i.e. GET is green, POST is blue, etc.) ✅
- Delete request from the tree. ✅
- Add rotating logging
- Inserting into the collection tree in sorted order, not at the bottom ✅
- Adding test framework.
- More user friendly errors.
- Add contributing guide.  
- External documentation. ✅
- Uploading files.
- Making it clear which HTTP headers are set automatically.  
- Enabling and disabling rows in tables. ✅
- Highlighting variables in tables to show if they've resolved or not.  
- File watcher so that if the request changes on disk then the UI updates to reflect it.
- Custom themes, loaded from theme directory. ✅
- Cookie editor.
- Dynamic in-app help system ✅
- Specify certificate path via config or CLI ✅
- Import from Insomnia and Postman (Postman import PR is open, needs further work).
- Improving OpenAPI import feature.
- General UX polish and removing footguns: validation, unsurprising navigation, warning when switching request when there are unsaved changes.