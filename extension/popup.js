import { ping, send, settings } from "./bridge.js";

const $ = (id) => document.getElementById(id);

async function render() {
  const s = await settings();
  const p = await ping(s);
  const status = $("status");
  $("down").hidden = p.up;
  if (!p.up) {
    status.textContent = "Not running";
    status.className = "pill bad";
    $("pair").hidden = true;
    $("main").hidden = true;
    return;
  }
  if (!p.authorized) {
    status.textContent = "Not paired";
    status.className = "pill warn";
    $("pair").hidden = false;
    $("main").hidden = true;
    $("token").focus();
    return;
  }
  status.textContent = `Connected${p.version ? " · " + p.version : ""}`;
  status.className = "pill ok";
  $("pair").hidden = true;
  $("main").hidden = false;
  $("intercept").checked = s.intercept;
  $("minSize").value = String(s.minSize);
}

$("save").addEventListener("click", async () => {
  await chrome.storage.local.set({ token: $("token").value.trim() });
  render();
});
$("token").addEventListener("keydown", (e) => e.key === "Enter" && $("save").click());

$("intercept").addEventListener("change", (e) => chrome.storage.local.set({ intercept: e.target.checked }));
$("minSize").addEventListener("change", (e) => chrome.storage.local.set({ minSize: Number(e.target.value) }));

$("unpair").addEventListener("click", async () => {
  await chrome.storage.local.set({ token: "" });
  render();
});

$("grab").addEventListener("click", async () => {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab?.url) return;
  const ok = await send({ urls: [tab.url], referer: tab.url, media: true });
  $("grab").textContent = ok ? "Sent to Blister" : "Couldn't reach Blister";
  setTimeout(() => window.close(), 900);
});

render();
