## About this document

If you have any feedback or suggestions, please open a [new discussion on GitHub](https://github.com/darrenburns/posting/discussions/). This roadmap is driven by community requests, so please open a discussion if you'd like to see something added.

<style>
.tag {
  display: inline-block;
  padding: 3px 8px;
  border-radius: 4px;
  font-weight: bold;
  font-size: 0.8em;
  margin-left: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  background: linear-gradient(135deg, rgba(30, 15, 45, 0.8), rgba(50, 30, 70, 0.9));
  border: 1px solid rgba(100, 70, 150, 0.4);
}
.ui { color: #88ccff; }
.collection { color: #a0b8ff; }
.environment { color: #80ffee; }
.variables { color: #eeff80; }
.auth { color: #ff80bf; }
.import { color: #a0ff80; }
.scripting { color: #d580ff; }
.documentation { color: #ffcc80; }
.ux { color: #ff9980; }
.file { color: #cccccc; }
.realtime { color: #80c8ff; }
.templates { color: #c080ff; }
.testing { color: #80ffb0; }
.cookies { color: #ffaa80; }
.security { color: #ff8080; }
.logging { color: #8080ff; }
.help { color: #80ff80; }
.legend-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 0;
  white-space: nowrap;
}
.legend-item div {
  margin-right: 20px;
}
.legend-item span.tag {
  flex-shrink: 0;
}
</style>

## Ongoing 🔄

Features that are currently being worked on.

- Add contributing guide <span class="tag documentation">Documentation</span>

## Upcoming 🚀

Features planned for the near future.

- Editing key/value editor rows without having to delete/re-add them <span class="tag ux">UX</span>
- Adjustable padding in UI via config file <span class="tag ui">UI</span>
- Don't require user to type `http://` or `https://` in URL field <span class="tag ux">UX</span>

## Longer Term 🔮

Features that are planned for future development but are not immediate priorities.

- File watcher so that if the request changes on disk then the UI updates to reflect it <span class="tag file">File</span>
- Translating to other languages <span class="tag documentation">Documentation</span>
    - I'd like to support e.g. Chinese, but need to investigate how that would render with double width characters in the terminal.
- Warning when switching request when there are unsaved changes <span class="tag ux">UX</span>
- Making it clear which HTTP headers are set automatically <span class="tag ux">UX</span>
- Collection switcher <span class="tag collection">Collection</span>
- Environment switcher <span class="tag environment">Environment</span>
- Viewing the currently loaded environment keys/values in a popup <span class="tag environment">Environment</span>
- Changing the environment at runtime via command palette <span class="tag environment">Environment</span>
- WebSocket and SSE support <span class="tag realtime">Realtime</span>
- Quickly open MDN links for headers <span class="tag ui">UI</span>
- Add rotating logging <span class="tag logging">Logging</span>
- Variable completion autocompletion in TextAreas <span class="tag variables">Variables</span>
- Variable resolution highlighting in TextAreas <span class="tag variables">Variables</span>
- Highlighting variables in *tables* to show if they've resolved or not <span class="tag variables">Variables</span>
- Create a `_template.posting.yaml` file for request templates <span class="tag templates">Templates</span>
- OAuth2 implementation (need to scope out what's involved) <span class="tag auth">Auth</span>
- Adding test framework <span class="tag testing">Testing</span>
- Uploading files <span class="tag file">Files</span>
- Cookie editor <span class="tag cookies">Cookies</span>
- Import from Insomnia and Postman (Postman import PR is open, needs further work) <span class="tag import">Import</span>
- Improving OpenAPI import feature <span class="tag import">Import</span>

## Completed ✓

Features that have been implemented and are available in the current version.

- Keymaps <span class="tag ui">UI</span>
- Pre-request and post-response scripts <span class="tag scripting">Scripting</span>
- Parse cURL commands <span class="tag import">Import</span>
- Watching environment files for changes & updating the UI <span class="tag environment">Environment</span>
- Bearer token auth <span class="tag auth">Auth</span>
- Add "quit" to command palette and footer <span class="tag ui">UI</span>
- More user friendly errors <span class="tag ux">UX</span>
- Duplicate request from the tree <span class="tag collection">Collection</span>
- Quickly duplicate request from the tree <span class="tag collection">Collection</span>
- Colour-coding for request types (i.e. GET is green, POST is blue, etc.) <span class="tag ui">UI</span>
- Delete request from the tree <span class="tag collection">Collection</span>
- Inserting into the collection tree in sorted order, not at the bottom <span class="tag collection">Collection</span>
- External documentation <span class="tag documentation">Documentation</span>
- Enabling and disabling rows in tables <span class="tag ui">UI</span>
- Custom themes, loaded from theme directory <span class="tag ui">UI</span>
- Dynamic in-app help system <span class="tag help">Help</span>
- Specify certificate path via config or CLI <span class="tag security">Security</span>


## Legend

The following tags are used to categorize features:

<div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 30px; margin-top: 15px;">
  <div class="legend-item"><div>User Interface improvements</div> <span class="tag ui">UI</span></div>
  <div class="legend-item"><div>Collection management</div> <span class="tag collection">Collection</span></div>
  <div class="legend-item"><div>Environment handling</div> <span class="tag environment">Environment</span></div>
  <div class="legend-item"><div>Variable handling</div> <span class="tag variables">Variables</span></div>
  <div class="legend-item"><div>Authentication methods</div> <span class="tag auth">Auth</span></div>
  <div class="legend-item"><div>Import capabilities</div> <span class="tag import">Import</span></div>
  <div class="legend-item"><div>Scripting capabilities</div> <span class="tag scripting">Scripting</span></div>
  <div class="legend-item"><div>Documentation</div> <span class="tag documentation">Documentation</span></div>
  <div class="legend-item"><div>User Experience</div> <span class="tag ux">UX</span></div>
  <div class="legend-item"><div>File handling</div> <span class="tag file">File</span></div>
  <div class="legend-item"><div>Real-time communication</div> <span class="tag realtime">Realtime</span></div>
  <div class="legend-item"><div>Template features</div> <span class="tag templates">Templates</span></div>
  <div class="legend-item"><div>Testing capabilities</div> <span class="tag testing">Testing</span></div>
  <div class="legend-item"><div>Cookie management</div> <span class="tag cookies">Cookies</span></div>
  <div class="legend-item"><div>Security features</div> <span class="tag security">Security</span></div>
  <div class="legend-item"><div>Logging capabilities</div> <span class="tag logging">Logging</span></div>
  <div class="legend-item"><div>Help and assistance</div> <span class="tag help">Help</span></div>
</div>