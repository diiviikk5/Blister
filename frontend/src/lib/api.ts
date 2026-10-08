// Thin wrapper over the Wails bindings. Outside the desktop shell (plain
// `vite dev` in a browser) it falls back to a simulated engine so the UI can
// be designed and tested without Go.
import type {
  AddRequest,
  Boot,
  Link,
  ProbeResult,
  Request,
  Settings,
  Task,
  ToolStatus,
} from "./types";
import { createMock } from "./mock";

type Fn = (...args: any[]) => Promise<any>;
type Handler = (...data: any[]) => void;

declare global {
  interface Window {
    go?: { app?: { App?: Record<string, Fn> } };
    runtime?: {
      EventsOn(name: string, cb: Handler): () => void;
      WindowMinimise(): void;
      WindowToggleMaximise(): void;
      WindowIsMaximised(): Promise<boolean>;
      Quit(): void;
      ClipboardGetText(): Promise<string>;
      ClipboardSetText(text: string): Promise<boolean>;
      OnFileDrop(cb: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean): void;
    };
  }
}

export const native = typeof window !== "undefined" && !!window.go?.app?.App;
const mock = native ? null : createMock();

function call<T>(name: string, ...args: unknown[]): Promise<T> {
  if (mock) return mock.call(name, ...args) as Promise<T>;
  return window.go!.app!.App![name](...args) as Promise<T>;
}

export function on(event: string, cb: Handler): () => void {
  if (mock) return mock.on(event, cb);
  return window.runtime!.EventsOn(event, cb);
}

export const api = {
  boot: () => call<Boot>("Boot"),
  tools: () => call<ToolStatus>("Tools"),
  installYtDlp: () => call<ToolStatus>("InstallYtDlp"),
  parse: (text: string) => call<Link[] | null>("Parse", text),
  probe: (url: string, req: Partial<Request> = {}) => call<ProbeResult>("Probe", url, { url, ...req }),
  add: (reqs: AddRequest[]) => call<Task[] | null>("Add", reqs),
  pause: (ids: string[]) => call<void>("Pause", ids),
  resume: (ids: string[]) => call<void>("Resume", ids),
  restart: (ids: string[]) => call<void>("Restart", ids),
  remove: (ids: string[], deleteFiles: boolean) => call<void>("Remove", ids, deleteFiles),
  pauseAll: () => call<void>("PauseAll"),
  resumeAll: () => call<void>("ResumeAll"),
  clearCompleted: () => call<void>("ClearCompleted"),
  move: (id: string, index: number) => call<void>("Move", id, index),
  tune: (id: string, connections: number, speedLimit: number) => call<void>("Tune", id, connections, speedLimit),
  selectFiles: (id: string, selected: boolean[]) => call<void>("SelectFiles", id, selected),
  rename: (id: string, name: string) => call<void>("Rename", id, name),
  setURL: (id: string, url: string) => call<void>("SetURL", id, url),
  open: (id: string) => call<void>("Open", id),
  reveal: (id: string) => call<void>("Reveal", id),
  openDownloads: () => call<void>("OpenDownloads"),
  openURL: (url: string) => call<void>("OpenURL", url),
  settings: () => call<Settings>("Settings"),
  saveSettings: (s: Settings) => call<Settings>("SaveSettings", s),
  resetToken: () => call<Settings>("ResetToken"),
  setSpeedLimit: (bps: number) => call<Settings>("SetSpeedLimit", bps),
  pickFolder: (current: string) => call<string>("PickFolder", current),
  pickTorrents: () => call<string[] | null>("PickTorrents"),
  pickImage: () => call<string>("PickImage"),
  relaunch: () => call<void>("Relaunch"),
};

export const win = {
  minimise: () => window.runtime?.WindowMinimise(),
  toggleMaximise: () => window.runtime?.WindowToggleMaximise(),
  isMaximised: () => window.runtime?.WindowIsMaximised() ?? Promise.resolve(false),
  close: () => (native ? call<void>("Close") : Promise.resolve()),
};

export async function readClipboard(): Promise<string> {
  try {
    if (window.runtime) return (await window.runtime.ClipboardGetText()) ?? "";
    return await navigator.clipboard.readText();
  } catch {
    return "";
  }
}

export async function copyText(text: string): Promise<void> {
  if (window.runtime) {
    await window.runtime.ClipboardSetText(text);
    return;
  }
  await navigator.clipboard.writeText(text).catch(() => {});
}

export function onFileDrop(cb: (paths: string[]) => void) {
  window.runtime?.OnFileDrop((_x, _y, paths) => cb(paths), false);
}

/** Wails rejects with a plain string; normalise to a message. */
export function errText(e: unknown): string {
  if (typeof e === "string") return e;
  if (e instanceof Error) return e.message;
  return String(e);
}
