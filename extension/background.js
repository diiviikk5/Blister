import { send, settings } from "./bridge.js";

// --------------------------------------------------------------- menus

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({ id: "link", title: "Download with Blister", contexts: ["link"] });
  chrome.contextMenus.create({ id: "media", title: "Download with Blister", contexts: ["video", "audio", "image"] });
  chrome.contextMenus.create({ id: "page", title: "Download video on this page with Blister", contexts: ["page"] });
  chrome.contextMenus.create({ id: "selection", title: "Download links in selection with Blister", contexts: ["selection"] });
});

const linkRe = /(magnet:\?[^\s"'<>]+|https?:\/\/[^\s"'<>]+)/gi;

chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  const referer = tab?.url ?? "";
  let ok = false;
  switch (info.menuItemId) {
    case "link":
      ok = await send({ urls: [info.linkUrl], referer });
      break;
    case "media":
      // Streamed players use blob: sources; let yt-dlp read the page instead.
      ok = /^blob:/.test(info.srcUrl)
        ? await send({ urls: [info.pageUrl], referer, media: true })
        : await send({ urls: [info.srcUrl], referer });
      break;
    case "page":
      ok = await send({ urls: [info.pageUrl], referer, media: true });
      break;
    case "selection": {
      const urls = [...new Set((info.selectionText ?? "").match(linkRe) ?? [])];
      ok = urls.length > 0 && (await send({ urls, referer }));
      break;
    }
  }
  if (!ok) notify("Blister isn't reachable", "Open Blister and paste your pairing code into the extension.");
});

// --------------------------------------------------------------- capture

// Downloads we let through on purpose (fallback) so we don't loop.
const passthrough = new Set();

chrome.downloads.onCreated.addListener(async (item) => {
  if (passthrough.has(item.url)) {
    passthrough.delete(item.url);
    return;
  }
  const s = await settings();
  if (!s.intercept || !s.token) return;
  if (!/^(https?|ftp):/i.test(item.finalUrl || item.url)) return; // blob:, data: stay in the browser
  if (item.state !== "in_progress") return;

  const url = item.finalUrl || item.url;
  const host = safeHost(url);
  if (s.skipHosts.split(/[\s,]+/).filter(Boolean).some((h) => host === h || host.endsWith("." + h))) return;

  const name = (item.filename || url.split("?")[0].split("/").pop() || "").toLowerCase();
  const ext = name.includes(".") ? name.split(".").pop() : "";
  const always = s.alwaysExts.split(/\s+/).includes(ext);
  const size = item.totalBytes > 0 ? item.totalBytes : item.fileSize;
  if (!always && size > 0 && size < s.minSize) return;

  // Stop the browser first so the server sees one client, then hand over.
  await chrome.downloads.cancel(item.id).catch(() => {});
  const ok = await send({ urls: [url], filename: baseName(item.filename), referer: item.referrer, size: size > 0 ? size : 0 });
  if (ok) {
    chrome.downloads.erase({ id: item.id }).catch(() => {});
    return;
  }
  // Blister is closed: give the download back to the browser.
  passthrough.add(url);
  chrome.downloads.erase({ id: item.id }).catch(() => {});
  chrome.downloads.download({ url, filename: baseName(item.filename) || undefined }).catch(() => {});
});

function baseName(p) {
  return (p || "").split(/[\\/]/).pop();
}

function safeHost(u) {
  try {
    return new URL(u).hostname;
  } catch {
    return "";
  }
}

function notify(title, message) {
  chrome.notifications?.create({ type: "basic", iconUrl: "icons/icon128.png", title, message });
}
