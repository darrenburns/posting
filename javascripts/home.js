// Posting 3 homepage behaviour: the page is driven like the app.
//   1-6      go to a pane          ctrl+o  jump mode (label every control)
//   y        copy the install command
// The command palette (ctrl+p, :, t) is shared with the rest of the docs in
// commands.js; this adds the panes and jump mode to it. Single-key shortcuts
// can be switched off from the palette, and never fire while typing.
(function () {
  var home = document.getElementById("p3-home");
  var cmd = window.postingCommands;
  if (!home || !cmd) return;

  var root = document.documentElement;
  root.classList.add("js-home");
  var reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var panes = Array.prototype.slice.call(home.querySelectorAll(".term > .pane"));
  var INSTALL = cmd.INSTALL;

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

  // ---- Copy ----------------------------------------------------------------
  function copyText(text, button) {
    cmd.copy(text, button, button ? button.previousElementSibling : $(".prompt code"));
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
  cmd.add(function () {
    var c = panes.map(function (p, i) {
      var name = $(".pane-title", p).textContent.replace(/^\[\d\]\s*/, "");
      var heading = p.querySelector("h1, h2");
      return { g: "go to", l: name + " — " + (heading ? heading.textContent.trim() : ""), k: String(i + 1), run: function () { go(p.id); } };
    });
    c.push({ g: "action", l: "Jump mode: label every link and button", k: "^O", run: startJump });
    return c;
  });
  cmd.on("open", function () {
    if (mode === "jump") endJump();
    setMode("command");
  });
  cmd.on("close", function () { setMode("normal"); });

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
    cmd.close();
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
    root.setAttribute("data-keys-busy", "");
    if (!targets.length) endJump();
  }

  function endJump() {
    layer.textContent = "";
    targets = [];
    root.removeAttribute("data-keys-busy");
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
    if (cmd.isOpen()) return;
    if (mode === "jump") { jumpKey(e); return; }
    if (e.metaKey || e.altKey || typing(e.target)) return;
    var key = e.key;
    if (e.ctrlKey) {
      if (key === "o" || key === "O") startJump();
      else return;
    } else if (!cmd.singleKeys()) {
      return;
    } else if (/^[1-9]$/.test(key) && panes[+key - 1]) {
      go(panes[+key - 1].id);
    } else if (key === "y") {
      copyText(INSTALL);
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
      else if (action === "palette") cmd.open();
      else if (action === "copy") copyText(INSTALL);
      else if (action === "theme") cmd.open("theme ");
    });
  });
  $$("[data-copy]").forEach(function (b) {
    b.addEventListener("click", function () { copyText(b.getAttribute("data-copy"), b); });
  });
})();
