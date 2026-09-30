// Posting 3 homepage behaviour: the page is driven like the app.
//   1-6      go to a pane          ctrl+o  jump mode (label every control)
//   ctrl+p : command palette       y       copy the install command
//   t        pick a theme          esc     leave jump mode / the palette
// Single-key shortcuts can be switched off from the command palette, and
// never fire while typing. The theme picker itself is wired by palette.js.
(function () {
  var home = document.getElementById("p3-home");
  if (!home) return;

  var root = document.documentElement;
  root.classList.add("js-home");
  var reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var panes = Array.prototype.slice.call(home.querySelectorAll(".term > .pane"));
  var KEYS = "posting.home.keys";
  var singleKeys = true;
  try { singleKeys = localStorage.getItem(KEYS) !== "off"; } catch (e) {}
  var INSTALL = "go install github.com/darrenburns/posting/cmd/posting@latest";

  function $(sel, ctx) { return (ctx || home).querySelector(sel); }
  function $$(sel, ctx) { return Array.prototype.slice.call((ctx || home).querySelectorAll(sel)); }

  // ---- Mode badge ----------------------------------------------------------
  var modeBadge = $("[data-mode]");
  var mode = "normal";
  function setMode(m) {
    mode = m;
    home.setAttribute("data-mode", m);
    modeBadge.textContent = m.toUpperCase();
  }

  // ---- Toasts --------------------------------------------------------------
  var toasts = $(".toasts");
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
  function copyText(text, button) {
    var done = function () {
      toast("Copied to clipboard", text);
      if (!button) return;
      button.setAttribute("data-copied", "");
      setTimeout(function () { button.removeAttribute("data-copied"); }, 1600);
    };
    var fallback = function () {
      var code = button ? button.previousElementSibling : $(".prompt code");
      var range = document.createRange();
      range.selectNodeContents(code);
      var sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
      toast("Selected", "Press copy in your browser to take it.");
    };
    if (navigator.clipboard && window.isSecureContext) navigator.clipboard.writeText(text).then(done, fallback);
    else fallback();
  }

  // ---- Panes ---------------------------------------------------------------
  function go(id) {
    var pane = document.getElementById(id);
    if (!pane) return;
    pane.scrollIntoView({ behavior: reduce ? "auto" : "smooth", block: "start" });
    pane.focus({ preventScroll: true });
    try { history.replaceState(null, "", "#" + id); } catch (e) {}
  }

  var paneButtons = $$(".kb-panes [data-go]");
  var posPane = $("[data-pos-pane]");
  var posPct = $("[data-pos-pct]");
  var ticking = false;
  function track() {
    ticking = false;
    var line = window.innerHeight * 0.35;
    var max = document.documentElement.scrollHeight - window.innerHeight;
    var current = 0;
    panes.forEach(function (p, i) {
      var top = p.getBoundingClientRect().top;
      if (top < line) current = i;
    });
    if (max > 0 && window.scrollY >= max - 2) current = panes.length - 1;
    panes.forEach(function (p, i) { p.toggleAttribute("data-active", i === current); });
    paneButtons.forEach(function (b, i) { b.setAttribute("aria-current", i === current ? "true" : "false"); });
    posPane.textContent = current + 1 + "/" + panes.length;
    posPct.textContent = (max > 0 ? Math.min(100, Math.round((window.scrollY / max) * 100)) : 100) + "%";
  }
  function schedule() {
    if (!ticking) { ticking = true; requestAnimationFrame(track); }
    if (mode === "jump") endJump();
  }
  window.addEventListener("scroll", schedule, { passive: true });
  window.addEventListener("resize", schedule);
  window.addEventListener("load", track);
  track();

  // Panes play their one-off animations the first time they come into view.
  if ("IntersectionObserver" in window) {
    var seen = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        entry.target.setAttribute("data-seen", "");
        seen.unobserve(entry.target);
      });
    }, { threshold: 0.3 });
    panes.forEach(function (p) { seen.observe(p); });
  } else {
    panes.forEach(function (p) { p.setAttribute("data-seen", ""); });
  }

  // ---- Themes --------------------------------------------------------------
  var picker = $(".theme-line .palette-picker");
  var themes = $$("[role=radio]", picker).map(function (b) { return b.getAttribute("data-palette"); });
  var themeName = $("[data-theme-name]");
  function currentTheme() { return root.getAttribute("data-palette") || themes[0]; }
  function syncTheme() { themeName.textContent = currentTheme(); }
  function setTheme(name) {
    var radio = picker.querySelector('[data-palette="' + name + '"]');
    if (radio) radio.click();
  }
  new MutationObserver(syncTheme).observe(root, { attributes: true, attributeFilter: ["data-palette"] });
  syncTheme();

  // ---- Feature tour --------------------------------------------------------
  var tabs = $$(".tour-tabs [role=tab]");
  var count = $("[data-tour-count]");
  function select(tab) {
    tabs.forEach(function (t, i) {
      var on = t === tab;
      t.setAttribute("aria-selected", on ? "true" : "false");
      t.tabIndex = on ? 0 : -1;
      document.getElementById(t.getAttribute("aria-controls")).hidden = !on;
      if (on) count.textContent = i + 1;
    });
  }
  tabs.forEach(function (t, i) {
    t.addEventListener("click", function () { select(t); });
    t.addEventListener("keydown", function (e) {
      var step = { ArrowDown: 1, ArrowRight: 1, j: 1, ArrowUp: -1, ArrowLeft: -1, k: -1 }[e.key];
      if (!step || e.ctrlKey || e.metaKey || e.altKey) return;
      e.preventDefault();
      e.stopPropagation();
      var next = tabs[(i + step + tabs.length) % tabs.length];
      next.focus();
      select(next);
    });
  });

  // ---- Command palette -----------------------------------------------------
  var dialog = $("#cmdp");
  var input = $("input", dialog);
  var list = $(".cmdp-list", dialog);
  var shown = [];
  var cursor = 0;
  var openedWith = null; // the theme to restore if a preview isn't chosen

  function commands() {
    var c = [];
    panes.forEach(function (p, i) {
      var name = $(".pane-title", p).textContent.replace(/^\[\d\]\s*/, "");
      var heading = p.querySelector("h1, h2");
      c.push({ g: "go to", l: name + " — " + (heading ? heading.textContent.trim() : ""), k: String(i + 1), run: function () { go(p.id); } });
    });
    c.push({ g: "action", l: "Jump mode: label every link and button", k: "^O", run: startJump });
    c.push({ g: "action", l: "Copy the install command", k: "y", run: function () { copyText(INSTALL); } });
    themes.forEach(function (name) {
      c.push({ g: "theme", l: name + (name === openedWith ? "  (current)" : ""), theme: name, k: "", run: function () { openedWith = name; setTheme(name); } });
    });
    $$(".site-foot nav a").forEach(function (a) {
      c.push({ g: "open", l: a.textContent.trim(), k: a.target === "_blank" ? "↗" : "", run: function () { a.click(); } });
    });
    c.push({ g: "setting", l: (singleKeys ? "Turn off" : "Turn on") + " single-key shortcuts (1-6, y, t, :)", k: "", run: toggleKeys });
    return c;
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

  function openPalette(query) {
    if (!dialog.showModal) return;
    if (mode === "jump") endJump();
    input.value = query || "";
    openedWith = currentTheme();
    cursor = 0;
    render();
    if (query === "theme ") {
      var at = themes.indexOf(currentTheme());
      if (at >= 0) { cursor = at; mark(); }
    }
    dialog.showModal();
    input.focus();
    setMode("command");
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
    setMode("normal");
  });
  dialog.addEventListener("click", function (e) { if (e.target === dialog) dialog.close(); });

  function toggleKeys() {
    singleKeys = !singleKeys;
    try { localStorage.setItem(KEYS, singleKeys ? "on" : "off"); } catch (e) {}
    toast("Single-key shortcuts " + (singleKeys ? "on" : "off"), singleKeys ? "" : "ctrl+o and ctrl+p still work.");
  }

  // ---- Jump mode -----------------------------------------------------------
  var layer = $(".jump-layer");
  var ALPHA = "asdfghjklqwertyuiopzxcvbnm";
  var targets = [];
  var typed = "";

  function onScreen(el) {
    if (el.closest("[hidden], .keybar, dialog, .jump-layer")) return null;
    if (el.checkVisibility && !el.checkVisibility({ visibilityProperty: true, opacityProperty: true })) return null;
    var r = el.getBoundingClientRect();
    if (r.width < 2 || r.height < 2) return null;
    if (r.bottom <= 0 || r.right <= 0 || r.top >= window.innerHeight - 8 || r.left >= window.innerWidth) return null;
    return r;
  }

  function labels(n) {
    if (n <= ALPHA.length) return ALPHA.slice(0, n).split("");
    var singles = Math.max(0, Math.min(ALPHA.length, Math.floor((ALPHA.length * ALPHA.length - n) / (ALPHA.length - 1))));
    var out = ALPHA.slice(0, singles).split("");
    for (var i = singles; i < ALPHA.length && out.length < n; i++) {
      for (var j = 0; j < ALPHA.length && out.length < n; j++) out.push(ALPHA[i] + ALPHA[j]);
    }
    return out;
  }

  function startJump() {
    if (dialog.open) dialog.close();
    var found = [];
    document.querySelectorAll("a[href], button, input:not([type=hidden]), summary").forEach(function (el) {
      var r = onScreen(el);
      if (r) found.push({ el: el, r: r });
    });
    found.sort(function (a, b) { return Math.round(a.r.top / 8) - Math.round(b.r.top / 8) || a.r.left - b.r.left; });
    var names = labels(found.length);
    layer.textContent = "";
    targets = found.map(function (t, i) {
      var span = document.createElement("span");
      span.className = "jump-label";
      span.style.left = Math.max(2, t.r.left) + "px";
      span.style.top = Math.max(10, t.r.top + t.r.height / 2) + "px";
      span.textContent = names[i];
      layer.appendChild(span);
      return { el: t.el, label: names[i], span: span };
    });
    typed = "";
    setMode("jump");
    if (!targets.length) endJump();
  }

  function endJump() {
    layer.textContent = "";
    targets = [];
    setMode("normal");
  }

  function jumpKey(e) {
    e.preventDefault();
    e.stopImmediatePropagation();
    var key = e.key.toLowerCase();
    if (e.key === "Backspace") {
      typed = typed.slice(0, -1);
    } else if (key.length === 1 && ALPHA.indexOf(key) >= 0 && !e.ctrlKey && !e.metaKey) {
      typed += key;
    } else if (!/^(Shift|Control|Alt|Meta)$/.test(e.key)) {
      endJump();
      return;
    }
    var hit = null;
    var live = 0;
    targets.forEach(function (t) {
      var on = t.label.indexOf(typed) === 0;
      t.span.hidden = !on;
      if (!on) return;
      live++;
      t.span.innerHTML = '<span class="hit"></span>';
      t.span.firstChild.textContent = typed;
      t.span.appendChild(document.createTextNode(t.label.slice(typed.length)));
      if (t.label === typed) hit = t.el;
    });
    if (hit) {
      endJump();
      hit.focus({ preventScroll: true });
      if (hit.matches("a[href], button, summary")) hit.click();
    } else if (!live) {
      endJump();
    }
  }

  // ---- Keys ----------------------------------------------------------------
  function typing(el) {
    return el && el.closest && el.closest("input, textarea, select, [contenteditable=''], [contenteditable=true]");
  }

  document.addEventListener("keydown", function (e) {
    if (dialog.open) return;
    if (mode === "jump") { jumpKey(e); return; }
    if (e.metaKey || e.altKey || typing(e.target)) return;
    var key = e.key;
    if (e.ctrlKey) {
      if (key === "o" || key === "O") startJump();
      else if (key === "p" || key === "P") openPalette();
      else return;
    } else if (!singleKeys) {
      return;
    } else if (/^[1-9]$/.test(key) && panes[+key - 1]) {
      go(panes[+key - 1].id);
    } else if (key === ":") {
      openPalette();
    } else if (key === "y") {
      copyText(INSTALL);
    } else if (key === "t") {
      openPalette("theme ");
    } else {
      return;
    }
    e.preventDefault();
    e.stopImmediatePropagation();
  }, true);

  // ---- Buttons -------------------------------------------------------------
  paneButtons.forEach(function (b) {
    b.addEventListener("click", function () { go(b.getAttribute("data-go")); });
  });
  $$("[data-action]").forEach(function (b) {
    b.addEventListener("click", function () {
      var action = b.getAttribute("data-action");
      if (action === "jump") startJump();
      else if (action === "palette") openPalette();
      else if (action === "copy") copyText(INSTALL);
      else if (action === "theme") openPalette("theme ");
    });
  });
  $$("[data-copy]").forEach(function (b) {
    b.addEventListener("click", function () { copyText(b.getAttribute("data-copy"), b); });
  });
})();
