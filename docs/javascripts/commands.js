// Command palette for every page of the docs, driven like Posting's own.
//   ctrl+p  :  open the palette        t  open it on the themes
// Moving over a theme previews it and esc puts the old one back. Pages add
// their own commands through window.postingCommands (the homepage adds its
// panes and jump mode). Single-key shortcuts never fire while typing, and
// can be switched off from the palette.
(function () {
  var root = document.documentElement;
  var KEYS = "posting.home.keys";
  var INSTALL = "go install github.com/darrenburns/posting/v3/cmd/posting@latest";
  var singleKeys = true;
  try { singleKeys = localStorage.getItem(KEYS) !== "off"; } catch (e) {}
  var extras = [];
  var listeners = { open: [], close: [] };

  function $$(sel, ctx) { return Array.prototype.slice.call((ctx || document).querySelectorAll(sel)); }
  function emit(name) { listeners[name].forEach(function (fn) { fn(); }); }

  // ---- Toasts --------------------------------------------------------------
  var toasts = document.createElement("div");
  toasts.className = "toasts";
  toasts.setAttribute("role", "status");
  toasts.setAttribute("aria-live", "polite");
  document.body.appendChild(toasts);

  function toast(title, body) {
    var t = document.createElement("div");
    t.className = "toast";
    var b = document.createElement("b");
    b.textContent = title;
    t.appendChild(b);
    if (body) t.appendChild(document.createTextNode(body));
    toasts.appendChild(t);
    setTimeout(function () { t.remove(); }, 2800);
  }

  // ---- Copy ----------------------------------------------------------------
  // `select` is an element to highlight when the clipboard isn't available.
  function copy(text, button, select) {
    var done = function () {
      toast("Copied to clipboard", text);
      if (!button) return;
      button.setAttribute("data-copied", "");
      setTimeout(function () { button.removeAttribute("data-copied"); }, 1600);
    };
    var fallback = function () {
      if (!select) { toast("Couldn't copy", text); return; }
      var range = document.createRange();
      range.selectNodeContents(select);
      var sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
      toast("Selected", "Press copy in your browser to take it.");
    };
    if (navigator.clipboard && window.isSecureContext) navigator.clipboard.writeText(text).then(done, fallback);
    else fallback();
  }

  // ---- Themes --------------------------------------------------------------
  // The pickers themselves are wired by palette.js; the palette drives them.
  var picker = document.querySelector(".palette-picker");
  var themes = picker ? $$("[role=radio]", picker).map(function (b) { return b.getAttribute("data-palette"); }) : [];
  function currentTheme() { return root.getAttribute("data-palette") || themes[0]; }
  function setTheme(name) {
    var radio = picker && picker.querySelector('[data-palette="' + name + '"]');
    if (radio) radio.click();
  }
  function syncTheme() {
    $$("[data-theme-name]").forEach(function (el) { el.textContent = currentTheme(); });
  }
  new MutationObserver(syncTheme).observe(root, { attributes: true, attributeFilter: ["data-palette"] });
  syncTheme();

  // ---- The palette ---------------------------------------------------------
  var dialog = document.createElement("dialog");
  dialog.className = "cmdp";
  dialog.id = "cmdp";
  dialog.setAttribute("aria-label", "Command palette");
  dialog.innerHTML =
    '<div class="cmdp-box">' +
      '<div class="cmdp-title" aria-hidden="true">commands</div>' +
      '<label class="cmdp-input"><span aria-hidden="true">&#10095;</span>' +
        '<input type="text" role="combobox" aria-label="Search commands" aria-controls="cmdp-list" aria-expanded="true" aria-autocomplete="list" autocomplete="off" spellcheck="false" placeholder="Search for commands…">' +
      '</label>' +
      '<ul class="cmdp-list" id="cmdp-list" role="listbox" aria-label="Commands"></ul>' +
      '<p class="cmdp-foot" aria-hidden="true"><b>&uarr;&darr;</b> move <b>enter</b> run <b>esc</b> close</p>' +
    '</div>';
  document.body.appendChild(dialog);
  var input = dialog.querySelector("input");
  var list = dialog.querySelector(".cmdp-list");
  var shown = [];
  var cursor = 0;
  var openedWith = null; // the theme to restore if a preview isn't chosen

  function visit(a) {
    if (a.target === "_blank") window.open(a.href, "_blank", "noopener");
    else location.href = a.href;
  }

  function label(el) {
    var clone = el.cloneNode(true);
    $$(".headerlink", clone).forEach(function (h) { h.remove(); });
    return clone.textContent.replace(/\s+/g, " ").trim();
  }

  function commands() {
    var c = [];
    var seen = {};
    function link(group, a, text) {
      var href = a.href.split("#")[0];
      if (!text || seen[href]) return;
      seen[href] = true;
      c.push({ g: group, l: text, k: a.target === "_blank" ? "↗" : "", run: function () { visit(a); } });
    }

    extras.forEach(function (fn) { c = c.concat(fn()); });
    $$(".md-content .md-typeset :is(h2, h3)[id]").forEach(function (h) {
      c.push({ g: "section", l: label(h), k: "", run: function () {
        h.scrollIntoView({ block: "start" });
        try { history.replaceState(null, "", "#" + h.id); } catch (e) {}
      } });
    });
    seen[location.href.split("#")[0]] = true;
    $$(".md-tabs__link, .md-nav--primary a.md-nav__link[href]").forEach(function (a) {
      if (a.getAttribute("href").charAt(0) !== "#") link("page", a, label(a));
    });
    c.push({ g: "action", l: "Search the docs", k: "/", run: openSearch });
    c.push({ g: "action", l: "Copy the install command", k: "", run: function () { copy(INSTALL); } });
    themes.forEach(function (name) {
      c.push({ g: "theme", l: name + (name === openedWith ? "  (current)" : ""), theme: name, k: "", run: function () { openedWith = name; setTheme(name); } });
    });
    $$(".md-source[href]").forEach(function (a) { link("open", a, "GitHub"); });
    c.push({ g: "setting", l: (singleKeys ? "Turn off" : "Turn on") + " single-key shortcuts", k: "", run: toggleKeys });
    return c;
  }

  function openSearch() {
    var toggle = document.getElementById("__search");
    var field = document.querySelector(".md-search__input");
    if (!toggle || !field) return;
    toggle.checked = true;
    toggle.dispatchEvent(new Event("change"));
    field.focus();
  }

  function matches(query, text) {
    text = text.toLowerCase();
    return query.toLowerCase().split(/\s+/).filter(Boolean).every(function (word) {
      if (text.indexOf(word) >= 0) return true;
      var at = 0;
      for (var i = 0; i < word.length; i++) {
        at = text.indexOf(word[i], at) + 1;
        if (!at) return false;
      }
      return true;
    });
  }

  function render() {
    var q = input.value;
    shown = commands().filter(function (c) { return matches(q, c.g + " " + c.l); });
    cursor = Math.min(cursor, Math.max(0, shown.length - 1));
    list.textContent = "";
    if (!shown.length) {
      var none = document.createElement("li");
      none.className = "empty";
      none.textContent = "No matching commands";
      list.appendChild(none);
      input.removeAttribute("aria-activedescendant");
      return;
    }
    shown.forEach(function (c, i) {
      var li = document.createElement("li");
      li.id = "cmdp-" + i;
      li.setAttribute("role", "option");
      li.setAttribute("aria-selected", i === cursor ? "true" : "false");
      li.innerHTML = '<span class="g"></span><span class="l"></span><kbd></kbd>';
      li.children[0].textContent = c.g;
      li.children[1].textContent = c.l;
      li.children[2].textContent = c.k;
      li.addEventListener("mousemove", function () { if (cursor !== i) { cursor = i; mark(); } });
      li.addEventListener("click", function () { run(i); });
      list.appendChild(li);
    });
    mark();
  }

  function mark() {
    $$("[role=option]", list).forEach(function (li, i) {
      li.setAttribute("aria-selected", i === cursor ? "true" : "false");
    });
    // Like Posting's theme picker, moving over a theme previews it.
    var c = shown[cursor];
    preview(c && c.theme ? c.theme : openedWith);
    var active = document.getElementById("cmdp-" + cursor);
    if (active) {
      input.setAttribute("aria-activedescendant", active.id);
      active.scrollIntoView({ block: "nearest" });
    }
  }

  function preview(name) {
    if (!name || name === currentTheme()) return;
    root.setAttribute("data-palette", name);
  }

  function run(i) {
    var c = shown[i];
    dialog.close();
    if (c) c.run();
  }

  function open(query) {
    if (!dialog.showModal) return;
    input.value = query || "";
    openedWith = currentTheme();
    cursor = 0;
    render();
    if (query === "theme ") {
      var at = shown.findIndex(function (c) { return c.theme === openedWith; });
      if (at >= 0) { cursor = at; mark(); }
    }
    dialog.showModal();
    input.focus();
    emit("open");
  }

  input.addEventListener("input", function () { cursor = 0; render(); });
  input.addEventListener("keydown", function (e) {
    var step = { ArrowDown: 1, ArrowUp: -1 }[e.key];
    if (e.ctrlKey && (e.key === "n" || e.key === "j")) step = 1;
    if (e.ctrlKey && (e.key === "p" || e.key === "k")) step = -1;
    if (step && shown.length) {
      e.preventDefault();
      cursor = (cursor + step + shown.length) % shown.length;
      mark();
    } else if (e.key === "Enter") {
      e.preventDefault();
      run(cursor);
    }
  });
  dialog.addEventListener("close", function () {
    preview(openedWith);
    emit("close");
  });
  dialog.addEventListener("click", function (e) { if (e.target === dialog) dialog.close(); });

  function toggleKeys() {
    singleKeys = !singleKeys;
    try { localStorage.setItem(KEYS, singleKeys ? "on" : "off"); } catch (e) {}
    toast("Single-key shortcuts " + (singleKeys ? "on" : "off"), singleKeys ? "" : "ctrl+p still opens the palette.");
  }

  // ---- Keys ----------------------------------------------------------------
  function typing(el) {
    return el && el.closest && el.closest("input, textarea, select, [contenteditable=''], [contenteditable=true]");
  }

  document.addEventListener("keydown", function (e) {
    // data-keys-busy: a page mode (the homepage's jump mode) owns the keys.
    if (dialog.open || root.hasAttribute("data-keys-busy")) return;
    if (e.metaKey || e.altKey || typing(e.target)) return;
    if (e.ctrlKey) {
      if (e.key !== "p" && e.key !== "P") return;
      open();
    } else if (singleKeys && e.key === ":") {
      open();
    } else if (singleKeys && e.key === "t") {
      open("theme ");
    } else {
      return;
    }
    e.preventDefault();
    e.stopImmediatePropagation();
  }, true);

  $$("[data-cmdp]").forEach(function (b) {
    b.addEventListener("click", function () { open(b.getAttribute("data-cmdp")); });
  });

  window.postingCommands = {
    INSTALL: INSTALL,
    open: open,
    close: function () { if (dialog.open) dialog.close(); },
    isOpen: function () { return dialog.open; },
    add: function (fn) { extras.push(fn); },
    on: function (name, fn) { listeners[name].push(fn); },
    singleKeys: function () { return singleKeys; },
    toast: toast,
    copy: copy
  };
})();
