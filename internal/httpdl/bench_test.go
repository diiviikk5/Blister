package httpdl

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

// BenchmarkDownload measures loopback throughput per connection count, which
// isolates our own overhead (locking, writes, buffers) from the network.
func BenchmarkDownload(b *testing.B) {
	data := payload(64 << 20)
	srv := rangeServer(data)
	defer srv.Close()
	c, _ := NewClient(ClientOptions{})
	for _, conns := range []int{1, 4, 16} {
		b.Run(fmt.Sprintf("conns=%d", conns), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			for i := 0; i < b.N; i++ {
				out := filepath.Join(b.TempDir(), "f.part")
				d := New(Options{Client: c, Request: Request{URL: srv.URL}, Path: out,
					Size: int64(len(data)), Resumable: true, Connections: conns})
				if err := d.Run(context.Background()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
