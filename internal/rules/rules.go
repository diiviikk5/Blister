// Package rules is Blister's automation: "when a download looks like this,
// treat it like that". Rules match on host, extension, kind and size and set
// the folder, connections, speed cap, tags, start state and a command to run
// when the download finishes.
package rules

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// Rule is one automation. Empty match fields match anything.
type Rule struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`

	// Match
	Hosts   string   `json:"hosts"` // "github.com, *.cdn.net"; a bare domain also matches its subdomains
	Exts    string   `json:"exts"`  // "zip rar 7z"
	Kinds   []string `json:"kinds"` // http, torrent, media, hls
	MinSize int64    `json:"minSize"`
	MaxSize int64    `json:"maxSize"`

	// Actions (zero values leave the default alone)
	Dir         string   `json:"dir"`
	Connections int      `json:"connections"`
	SpeedLimit  int64    `json:"speedLimit"`
	Paused      bool     `json:"paused"`
	Tags        []string `json:"tags"`
	Run         string   `json:"run"` // shell command on completion; {path} {name} {dir} {url} {size}
}

// Item is what a rule looks at.
type Item struct {
	URL  string
	Name string
	Kind string
	Size int64 // <= 0 when unknown; size conditions then don't match
}

func split(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ',' || r == ' ' || r == ';' || r == '\n' || r == '\t'
	})
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

func hostMatches(host, pattern string) bool {
	pattern = strings.TrimPrefix(pattern, "www.")
	if strings.ContainsAny(pattern, "*?") {
		ok, _ := path.Match(pattern, host)
		return ok
	}
	return host == pattern || strings.HasSuffix(host, "."+pattern)
}

func extOf(it Item) string {
	name := it.Name
	if name == "" {
		if u, err := url.Parse(it.URL); err == nil {
			name = path.Base(u.Path)
		}
	}
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}

// Matches reports whether r applies to it.
func (r Rule) Matches(it Item) bool {
	if !r.Enabled {
		return false
	}
	if hosts := split(r.Hosts); len(hosts) > 0 {
		h := hostOf(it.URL)
		ok := false
		for _, p := range hosts {
			if hostMatches(h, p) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if exts := split(r.Exts); len(exts) > 0 {
		e := extOf(it)
		ok := false
		for _, x := range exts {
			if strings.TrimPrefix(x, ".") == e {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(r.Kinds) > 0 {
		ok := false
		for _, k := range r.Kinds {
			if strings.EqualFold(k, it.Kind) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if r.MinSize > 0 && (it.Size <= 0 || it.Size < r.MinSize) {
		return false
	}
	if r.MaxSize > 0 && (it.Size <= 0 || it.Size > r.MaxSize) {
		return false
	}
	return true
}

// Effect is the combined result of every matching rule. Later rules win
// for single values; tags and commands accumulate.
type Effect struct {
	Dir         string
	Connections int
	SpeedLimit  int64
	Paused      bool
	Tags        []string
	Matched     []string // rule IDs
}

// Evaluate runs all rules against it.
func Evaluate(rs []Rule, it Item) Effect {
	var e Effect
	for _, r := range rs {
		if !r.Matches(it) {
			continue
		}
		e.Matched = append(e.Matched, r.ID)
		if r.Dir != "" {
			e.Dir = r.Dir
		}
		if r.Connections > 0 {
			e.Connections = r.Connections
		}
		if r.SpeedLimit > 0 {
			e.SpeedLimit = r.SpeedLimit
		}
		e.Paused = e.Paused || r.Paused
		e.Tags = append(e.Tags, r.Tags...)
	}
	return e
}

// Expand fills a command template. Values are quoted for cmd/sh so paths
// with spaces survive.
func Expand(tmpl string, vars map[string]string) string {
	out := tmpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{"+k+"}", `"`+strings.ReplaceAll(v, `"`, ``)+`"`)
	}
	return out
}
