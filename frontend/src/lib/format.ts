const units = ["B", "KB", "MB", "GB", "TB"];

/** 1536 -> "1.5 KB". Unknown sizes (<0) read as an em-free dash. */
export function bytes(n: number, digits = 1): string {
  if (n == null || n < 0 || !isFinite(n)) return "–";
  if (n < 1024) return `${Math.round(n)} B`;
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  return `${n.toFixed(n >= 100 ? 0 : digits)} ${units[i]}`;
}

export function speed(n: number): string {
  if (!n || n <= 0) return "0 B/s";
  return `${bytes(n)}/s`;
}

/** Seconds -> "3m 20s" / "2h 05m" / "4d 2h". */
export function eta(s: number): string {
  if (s == null || s < 0 || !isFinite(s)) return "";
  if (s < 1) return "now";
  if (s < 60) return `${Math.round(s)}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${String(Math.round(s % 60)).padStart(2, "0")}s`;
  const h = Math.floor(m / 60);
  if (h < 48) return `${h}h ${String(m % 60).padStart(2, "0")}m`;
  return `${Math.floor(h / 24)}d ${h % 24}h`;
}

export function percent(done: number, size: number): string {
  if (size <= 0) return "";
  const p = (done / size) * 100;
  return p >= 99.95 && done < size ? "99.9%" : `${p.toFixed(p < 10 ? 1 : 0)}%`;
}

export function ago(iso?: string): string {
  if (!iso || iso.startsWith("0001")) return "";
  const d = (Date.now() - new Date(iso).getTime()) / 1000;
  if (d < 60) return "just now";
  if (d < 3600) return `${Math.floor(d / 60)}m ago`;
  if (d < 86400) return `${Math.floor(d / 3600)}h ago`;
  if (d < 86400 * 7) return `${Math.floor(d / 86400)}d ago`;
  return new Date(iso).toLocaleDateString();
}

/** Parses "2.5 MB", "800k", "1g" into bytes per second; "" or 0 = unlimited. */
export function parseRate(s: string): number {
  const m = /^\s*([\d.]+)\s*([kmgt]?)i?b?\s*(\/s)?\s*$/i.exec(s);
  if (!m) return 0;
  const mult: Record<string, number> = { "": 1, k: 1024, m: 1024 ** 2, g: 1024 ** 3, t: 1024 ** 4 };
  return Math.round(parseFloat(m[1]) * mult[m[2].toLowerCase()]);
}

export function host(url: string): string {
  if (url.startsWith("magnet:")) return "magnet link";
  try {
    return new URL(url).hostname.replace(/^www\./, "");
  } catch {
    return url.length > 40 ? url.slice(0, 40) + "…" : url;
  }
}
