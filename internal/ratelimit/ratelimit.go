// Package ratelimit provides adjustable byte-rate limiters that can be
// stacked (global -> per task) and wrapped around any io.Reader.
package ratelimit

import (
	"context"
	"io"
	"sync"

	"golang.org/x/time/rate"
)

// chunk is the largest single wait we ask the limiter for. Reads larger than
// this are split so a low limit never deadlocks against the burst size.
const chunk = 32 * 1024

// Limiter is a byte-per-second limiter whose rate can be changed at runtime.
// A zero or negative rate means unlimited.
type Limiter struct {
	mu  sync.RWMutex
	lim *rate.Limiter
	bps int64
}

// New returns a limiter capped at bps bytes per second (<=0 = unlimited).
func New(bps int64) *Limiter {
	l := &Limiter{}
	l.SetRate(bps)
	return l
}

// SetRate changes the cap. Safe to call while readers are active.
func (l *Limiter) SetRate(bps int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.bps = bps
	if bps <= 0 {
		l.lim = rate.NewLimiter(rate.Inf, 0)
		return
	}
	burst := int(bps)
	if burst < chunk {
		burst = chunk
	}
	l.lim = rate.NewLimiter(rate.Limit(bps), burst)
}

// Rate returns the current cap in bytes per second (0 = unlimited).
func (l *Limiter) Rate() int64 {
	if l == nil {
		return 0
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.bps < 0 {
		return 0
	}
	return l.bps
}

// WaitN blocks until n bytes may pass.
func (l *Limiter) WaitN(ctx context.Context, n int) error {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	lim, unlimited := l.lim, l.bps <= 0
	l.mu.RUnlock()
	if unlimited {
		return nil
	}
	for n > 0 {
		step := n
		if step > lim.Burst() {
			step = lim.Burst()
		}
		if err := lim.WaitN(ctx, step); err != nil {
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
