## About this document

This is a very high level overview of what I'm planning on working on.

It's unrefined and unordered at the moment.

I would like to work with the community to refine and prioritize this list.

## The Roadmap

- Variable completion autocompletion TextAreas.
- Variable resolution highlighting in TextAreas.
- Add "quit" to command palette and footer ✅
- Duplicate request from the tree. ✅
- Quickly duplicate request from the tree ✅
- Colour-coding for request types (i.e. GET is green, POST is blue, etc.)
- Delete request from the tree.  
- Add rotating logging
- Inserting into the collection tree at the position beside the cursor, not at the bottom ✅
- Improved distribution (move beyond pipx, e.g. `brew`)  
- Adding test framework.  
- More user friendly errors
- Keymaps.
- I could host themes as YAML files online and offer a `posting themes install foo` which would download and move the file to the users theme directory, which would then be loaded on startup. The app could potentially even have a builtin theme "browser" which lets people preview themes (download the YAML from GitHub into memory to preview it in your app).
- Add contributing guide.  
- External documentation. ✅
- Uploading files.  
- Making it clear which HTTP headers are set automatically.  
- Enabling and disabling rows in tables.
- Highlighting variables in tables to show if they've resolved or not.  
- (Maybe) File watchers so that if the request changes on disk then the UI updates to reflect it.
- Pre-request and post-response scripts and assertions.  
- Custom themes, loaded from theme directory. ✅
- Cookie editor.  
- Dynamic in-app help system ✅
- Specify certificate path via config or CLI ✅
- Import from Insomnia and Postman.
- Improving OpenAPI import feature.  
- General UX polish and removing footguns: validation, unsurprising navigation, warning when switching request when there are unsaved changes.