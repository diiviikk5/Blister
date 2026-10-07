package httpdl

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
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
	// Drain at most a little so the connection can be reused.
	_, _ = io.CopyN(io.Discard, resp.Body, 4096)

	if resp.StatusCode >= 400 {
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
		if total := parseContentRangeTotal(resp.Header.Get("Content-Range")); total >= 0 {
			p.Size = total
			p.Resumable = true
		}
	case http.StatusOK:
		if resp.ContentLength >= 0 {
			p.Size = resp.ContentLength
		}
		// 200 to a range request means the server ignores ranges, unless it
		// advertises them anyway (rare but harmless to trust).
		p.Resumable = strings.EqualFold(resp.Header.Get("Accept-Ranges"), "bytes") && p.Size > 0
	}
	p.FileName = FileName(resp.Header.Get("Content-Disposition"), resp.Request.URL, p.ContentType)
	return p, nil
}

// parseContentRangeTotal returns the total from "bytes 0-0/12345", or -1.
func parseContentRangeTotal(v string) int64 {
	i := strings.LastIndexByte(v, '/')
	if i < 0 {
		return -1
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v[i+1:]), 10, 64)
	if err != nil {
		return -1
	}
	return n
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
		name = name[:200-len(ext)] + ext
	}
	upper := strings.ToUpper(strings.TrimSuffix(name, path.Ext(name)))
	switch upper {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "LPT1", "LPT2", "LPT3":
		name = "_" + name
	}
	return name
}
