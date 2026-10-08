// Mirrors the Go types in internal/engine, internal/config and internal/app.

export type Kind = "http" | "torrent" | "hls" | "media";
export type Status =
  | "queued"
  | "starting"
  | "downloading"
  | "paused"
  | "seeding"
  | "completed"
  | "error";

export type Category =
  | "video"
  | "audio"
  | "archive"
  | "program"
  | "document"
  | "image"
  | "other"
  | string;

export interface Segment {
  start: number;
  end: number;
  pos: number;
}

export interface Request {
  url: string;
  headers?: Record<string, string>;
  cookies?: string;
  referer?: string;
  userAgent?: string;
  username?: string;
  password?: string;
}

export interface TorrentFile {
  path: string;
  size: number;
  done: number;
  selected: boolean;
}

export interface TorrentInfo {
  infoHash: string;
  files?: TorrentFile[];
  ratio: number;
  pieces: number;
  pieceMap?: string;
}

export interface MediaInfo {
  format: string;
  audioOnly: boolean;
  title?: string;
  thumbnail?: string;
  stage?: string;
}

export interface Task {
  id: string;
  kind: Kind;
  url: string;
  name: string;
  nameFixed?: boolean;
  dir: string;
  category: Category;
  status: Status;
  error?: string;
  size: number;
  done: number;
  speed: number;
  upSpeed: number;
  uploaded: number;
  eta: number;
  conns: number;
  seeds: number;
  resumable: boolean;
  connections: number;
  speedLimit: number;
  priority: number;
  request: Request;
  segments?: Segment[];
  checksum?: string;
  verified?: boolean | null;
  torrent?: TorrentInfo;
  media?: MediaInfo;
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
  tags?: string[];
}

export interface TickItem {
  id: string;
  status: Status;
  done: number;
  size: number;
  speed: number;
  upSpeed: number;
  eta: number;
  conns: number;
  seeds: number;
  map?: string;
  heads?: number[];
}

export interface Tick {
  items: TickItem[] | null;
  speed: number;
  upSpeed: number;
  active: number;
}

export interface Settings {
  downloadDir: string;
  categorizeFolders: boolean;
  maxActive: number;
  connections: number;
  speedLimit: number;
  uploadLimit: number;
  maxRetries: number;
  proxy: string;
  userAgent: string;
  insecureTls: boolean;
  torrentPort: number;
  seedAfter: boolean;
  seedRatio: number;
  enableDht: boolean;
  enableUpnp: boolean;
  encryptPeers: boolean;
  ytDlpPath: string;
  ffmpegPath: string;
  clipboardWatch: boolean;
  browserPort: number;
  browserToken: string;
  theme: "dark" | "light" | "system";
  accent: string;
  density: "comfortable" | "compact";
  notifications: boolean;
  sounds: boolean;
  confirmDelete: boolean;
  startOnBoot: boolean;
  closeToTray: boolean;
  scheduleStart: string;
  scheduleEnd: string;
}

export interface ToolStatus {
  ytDlp: string;
  ffmpeg: string;
}

export interface Boot {
  version: string;
  tasks: Task[] | null;
  settings: Settings;
  tools: ToolStatus;
  dataDir: string;
}

export interface Link {
  url: string;
  kind: Kind;
  name: string;
}

export interface ProbeResult {
  name: string;
  size: number;
  resumable: boolean;
  category: Category;
  type: string;
  error?: string;
}

export interface AddRequest {
  url: string;
  kind?: Kind;
  name?: string;
  dir?: string;
  connections?: number;
  speedLimit?: number;
  request?: Partial<Request>;
  checksum?: string;
  paused?: boolean;
  priority?: number;
  audioOnly?: boolean;
  format?: string;
  size?: number;
  tags?: string[];
}

export interface External {
  urls: string[];
  request: Request;
  fileName?: string;
  media?: boolean;
  source: "browser" | "launch" | "clipboard";
}
