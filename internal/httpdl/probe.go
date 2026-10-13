package httpdl

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Request describes what to fetch and how to look while fetching it.
type Request struct {
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers,omitempty"`
	Cookies   string            `json:"cookies,omitempty"`
	Referer   string            `json:"referer,omitempty"`
	UserAgent string            `json:"userAgent,omitempty"`
	Username  string            `json:"username,omitempty"`
	Password  string            `json:"password,omitempty"`
}

// Probe is what we learn about a remote file before downloading it.
type Probe struct {
	FinalURL     string `json:"finalUrl"`
	Size         int64  `json:"size"` // -1 when unknown
	Resumable    bool   `json:"resumable"`
	FileName     string `json:"fileName"`
	ContentType  string `json:"contentType"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
}

// HTTPError is a non-success HTTP status.
type HTTPError struct {
	Status int
	URL    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("server returned %d %s", e.Status, http.StatusText(e.Status))
}

// Temporary reports whether retrying could help.
func (e *HTTPError) Temporary() bool {
	return e.Status == 408 || e.Status == 425 || e.Status == 429 || e.Status >= 500
}

// NewRequest carries browser authentication while clearing cache/range headers
// that could return a partial body or a 304 instead of the requested file.
func (r Request) NewRequest(ctx context.Context, method string) (*http.Request, error) {
	return r.build(ctx, method, "")
}

func (r Request) build(ctx context.Context, method, rawURL string) (*http.Request, error) {
	if rawURL == "" {
		rawURL = r.URL
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	ua := r.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")
	if r.Referer != "" {
		req.Header.Set("Referer", r.Referer)
	}
	if r.Cookies != "" {
		req.Header.Set("Cookie", r.Cookies)
	}
	if r.Username != "" || r.Password != "" {
		req.SetBasicAuth(r.Username, r.Password)
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	// These are controlled by the engine, never by a captured browser request.
	for _, k := range []string{"Range", "If-Range", "If-None-Match", "If-Modified-Since"} {
		req.Header.Del(k)
	}
	req.Header.Set("Accept-Encoding", "identity")
	return req, nil
}

// ProbeURL asks the server about the file. It uses a one-byte ranged GET
// rather than HEAD because a surprising number of CDNs answer HEAD wrongly.
func ProbeURL(ctx context.Context, c *http.Client, r Request) (*Probe, error) {
	req, err := r.build(ctx, http.MethodGet, "")
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Empty files commonly answer bytes=0-0 with 416 and bytes */0.
	empty := resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && strings.TrimSpace(resp.Header.Get("Content-Range")) == "bytes */0"
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent && !empty {
		return nil, &HTTPError{Status: resp.StatusCode, URL: r.URL}
	}

	p := &Probe{
		FinalURL:     resp.Request.URL.String(),
		Size:         -1,
		ContentType:  resp.Header.Get("Content-Type"),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}
	switch resp.StatusCode {
	case http.StatusPartialContent:
		start, end, total, err := parseContentRange(resp.Header.Get("Content-Range"))
		if err != nil || start != 0 || end != 0 {
			return nil, ErrInvalidRange
		}
		p.Size, p.Resumable = total, true
	case http.StatusOK:
		if resp.ContentLength >= 0 {
			p.Size = resp.ContentLength
		}
		// Accept-Ranges is only a claim; the response must prove support.
	case http.StatusRequestedRangeNotSatisfiable:
		p.Size = 0
	}
	p.FileName = FileName(resp.Header.Get("Content-Disposition"), resp.Request.URL, p.ContentType)
	return p, nil
}

// parseContentRange validates an inclusive range with a known total size.
func parseContentRange(v string) (start, end, total int64, err error) {
	fields := strings.Fields(v)
	if len(fields) != 2 || fields[0] != "bytes" {
		return 0, 0, 0, ErrInvalidRange
	}
	span, length, ok := strings.Cut(fields[1], "/")
	first, last, rangeOK := strings.Cut(span, "-")
	if !ok || !rangeOK {
		return 0, 0, 0, ErrInvalidRange
	}
	start, e1 := strconv.ParseInt(first, 10, 64)
	end, e2 := strconv.ParseInt(last, 10, 64)
	total, e3 := strconv.ParseInt(length, 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || start < 0 || end < start || total <= end {
		return 0, 0, 0, ErrInvalidRange
	}
	return start, end, total, nil
}

// FileName picks the best local name from Content-Disposition, the URL path,
// and finally the MIME type.
func FileName(disposition string, u *url.URL, contentType string) string {
	name := ""
	if disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			name = params["filename"]
		} else if i := strings.Index(strings.ToLower(disposition), "filename="); i >= 0 {
			name = strings.Trim(strings.SplitN(disposition[i+9:], ";", 2)[0], `" `)
		}
	}
	if name == "" && u != nil {
		if base := path.Base(u.Path); base != "/" && base != "." {
			if un, err := url.PathUnescape(base); err == nil {
				name = un
			} else {
				name = base
			}
		}
	}
	name = Sanitize(name)
	if name == "" {
		name = "download"
	}
	if path.Ext(name) == "" && contentType != "" {
		mt, _, _ := mime.ParseMediaType(contentType)
		if mt != "" && mt != "application/octet-stream" && mt != "text/html" {
			if exts, _ := mime.ExtensionsByType(mt); len(exts) > 0 {
				name += exts[len(exts)-1]
			}
		}
	}
	return name
}

// Sanitize strips characters Windows (the strictest target) forbids.
func Sanitize(name string) string {
	name = strings.Map(func(r rune) rune {
		switch {
		case r < 32, strings.ContainsRune(`<>:"/\|?*`, r):
			return '_'
		}
		return r
	}, name)
	name = strings.TrimRight(strings.TrimSpace(name), ". ")
	if len(name) > 200 {
		ext := path.Ext(name)
		if len(ext) > 20 {
			ext = ""
		}
		base := name[:200-len(ext)]
		for !utf8.ValidString(base) {
			base = base[:len(base)-1]
		}
		name = base + ext
	}
	stem, _, _ := strings.Cut(name, ".")
	upper := strings.ToUpper(strings.TrimSpace(stem))
	switch upper {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		name = "_" + name
	}
	return name
}
