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

## Not yet in Posting 3 🚧

Posting 3 is a rewrite of Posting in Go. These Posting 2 features haven't been brought across yet.

- Pre-request and post-response scripts (requests keep their scripts, but they aren't run) <span class="tag scripting">Scripting</span>
- Undo and redo in text areas <span class="tag ux">UX</span>
- Live reloading of theme files, and X resources themes <span class="tag ui">UI</span>
- Custom syntax highlighting and method colours in theme files <span class="tag ui">UI</span>

## Longer Term 🔮

Features that are planned for future development but are not immediate priorities.

- Searching in responses <span class="tag requests">Requests</span>
- Multipart form bodies and file uploads <span class="tag requests">Requests</span>
- Transparent background support (experimentation) <span class="tag ui">UI</span>
- In-app information about headers, and quickly opening MDN links for them <span class="tag documentation">Documentation</span>
- Jump mode 2-stage jump - if you press shift+[jump target key], then it'll jump to the target and then show a secondary overlay of available targets within that section <span class="tag ux">UX</span>
- Translating to other languages <span class="tag documentation">Documentation</span>
    - I'd like to support e.g. Chinese, but need to investigate how that would render with double width characters in the terminal.
- Warning when closing a tab that has unsaved changes <span class="tag ux">UX</span>
- Request tagging: the ability to add tags to requests, and filter by tag <span class="tag requests">Requests</span>
- Making it clear which HTTP headers are set automatically <span class="tag ux">UX</span>
- Collection switcher <span class="tag collection">Collection</span>
- WebSocket and SSE support <span class="tag realtime">Realtime</span>
- Add rotating logging <span class="tag logging">Logging</span>
- Create a `_template.posting.yaml` file for request templates <span class="tag requests">Requests</span>
- OAuth2 implementation (need to scope out what's involved) <span class="tag auth">Auth</span>
- Adding test framework <span class="tag testing">Testing</span>
- Cookie editor <span class="tag requests">Requests</span>

## Completed ✓

Features that have been implemented in Posting 3.

- Request tabs, with a preview tab for browsing the collection <span class="tag ui">UI</span>
- Layered environments: `posting.env`, `<name>.env` and `.local.env` files <span class="tag environment">Environment</span>
- Environment switcher, remembered per collection <span class="tag environment">Environment</span>
- Viewing and overriding the loaded variables in a popup <span class="tag environment">Environment</span>
- Variable autocompletion and highlighting in inputs and text areas <span class="tag environment">Environment</span>
- Showing the active environment in the header <span class="tag ui">UI</span>
- Request timing (trace) in the Response panel and history <span class="tag requests">Requests</span>
- Request and response history, kept per collection <span class="tag requests">Requests</span>
- Filtering the collection by name, folder and method <span class="tag collection">Collection</span>
- Manually resize sections (sidebar, request, response) <span class="tag ui">UI</span>
- Reloading requests when they change on disk <span class="tag requests">Requests</span>
- A footer that shows the keys for whatever has focus <span class="tag ux">UX</span>
- Import from OpenAPI, Postman and Bruno <span class="tag import">Import</span>
- Export as curl, with or without variables filled in <span class="tag import">Import</span>
- Path parameters <span class="tag requests">Requests</span>
- Adjustable spacing in the UI via config file <span class="tag ui">UI</span>
- Don't require user to type `http://` or `https://` in URL field <span class="tag ux">UX</span>
- Editing key/value rows inline <span class="tag ux">UX</span>
- Keymaps <span class="tag ui">UI</span>
- Parse cURL commands <span class="tag import">Import</span>
- Watching environment files for changes & updating the UI <span class="tag environment">Environment</span>
- Basic, Digest and Bearer token auth <span class="tag auth">Auth</span>
- Duplicate and delete requests from the tree <span class="tag collection">Collection</span>
- Colour-coding for request methods <span class="tag ui">UI</span>
- Enabling and disabling rows in tables <span class="tag ux">UX</span>
- Custom themes, loaded from the theme directory <span class="tag ui">UI</span>
- Specify certificates via config <span class="tag security">Security</span>
- Encrypted client certificate keys (`ssl.password`) <span class="tag security">Security</span>

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