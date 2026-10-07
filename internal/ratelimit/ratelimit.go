// Package ratelimit provides adjustable byte-rate limiters that can be
// stacked (global -> per task) and wrapped around any io.Reader.
package ratelimit

import (
	"context"
	"io"
	"sync/atomic"

	"golang.org/x/time/rate"
)

// chunk is the largest single wait we ask the limiter for. Reads larger than
// this are split so a low limit never deadlocks against the burst size.
const chunk = 32 * 1024

// Limiter is a byte-per-second limiter whose rate can be changed at runtime.
// A zero or negative rate means unlimited. The underlying token bucket is
// stable for the limiter's lifetime, so it can be shared with libraries that
// take a *rate.Limiter (e.g. the torrent engine).
type Limiter struct {
	lim *rate.Limiter
	bps atomic.Int64
}

// New returns a limiter capped at bps bytes per second (<=0 = unlimited).
func New(bps int64) *Limiter {
	l := &Limiter{lim: rate.NewLimiter(rate.Inf, chunk)}
	l.SetRate(bps)
	return l
}

// SetRate changes the cap. Safe to call while readers are active.
func (l *Limiter) SetRate(bps int64) {
	if bps <= 0 {
		l.bps.Store(0)
		l.lim.SetLimit(rate.Inf)
		return
	}
	burst := int(bps)
	if burst < chunk {
		burst = chunk
	}
	l.bps.Store(bps)
	l.lim.SetBurst(burst)
	l.lim.SetLimit(rate.Limit(bps))
}

// Rate returns the current cap in bytes per second (0 = unlimited).
func (l *Limiter) Rate() int64 {
	if l == nil {
		return 0
	}
	return l.bps.Load()
}

// Raw exposes the token bucket for libraries that accept *rate.Limiter.
func (l *Limiter) Raw() *rate.Limiter { return l.lim }

// WaitN blocks until n bytes may pass.
func (l *Limiter) WaitN(ctx context.Context, n int) error {
	if l == nil || l.bps.Load() <= 0 {
		return nil
	}
	for n > 0 {
		step := n
		if b := l.lim.Burst(); step > b {
			step = b
		}
		if err := l.lim.WaitN(ctx, step); err != nil {
			return err
		}
		n -= step
	}
	return nil
}

// Chain is an ordered set of limiters that must all admit a read.
type Chain []*Limiter

// WaitN waits on every limiter in the chain.
func (c Chain) WaitN(ctx context.Context, n int) error {
	for _, l := range c {
		if err := l.WaitN(ctx, n); err != nil {
			return err
		}
	}
	return nil
}

type reader struct {
	ctx   context.Context
	r     io.Reader
	chain Chain
}

// Reader wraps r so every read is throttled by all limiters in chain.
func Reader(ctx context.Context, r io.Reader, chain ...*Limiter) io.Reader {
	return &reader{ctx: ctx, r: r, chain: chain}
}

func (r *reader) Read(p []byte) (int, error) {
	if len(p) > chunk {
		p = p[:chunk]
	}
	n, err := r.r.Read(p)
	if n > 0 {
		if werr := r.chain.WaitN(r.ctx, n); werr != nil {
			return n, werr
		}
	}
	return n, err
}
