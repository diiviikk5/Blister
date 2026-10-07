// Package linkgrab pulls downloadable links out of free-form text and expands
// batch patterns such as https://host/ep[01-24].mkv or img[a-f].png.
package linkgrab

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// MaxExpand caps how many links a single batch pattern may produce.
const MaxExpand = 10000

var (
	urlRe    = regexp.MustCompile(`(?i)\b(?:https?|ftp)://[^\s"'<>\x60]+`)
	magnetRe = regexp.MustCompile(`(?i)\bmagnet:\?[^\s"'<>\x60]+`)
	rangeRe  = regexp.MustCompile(`\[(\d+)-(\d+)(?::(\d+))?\]|\[([a-zA-Z])-([a-zA-Z])\]`)
)

// Extract returns every unique http(s)/ftp/magnet link in text, in order of
// appearance. Batch patterns are expanded.
func Extract(text string) []string {
	type hit struct {
		pos int
		s   string
	}
	var hits []hit
	for _, m := range magnetRe.FindAllStringIndex(text, -1) {
		hits = append(hits, hit{m[0], text[m[0]:m[1]]})
	}
	for _, m := range urlRe.FindAllStringIndex(text, -1) {
		hits = append(hits, hit{m[0], text[m[0]:m[1]]})
	}
	// order by position (insertion sort; lists are small)
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].pos < hits[j-1].pos; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}

	seen := map[string]bool{}
	var out []string
	for _, h := range hits {
		raw := strings.TrimRight(h.s, ".,;:!?)]}>")
		// keep a closing bracket that belongs to a batch pattern
		if strings.Count(h.s, "[") > strings.Count(raw, "]") && strings.HasSuffix(h.s, "]") {
			raw = h.s
		}
		expanded, err := Expand(raw)
		if err != nil {
			expanded = []string{raw}
		}
		for _, u := range expanded {
			if !seen[u] {
				seen[u] = true
				out = append(out, u)
			}
		}
	}
	return out
}

// Expand expands the first batch pattern in s, recursively, so multiple
// patterns produce the cartesian product. Numeric ranges keep zero padding
// from the start value ([001-120]) and accept a step ([0-100:5]).
func Expand(s string) ([]string, error) {
	loc := rangeRe.FindStringSubmatchIndex(s)
	if loc == nil {
		return []string{s}, nil
	}
	prefix, suffix := s[:loc[0]], s[loc[1]:]
	sub := func(i int) string {
		if loc[2*i] < 0 {
			return ""
		}
		return s[loc[2*i]:loc[2*i+1]]
	}

	var parts []string
	if a := sub(1); a != "" {
		b, stepS := sub(2), sub(3)
		from, _ := strconv.Atoi(a)
		to, _ := strconv.Atoi(b)
		step := 1
		if stepS != "" {
			step, _ = strconv.Atoi(stepS)
			if step <= 0 {
				return nil, fmt.Errorf("invalid step %q", stepS)
			}
		}
		width := 0
		if len(a) > 1 && a[0] == '0' {
			width = len(a)
		}
		if from <= to {
			for i := from; i <= to; i += step {
				parts = append(parts, fmt.Sprintf("%0*d", width, i))
			}
		} else {
			for i := from; i >= to; i -= step {
				parts = append(parts, fmt.Sprintf("%0*d", width, i))
			}
		}
	} else {
		a, b := sub(4)[0], sub(5)[0]
		if a <= b {
			for c := a; c <= b; c++ {
				parts = append(parts, string(c))
			}
		} else {
			for c := a; c >= b; c-- {
				parts = append(parts, string(c))
			}
		}
	}
	if len(parts) > MaxExpand {
		return nil, fmt.Errorf("pattern expands to %d links (max %d)", len(parts), MaxExpand)
	}

	var out []string
	for _, p := range parts {
		rest, err := Expand(p + suffix)
		if err != nil {
			return nil, err
		}
		for _, r := range rest {
			out = append(out, prefix+r)
			if len(out) > MaxExpand {
				return nil, fmt.Errorf("pattern expands to more than %d links", MaxExpand)
			}
		}
	}
	return out, nil
}

// IsMagnet reports whether s is a magnet URI.
func IsMagnet(s string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), "magnet:?")
}

// Valid reports whether s looks like a link Blister can fetch.
func Valid(s string) bool {
	if IsMagnet(s) {
		return true
	}
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || u.Host == "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "ftp":
		return true
	}
	return false
}
