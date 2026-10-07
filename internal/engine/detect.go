package engine

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/diiviikk5/Blister/internal/linkgrab"
)

// mediaHosts are sites where the page URL isn't the file: yt-dlp knows how to
// find the real streams.
var mediaHosts = []string{
	"youtube.com", "youtu.be", "music.youtube.com",
	"vimeo.com", "dailymotion.com", "twitch.tv",
	"twitter.com", "x.com", "tiktok.com", "instagram.com", "facebook.com", "fb.watch",
	"reddit.com", "v.redd.it", "soundcloud.com", "bandcamp.com", "mixcloud.com",
	"bilibili.com", "nicovideo.jp", "streamable.com", "rumble.com", "kick.com",
	"pinterest.com", "tumblr.com", "bsky.app", "threads.net", "linkedin.com",
}

// DetectKind picks the right driver for a link.
func DetectKind(raw string) Kind {
	if linkgrab.IsMagnet(raw) {
		return KindTorrent
	}
	if IsLocalPath(raw) {
		if strings.EqualFold(filepath.Ext(raw), ".torrent") {
			return KindTorrent
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return KindHTTP
	}
	p := strings.ToLower(u.Path)
	switch {
	case strings.HasSuffix(p, ".torrent"):
		return KindTorrent
	case strings.HasSuffix(p, ".m3u8"):
		return KindHLS
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	host = strings.TrimPrefix(host, "m.")
	for _, h := range mediaHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return KindMedia
		}
	}
	return KindHTTP
}

// IsLocalPath reports whether s is a filesystem path rather than a URL
// ("C:\x.torrent" parses as a URL with scheme "c", so check explicitly).
func IsLocalPath(s string) bool {
	if strings.HasPrefix(s, "file://") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, `\\`) {
		return true
	}
	return len(s) > 2 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') &&
		((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z'))
}

// GuessName derives a provisional display name from a link.
func GuessName(raw string, k Kind) string {
	if k == KindTorrent && linkgrab.IsMagnet(raw) {
		if u, err := url.Parse(raw); err == nil {
			if dn := u.Query().Get("dn"); dn != "" {
				return dn
			}
			if xt := u.Query().Get("xt"); xt != "" {
				h := xt[strings.LastIndexByte(xt, ':')+1:]
				if len(h) > 12 {
					h = h[:12]
				}
				return "Torrent " + h
			}
		}
		return "Torrent"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "download"
	}
	if k == KindMedia {
		return u.Hostname() + " media"
	}
	base := path.Base(u.Path)
	if un, err := url.PathUnescape(base); err == nil {
		base = un
	}
	if base == "" || base == "/" || base == "." {
		base = u.Hostname()
	}
	if k == KindHLS {
		base = strings.TrimSuffix(base, path.Ext(base))
		if base == "index" || base == "master" || base == "playlist" || base == "" {
			base = u.Hostname() + " stream"
		}
		base += ".ts"
	}
	return base
}

// InWindow reports whether now falls inside the daily HH:MM window
// [start, end). An empty bound means "always"; windows may wrap midnight.
func InWindow(now time.Time, start, end string) bool {
	if start == "" || end == "" || start == end {
		return true
	}
	s, err1 := time.Parse("15:04", start)
	e, err2 := time.Parse("15:04", end)
	if err1 != nil || err2 != nil {
		return true
	}
	cur := now.Hour()*60 + now.Minute()
	sm := s.Hour()*60 + s.Minute()
	em := e.Hour()*60 + e.Minute()
	if sm < em {
		return cur >= sm && cur < em
	}
	return cur >= sm || cur < em
}
