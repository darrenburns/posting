## About this document

If you have any feedback or suggestions, please open a [new discussion on GitHub](https://github.com/darrenburns/posting/discussions/). This roadmap is driven by community requests, so please open a discussion if you'd like to see something added.

<style>
.tag {
  --tag: var(--muted);
  display: inline-block;
  padding: 0.2em 0.6em;
  border-radius: 0.3em;
  font-family: var(--font-grid);
  font-weight: 600;
  font-size: 0.75em;
  margin-left: 8px;
  color: var(--tag);
  background: color-mix(in oklab, var(--tag) 12%, transparent);
  border: 1px solid color-mix(in oklab, var(--tag) 28%, transparent);
}
.ui { --tag: var(--info-text); }
.collection { --tag: var(--primary-text); }
.environment { --tag: var(--success-text); }
.variables { --tag: var(--warning-text); }
.auth { --tag: var(--accent-text); }
.import { --tag: var(--success-text); }
.scripting { --tag: var(--secondary-text); }
.documentation { --tag: var(--warning-text); }
.ux { --tag: var(--error-text); }
.requests { --tag: var(--muted); }
.realtime { --tag: var(--info-text); }
.testing { --tag: var(--success-text); }
.cookies { --tag: var(--warning-text); }
.security { --tag: var(--error-text); }
.logging { --tag: var(--secondary-text); }
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

## Next planned features 🚀

Features planned to be worked on next.

- Documentation on using 3rd party libraries in scripts <span class="tag documentation">Documentation</span>
- Transparent background support (experimentation) <span class="tag ui">UI</span>
- In-app information about headers <span class="tag documentation">Documentation</span>
- A better footer <span class="tag ux">UX</span> <span class="tag ui">UI</span>
  - The footer currently contains too many bindings. There should be a way to show that it is scrollable, possibly showing grouping of keybindings.

## Longer Term 🔮

Features that are planned for future development but are not immediate priorities.

- Directional navigation <span class="tag ui">UI</span> <span class="tag ux">UX</span>
- Jump mode 2-stage jump - if you press shift+[jump target key], then it'll jump to the target and then show a secondary overlay of available targets within that section <span class="tag ux">UX</span>
- Manually resize sections (sidebar, request, response) <span class="tag ui">UI</span>
- Searching in responses (this will likely be simpler with upcoming Textual changes) <span class="tag requests">Requests</span>
- File watcher so that if the request changes on disk then the UI updates to reflect it <span class="tag requests">Requests</span>
- Translating to other languages <span class="tag documentation">Documentation</span>
    - I'd like to support e.g. Chinese, but need to investigate how that would render with double width characters in the terminal.
- Warning when switching request when there are unsaved changes <span class="tag ux">UX</span>
- Request tagging: the ability to add tags to requests, and filter by tag <span class="tag requests">Requests</span>
- Making it clear which HTTP headers are set automatically <span class="tag ux">UX</span>
- Collection switcher <span class="tag collection">Collection</span>
- Environment switcher <span class="tag environment">Environment</span>
- Viewing the currently loaded environment keys/values in a popup <span class="tag environment">Environment</span>
- Changing the environment at runtime via command palette <span class="tag environment">Environment</span>
- WebSocket and SSE support <span class="tag realtime">Realtime</span>
- Quickly open MDN links for headers <span class="tag ui">UI</span>
- Add rotating logging <span class="tag logging">Logging</span>
- Variable completion autocompletion in TextAreas <span class="tag environment">Environment</span>
- Variable resolution highlighting in TextAreas <span class="tag environment">Environment</span>
- Status bar? Showing the currently selected env, collection, current path, whether there's unsaved changes, etc. <span class="tag ui">UI</span>
- Highlighting variables in *tables* to show if they've resolved or not <span class="tag environment">Environment</span>
- Create a `_template.posting.yaml` file for request templates <span class="tag requests">Requests</span>
- OAuth2 implementation (need to scope out what's involved) <span class="tag auth">Auth</span>
- Adding test framework <span class="tag testing">Testing</span>
- Uploading files <span class="tag requests">Requests</span>
- Cookie editor <span class="tag requests">Requests</span>
- Import from Postman (PR is open, needs further work) <span class="tag import">Import</span>

## Completed ✓

Features that have been implemented and are available in the latest version.

- Path parameters <span class="tag requests">Requests</span>
- Adjustable padding in UI via config file <span class="tag ui">UI</span>
- Don't require user to type `http://` or `https://` in URL field <span class="tag ux">UX</span>
- Documentation on changing the UI at runtime (e.g. showing/hiding sections, etc.) <span class="tag documentation">Documentation</span>
- Editing key/value editor rows without having to delete/re-add them <span class="tag ux">UX</span>
- Keymaps <span class="tag ui">UI</span>
- Pre-request and post-response scripts <span class="tag scripting">Scripting</span>
- Parse cURL commands <span class="tag import">Import</span>
- Watching environment files for changes & updating the UI <span class="tag environment">Environment</span>
- Bearer token auth <span class="tag auth">Auth</span>
- Add "quit" to command palette and footer <span class="tag ux">UX</span>
- More user friendly errors <span class="tag ux">UX</span>
- Duplicate request from the tree <span class="tag collection">Collection</span>
- Quickly duplicate request from the tree <span class="tag collection">Collection</span>
- Colour-coding for request types (i.e. GET is green, POST is blue, etc.) <span class="tag ui">UI</span>
- Delete request from the tree <span class="tag collection">Collection</span>
- Inserting into the collection tree in sorted order, not at the bottom <span class="tag collection">Collection</span>
- External documentation <span class="tag documentation">Documentation</span>
- Enabling and disabling rows in tables <span class="tag ux">UX</span>
- Custom themes, loaded from theme directory <span class="tag ui">UI</span>
- Dynamic in-app help system <span class="tag documentation">Documentation</span>
- Specify certificate path via config or CLI <span class="tag security">Security</span>


## Legend

The following tags are used to categorize features:

<div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 30px; margin-top: 15px;">
  <div class="legend-item"><div>User Interface improvements</div> <span class="tag ui">UI</span></div>
  <div class="legend-item"><div>Collection management</div> <span class="tag collection">Collection</span></div>
  <div class="legend-item"><div>Environment handling</div> <span class="tag environment">Environment</span></div>
  <div class="legend-item"><div>Authentication methods</div> <span class="tag auth">Auth</span></div>
  <div class="legend-item"><div>Import capabilities</div> <span class="tag import">Import</span></div>
  <div class="legend-item"><div>Scripting capabilities</div> <span class="tag scripting">Scripting</span></div>
  <div class="legend-item"><div>Documentation</div> <span class="tag documentation">Documentation</span></div>
  <div class="legend-item"><div>User Experience</div> <span class="tag ux">UX</span></div>
  <div class="legend-item"><div>Requests</div> <span class="tag requests">Requests</span></div>
  <div class="legend-item"><div>Testing capabilities</div> <span class="tag testing">Testing</span></div>
  <div class="legend-item"><div>Security features</div> <span class="tag security">Security</span></div>
  <div class="legend-item"><div>Logging capabilities</div> <span class="tag logging">Logging</span></div>
</div>