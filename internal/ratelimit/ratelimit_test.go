package ratelimit

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

func TestUnlimitedIsFast(t *testing.T) {
	src := bytes.NewReader(make([]byte, 8<<20))
	start := time.Now()
	n, err := io.Copy(io.Discard, Reader(context.Background(), src, New(0)))
	if err != nil || n != 8<<20 {
		t.Fatalf("copy: n=%d err=%v", n, err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("unlimited reader too slow: %v", time.Since(start))
	}
}

func TestLimitIsEnforced(t *testing.T) {
	const bps = 256 * 1024
	// Burst admits the first second instantly; the remaining 256 KiB should
	// take roughly one more second.
	src := bytes.NewReader(make([]byte, 2*bps))
	start := time.Now()
	if _, err := io.Copy(io.Discard, Reader(context.Background(), src, New(bps))); err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el < 800*time.Millisecond {
		t.Fatalf("limit not enforced, took %v", el)
	}
}

func TestChainUsesStrictest(t *testing.T) {
	const slow = 128 * 1024
	src := bytes.NewReader(make([]byte, 2*slow))
	start := time.Now()
	if _, err := io.Copy(io.Discard, Reader(context.Background(), src, New(0), New(slow))); err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el < 800*time.Millisecond {
		t.Fatalf("chain ignored strict limiter, took %v", el)
	}
}

func TestCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	l := New(1024)
	_ = l.WaitN(ctx, 1024) // drain burst
	if err := l.WaitN(ctx, 1024); err == nil {
		t.Fatal("expected context error")
	}
}
