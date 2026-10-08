// Talks to Blister's local bridge (127.0.0.1 only). Shared by the service
// worker and the popup.

export const defaults = {
  token: "",
  port: 7323,
  intercept: true,
  // Smaller files stay in the browser; they finish before Blister would help.
  minSize: 1024 * 1024,
  // Always hand these over, whatever their size.
  alwaysExts: "zip rar 7z iso exe msi dmg tar gz xz bz2 mp4 mkv avi mov mp3 flac torrent apk pdf",
  skipHosts: "",
};

export async function settings() {
  const s = await chrome.storage.local.get(defaults);
  return { ...defaults, ...s };
}

function base(s) {
  return `http://127.0.0.1:${s.port}`;
}

/** { up, authorized, version } — up is false when Blister isn't running. */
export async function ping(s) {
  s ??= await settings();
  try {
    const r = await fetch(`${base(s)}/ping`, {
      headers: { "X-Blister-Token": s.token },
      signal: AbortSignal.timeout(1200),
    });
    const j = await r.json();
    return { up: j.app === "blister", authorized: !!j.authorized, version: j.version };
  } catch {
    return { up: false, authorized: false };
  }
}

async function cookiesFor(url) {
  try {
    const list = await chrome.cookies.getAll({ url });
    return list.map((c) => `${c.name}=${c.value}`).join("; ");
  } catch {
    return "";
  }
}

/**
 * Sends links to Blister. Returns true if Blister took them, so the caller
 * can fall back to the browser otherwise.
 */
export async function send({ urls, filename = "", referer = "", media = false, size = 0 }) {
  const s = await settings();
  if (!s.token) return false;
  const first = urls[0];
  const body = {
    url: first,
    urls: urls.slice(1),
    filename,
    referer,
    media,
    size,
    userAgent: navigator.userAgent,
    cookies: /^https?:/i.test(first) ? await cookiesFor(first) : "",
  };
  try {
    const r = await fetch(`${base(s)}/add`, {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Blister-Token": s.token },
      body: JSON.stringify(body),
      signal: AbortSignal.timeout(3000),
    });
    return r.ok;
  } catch {
    return false;
  }
}
