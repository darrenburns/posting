// Theme pickers: radio groups that re-colour every page of the docs. The
// choice is saved, and restored before first paint by overrides/main.html.
(function () {
  var root = document.documentElement;
  var KEY = "posting.palette";
  var reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  var groups = Array.prototype.slice.call(document.querySelectorAll(".palette-picker"));
  if (!groups.length) return;
  var names = Array.prototype.map.call(groups[0].querySelectorAll("[role=radio]"), function (b) {
    return b.getAttribute("data-palette");
  });
  if (names.indexOf(root.getAttribute("data-palette")) < 0) root.removeAttribute("data-palette");

  function active() { return root.getAttribute("data-palette") || names[0]; }
  function mark() {
    document.querySelectorAll(".palette-picker [role=radio]").forEach(function (b) {
      var on = b.getAttribute("data-palette") === active();
      b.setAttribute("aria-checked", on ? "true" : "false");
      b.tabIndex = on ? 0 : -1;
    });
  }
  function choose(name) {
    var apply = function () { root.setAttribute("data-palette", name); mark(); };
    if (document.startViewTransition && !reduce) document.startViewTransition(apply); else apply();
    try { localStorage.setItem(KEY, name); } catch (e) {}
  }

  groups.forEach(function (group) {
    var radios = Array.prototype.slice.call(group.querySelectorAll("[role=radio]"));
    radios.forEach(function (b, i) {
      b.addEventListener("click", function () { choose(b.getAttribute("data-palette")); });
      b.addEventListener("keydown", function (e) {
        var step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
        if (!step) return;
        e.preventDefault();
        var next = radios[(i + step + radios.length) % radios.length];
        next.focus();
        choose(next.getAttribute("data-palette"));
      });
    });
  });
  mark();

  // The header menu opens under its button.
  var menu = document.getElementById("theme-menu");
  var toggle = document.querySelector("[popovertarget=theme-menu]");
  if (!menu || !toggle || !menu.showPopover) {
    if (toggle) toggle.hidden = true;
    return;
  }
  menu.addEventListener("beforetoggle", function (e) {
    if (e.newState !== "open") return;
    var box = toggle.getBoundingClientRect();
    menu.style.setProperty("--menu-top", box.bottom + 8 + "px");
    menu.style.setProperty("--menu-right", Math.max(8, window.innerWidth - box.right) + "px");
  });
  menu.addEventListener("toggle", function (e) {
    var open = e.newState === "open";
    toggle.setAttribute("aria-expanded", open ? "true" : "false");
    if (open) {
      var checked = menu.querySelector("[aria-checked=true]");
      if (checked) checked.focus();
    }
  });
})();
