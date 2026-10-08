const root = document.documentElement;
const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];
const reduced = matchMedia("(prefers-reduced-motion: reduce)").matches;
const store = (k, v) => { try { localStorage.setItem(k, v); } catch {} };

function syncThemeColor() {
  const meta = $('meta[name="theme-color"]');
  if (meta) meta.content = getComputedStyle(document.body).getPropertyValue("--ib-paper").trim();
}

// ---------------------------------------------------------------- light / dark
const modeBtn = $("#mode");
function syncMode() {
  const dark = root.dataset.theme === "dark";
  modeBtn.setAttribute("aria-pressed", String(dark));
  modeBtn.setAttribute("aria-label", dark ? "Switch to light mode" : "Switch to dark mode");
  syncThemeColor();
}
modeBtn.addEventListener("click", () => {
  const dark = root.dataset.theme !== "dark";
  if (dark) root.dataset.theme = "dark";
  else delete root.dataset.theme;
  store("blister-mode", dark ? "dark" : "light");
  syncMode();
});
syncMode();
