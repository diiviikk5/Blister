<p align="center">
  <img src="build/appicon.png" width="96" alt="Blister" />
</p>

<h1 align="center">Blister</h1>

<p align="center"><b>The download manager that doesn't wait.</b><br/>
Files, torrents and videos from 1,800+ sites, split across up to 64 connections, in one small native app.</p>

---

## What it does

- **Multi-connection downloads.** Every file is split into segments that download in parallel (16 by default, up to 64). When a connection finishes early it takes over half of the slowest one's remaining range, so the tail of a download never crawls on a single connection.
- **Resume anything.** Pause, quit, reboot: progress is saved per segment and picks up where it left off, as long as the server supports ranges.
- **Torrents built in.** Magnet links and `.torrent` files with DHT, UPnP, peer encryption, per-file selection, a live piece map and seed-to-ratio.
- **Video sites.** YouTube, X, Instagram, TikTok, Reddit, Vimeo, Twitch and many more through yt-dlp, which Blister installs and keeps updated for you. Pick 4K, 1080p, 720p or audio-only MP3.
- **Catches links.** Copy a link anywhere and Blister offers to grab it. The browser extension sends downloads over with their cookies and referer, so logged-in downloads work.
- **Batch links.** Paste a page of text and every link is found. Patterns like `part[01-12].rar` expand into twelve downloads.
- **Queue and schedule.** Choose how many run at once, cap speed globally or per download (and change it live), or only download overnight.
- **Checksums.** Paste a `sha256:` / `sha1:` / `md5:` and the file is verified when it finishes.
- **Tidy by default.** Files land in Videos, Music, Archives, Programs… inside your download folder.
- **No accounts, ads or telemetry.**

## Rice it

Blister is built to be riced. Open **Rice Studio** (`Ctrl Shift R`) and everything is live:

- **13 themes** to start from: Thermal, Lilac Paper, Phosphor (green CRT terminal), Catppuccin Mocha, Gruvbox, Nord, Tokyo Night, Rosé Pine, Dracula, Everforest, Neon '84, Mono and Solar Paper. Or hit **Shuffle** for a fresh readable palette.
- **Colours**: all 15 tokens, with a live contrast checker.
- **Shape**: outline width, corner radius, depth, and shadow style (3D extrusion, hard, soft, glow or flat).
- **Type**: any installed font (Nerd Fonts welcome), monospace-everything, scale, heading weight and tracking.
- **Layout**: list, dense table or card grid; line or card rows; three densities; sidebar left, right or hidden; details right, bottom or off.
- **Progress bars**: lanes (live connection map), solid, line, `▰▰▱` blocks, `[##=->..]` ASCII, braille or dots.
- **Effects**: wallpaper with dim and blur, see-through surfaces, film grain, CRT scanlines, vignette, motion off/normal/springy, and Windows 11 **Mica / Acrylic** window backdrops.
- **Custom CSS** applied as you type, with snippets.
- **Share**: every look is a single `blister-rice:` code. Copy yours, paste someone else's, or save variations to your library.

Plus:

- **`Ctrl K` command palette**: every action, theme, layout, filter and download.
- **Mini bar** (`Ctrl M`): the whole app as a slim always-on-top strip with live speed and progress.
- **3D lanes**: the selected download as 16 isometric blocks filling from the live coverage map.
- **Rules**: "when the host, extension, kind or size looks like this, use this folder, connection count, speed cap, tags or start paused". They apply to links from anywhere.
- **Hooks**: run any command when a download finishes, globally or per rule, with `{path}` `{name}` `{dir}` `{url}` `{size}`. For example, unzip archives automatically.

## Install

Grab `Blister.exe` from [Releases](https://github.com/diiviikk5/Blister/releases/latest) and run it. Windows 10/11 with the WebView2 runtime, which ships with Windows 11 and current Windows 10.

### Browser extension (Chrome, Edge, Brave)

1. Open `chrome://extensions`, turn on **Developer mode** and choose **Load unpacked**.
2. Select the `extension` folder from this repo.
3. In Blister open **Settings → Browser**, copy the pairing code and paste it into the extension popup.

From then on downloads go to Blister (small files under 1 MB stay in the browser; change that in the popup). Right-click links, videos or selected text for **Download with Blister**. If Blister isn't running, the browser simply downloads as usual.

## Keyboard

| Keys | Action |
| --- | --- |
| `Ctrl V` (anywhere in the list) | Add the copied links |
| `Ctrl N` | Add download |
| `Ctrl Enter` | Start downloads from the add dialog |
| `Space` | Pause or resume selected |
| `Delete` | Remove selected |
| `F2` | Rename |
| `Enter` | Open finished file |
| `Ctrl F` | Search |
| `Ctrl A` | Select all |
| `Ctrl ,` | Settings |
| `Ctrl K` | Command palette |
| `Ctrl Shift R` | Rice Studio |
| `Ctrl M` | Mini bar |

## Build from source

Requirements: Go 1.26+, Node 22+, and the [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation).

```bash
wails build
```

The app lands in `build/bin/Blister.exe`. For live reload while working on the UI:

```bash
wails dev
```

The UI also runs on its own against a simulated engine, which is handy for design work:

```bash
cd frontend && npm install && npm run dev
```

Tests:

```bash
go test ./...
```

## How it's put together

```
main.go              Wails window, single-instance handoff, file drops
internal/app         Everything the UI can call; clipboard watch; notifications
internal/engine      Queue, scheduler, telemetry, persistence, the HTTP driver
internal/httpdl      Segmented downloader: probing, dynamic splitting, retries
internal/bt          BitTorrent (anacrolix/torrent) with plain-file storage
internal/media       yt-dlp driver for video sites and HLS
internal/bridge      127.0.0.1-only API for the browser extension
internal/ratelimit   Stackable token-bucket limiters (global → per download)
internal/linkgrab    Link extraction and [01-12] batch expansion
internal/rules       Automation rules and hook templates
internal/category    File-type sorting
frontend/            Svelte 5 UI in the isometric-brutalism design kit
extension/           Manifest V3 browser extension
website/             blister's landing page
```

The engine talks to the UI with two kinds of events: full task updates when something changes, and a compact tick twice a second with speed, ETA and a 120-cell coverage map of each file plus the positions of the connections currently writing. That map is what draws the segmented bar.

## Where things live

- Settings and the download list: `%APPDATA%\Blister`
- Downloads: `Downloads\Blister` by default (change it in Settings)
- yt-dlp: `%APPDATA%\Blister\tools`
