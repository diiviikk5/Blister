package httpdl

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"
)

// DefaultUserAgent is sent when the caller does not supply one. Many file
// hosts serve slower or refuse unknown agents, so we look like a browser.
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// ClientOptions tunes the shared HTTP client.
type ClientOptions struct {
	// Proxy is an http(s):// or socks5:// URL. Empty uses the system/env proxy.
	Proxy string
	// InsecureTLS skips certificate verification (for self-signed mirrors).
	InsecureTLS bool
	// ConnectTimeout bounds TCP + TLS setup.
	ConnectTimeout time.Duration
}

// NewClient builds an HTTP client tuned for bulk parallel transfers.
//
// HTTP/2 is deliberately disabled: it multiplexes every request to a host onto
// a single TCP connection, which defeats the point of segmented downloading.
// Separate HTTP/1.1 connections each get their own congestion window, which is
// where multi-connection speedups come from.
func NewClient(o ClientOptions) (*http.Client, error) {
	if o.ConnectTimeout == 0 {
		o.ConnectTimeout = 20 * time.Second
	}
	dialer := &net.Dialer{Timeout: o.ConnectTimeout, KeepAlive: 30 * time.Second}

	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     false,
		TLSNextProto:          map[string]func(string, *tls.Conn) http.RoundTripper{},
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: o.InsecureTLS}, //nolint:gosec // user opt-in
		TLSHandshakeTimeout:   o.ConnectTimeout,
		ResponseHeaderTimeout: 45 * time.Second,
		ExpectContinueTimeout: time.Second,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		// Compression would make byte ranges meaningless.
		DisableCompression: true,
		ReadBufferSize:     256 << 10,
		WriteBufferSize:    32 << 10,
	}
	if o.Proxy != "" {
		pu, err := url.Parse(o.Proxy)
		if err != nil {
			return nil, err
		}
		tr.Proxy = http.ProxyURL(pu)
	}
	return &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 15 {
				return http.ErrUseLastResponse
			}
			// Carry our headers across redirects (Go drops some by default
			// when the host changes, but keeps User-Agent/Referer/Cookie
			// only if we copy them).
			for _, k := range []string{"User-Agent", "Referer", "Accept"} {
				if v := via[0].Header.Get(k); v != "" && req.Header.Get(k) == "" {
					req.Header.Set(k, v)
				}
			}
			return nil
		},
	}, nil
}
