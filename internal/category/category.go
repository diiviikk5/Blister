// Package category classifies downloads by file type so they can be routed
// into tidy sub-folders and filtered in the UI.
package category

import (
	"path"
	"strings"
)

// Category is a coarse file type bucket.
type Category string

const (
	Video    Category = "video"
	Audio    Category = "audio"
	Image    Category = "image"
	Document Category = "document"
	Archive  Category = "archive"
	Program  Category = "program"
	Torrent  Category = "torrent"
	Other    Category = "other"
)

// All lists every category in display order.
var All = []Category{Video, Audio, Image, Document, Archive, Program, Torrent, Other}

var byExt = map[string]Category{}

func reg(c Category, exts ...string) {
	for _, e := range exts {
		byExt[e] = c
	}
}

func init() {
	reg(Video, "mp4", "mkv", "webm", "avi", "mov", "wmv", "flv", "m4v", "mpg", "mpeg", "3gp", "ts", "m2ts", "vob", "ogv", "m3u8")
	reg(Audio, "mp3", "flac", "wav", "aac", "m4a", "ogg", "opus", "wma", "alac", "aiff", "ape", "mka")
	reg(Image, "jpg", "jpeg", "png", "gif", "webp", "bmp", "tiff", "tif", "svg", "heic", "avif", "ico", "raw", "psd")
	reg(Document, "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "odt", "ods", "odp", "txt", "rtf", "md", "csv", "epub", "mobi", "azw3", "djvu")
	reg(Archive, "zip", "rar", "7z", "tar", "gz", "tgz", "bz2", "xz", "zst", "lz", "lzma", "cab", "iso", "img", "dmg")
	reg(Program, "exe", "msi", "msix", "appx", "apk", "aab", "deb", "rpm", "appimage", "pkg", "bat", "sh", "jar")
	reg(Torrent, "torrent")
}

// FromName classifies a file name (or URL path) by its extension.
func FromName(name string) Category {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(stripQuery(name)), "."))
	// Multi-part archive suffixes like .part1.rar / .001
	if ext != "" && isDigits(ext) {
		return Archive
	}
	if c, ok := byExt[ext]; ok {
		return c
	}
	return Other
}

// FromMIME classifies by Content-Type when the name carries no extension.
func FromMIME(mime string) Category {
	mime = strings.ToLower(strings.TrimSpace(strings.SplitN(mime, ";", 2)[0]))
	switch {
	case strings.HasPrefix(mime, "video/"), mime == "application/vnd.apple.mpegurl", mime == "application/x-mpegurl":
		return Video
	case strings.HasPrefix(mime, "audio/"):
		return Audio
	case strings.HasPrefix(mime, "image/"):
		return Image
	case mime == "application/x-bittorrent":
		return Torrent
	case strings.HasPrefix(mime, "text/"), mime == "application/pdf", strings.Contains(mime, "officedocument"), strings.Contains(mime, "msword"), mime == "application/epub+zip":
		return Document
	case strings.Contains(mime, "zip"), strings.Contains(mime, "compressed"), strings.Contains(mime, "x-tar"), strings.Contains(mime, "x-7z"), strings.Contains(mime, "rar"), strings.Contains(mime, "gzip"):
		return Archive
	case strings.Contains(mime, "msdownload"), strings.Contains(mime, "x-msi"), strings.Contains(mime, "android.package"), strings.Contains(mime, "x-executable"):
		return Program
	}
	return Other
}

// Detect prefers the extension and falls back to the MIME type.
func Detect(name, mime string) Category {
	if c := FromName(name); c != Other {
		return c
	}
	return FromMIME(mime)
}

// Folder is the default sub-folder name for a category.
func (c Category) Folder() string {
	switch c {
	case Video:
		return "Videos"
	case Audio:
		return "Music"
	case Image:
		return "Images"
	case Document:
		return "Documents"
	case Archive:
		return "Archives"
	case Program:
		return "Programs"
	case Torrent:
		return "Torrents"
	}
	return "Other"
}

func stripQuery(s string) string {
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		return s[:i]
	}
	return s
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
