import type { Task } from "./types";
import { catOf } from "./store.svelte";

export const catIcon: Record<string, string> = {
  video: "film",
  audio: "music",
  torrent: "magnet",
  archive: "box",
  program: "app",
  document: "doc",
  image: "image",
  other: "file",
};

/** Tile colours per type; ink text sits on all of them. */
export const catColor: Record<string, string> = {
  video: "#ff7a45",
  audio: "#ffd60a",
  torrent: "#9d8cff",
  archive: "#c9a27a",
  program: "#2fd27a",
  document: "#f2ecdf",
  image: "#ff8fb1",
  other: "#b9b1a6",
};

export function iconFor(t: Task) {
  const c = catOf(t);
  return { icon: catIcon[c] ?? "file", color: catColor[c] ?? catColor.other };
}

export const statusLabel: Record<string, string> = {
  queued: "Queued",
  starting: "Starting",
  downloading: "Downloading",
  paused: "Paused",
  seeding: "Seeding",
  completed: "Done",
  error: "Failed",
};

export const kindLabel: Record<string, string> = {
  http: "HTTP",
  torrent: "Torrent",
  media: "Video",
  hls: "Stream",
};
