package httpdl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diiviikk5/Blister/internal/ratelimit"
)

// Segment is a byte range [Start, End) of which [Start, Pos) is on disk.
// End == -1 means "until EOF" (size unknown, single stream).
type Segment struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
	Pos   int64 `json:"pos"`
}

// Remaining bytes in the segment, or -1 if open ended.
func (s Segment) Remaining() int64 {
	if s.End < 0 {
		return -1
	}
	return s.End - s.Pos
}

// Options configures one download.
type Options struct {
	Client  *http.Client
	Request Request
	// Path is the file bytes are written to (usually "<name>.part").
	Path string
	// Size is the total length, or -1 if unknown.
	Size int64
	// Resumable means the server honours byte ranges.
	Resumable bool
	// Connections is the target number of parallel connections.
	Connections int
	// MinSegment is the smallest range worth splitting off for a new
	// connection. Defaults to 1 MiB.
	MinSegment int64
	// Segments is saved state from a previous run, used to resume.
	Segments []Segment
	// Limiters throttle all bytes read (e.g. global + per task).
	Limiters []*ratelimit.Limiter
	// MaxRetries per connection without any progress before giving up.
	MaxRetries int
	// StallTimeout aborts and retries a connection that delivers nothing.
	StallTimeout time.Duration
}

type segment struct {
	Segment
	busy bool
}

// Downloader fetches one file over many connections. It is IDM-style: work
// is split up front, and whenever a connection runs out of work it steals
// the back half of the largest remaining segment, so fast connections keep
// helping slow ones until the very last byte.
type Downloader struct {
	o Options

	mu     sync.Mutex
	segs   []*segment
	target int
	active int

	written  atomic.Int64
	conns    atomic.Int32
	wg       sync.WaitGroup
	file     *os.File
	ctx      context.Context
	fail     context.CancelCauseFunc
	spawnCh  chan struct{}
	finished chan struct{}
}

const bufSize = 128 << 10

// ErrRangeIgnored means a server answered a ranged request with the full body.
var ErrRangeIgnored = errors.New("server ignored byte range request")

// New prepares a downloader. Call Run to start it.
func New(o Options) *Downloader {
	if o.MinSegment <= 0 {
		o.MinSegment = 1 << 20
	}
	if o.Connections <= 0 {
		o.Connections = 8
	}
	if o.MaxRetries <= 0 {
		o.MaxRetries = 8
	}
	if o.StallTimeout <= 0 {
		o.StallTimeout = 30 * time.Second
	}
	if !o.Resumable || o.Size <= 0 {
		o.Connections = 1
	}
	d := &Downloader{o: o, target: o.Connections, spawnCh: make(chan struct{}, 1), finished: make(chan struct{})}
	d.initSegments()
	return d
}

func (d *Downloader) initSegments() {
	o := d.o
	if !o.Resumable || o.Size <= 0 {
		// Can't seek: always restart from zero.
		d.segs = []*segment{{Segment: Segment{Start: 0, End: o.Size, Pos: 0}}}
		if o.Size <= 0 {
			d.segs[0].End = -1
		}
		return
	}
	if len(o.Segments) > 0 && validSegments(o.Segments, o.Size) {
		for _, s := range o.Segments {
			if s.Pos < s.End {
				d.segs = append(d.segs, &segment{Segment: s})
			}
			d.written.Add(s.Pos - s.Start)
		}
		return
	}
	n := int64(o.Connections)
	if per := o.Size / n; per < o.MinSegment {
		n = o.Size / o.MinSegment
		if n < 1 {
			n = 1
		}
	}
	per := o.Size / n
	for i := int64(0); i < n; i++ {
		s := i * per
		e := s + per
		if i == n-1 {
			e = o.Size
		}
		d.segs = append(d.segs, &segment{Segment: Segment{Start: s, End: e, Pos: s}})
	}
}

func validSegments(segs []Segment, size int64) bool {
	cp := append([]Segment(nil), segs...)
	sort.Slice(cp, func(i, j int) bool { return cp[i].Start < cp[j].Start })
	var next int64
	for _, s := range cp {
		if s.Start != next || s.End < s.Start || s.Pos < s.Start || s.Pos > s.End {
			return false
		}
		next = s.End
	}
	return next == size
}

// Written is the number of bytes on disk so far.
func (d *Downloader) Written() int64 { return d.written.Load() }

// ActiveConnections is the number of connections currently transferring.
func (d *Downloader) ActiveConnections() int { return int(d.conns.Load()) }

// Snapshot returns the resumable state, including finished segments so the
// layout always covers the whole file.
func (d *Downloader) Snapshot() []Segment {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]Segment, 0, len(d.segs))
	for _, s := range d.segs {
		out = append(out, s.Segment)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	// Fill the gaps left by segments we dropped from memory as completed.
	if d.o.Size > 0 && d.o.Resumable {
		var filled []Segment
		var next int64
		for _, s := range out {
			if s.Start > next {
				filled = append(filled, Segment{Start: next, End: s.Start, Pos: s.Start})
			}
			filled = append(filled, s)
			next = s.End
		}
		if next < d.o.Size {
			filled = append(filled, Segment{Start: next, End: d.o.Size, Pos: d.o.Size})
		}
		out = filled
	}
	return out
}

// SetConnections changes the parallelism of a running download.
func (d *Downloader) SetConnections(n int) {
	if n < 1 {
		n = 1
	}
	d.mu.Lock()
	if !d.o.Resumable || d.o.Size <= 0 {
		n = 1
	}
	d.target = n
	d.mu.Unlock()
	select {
	case d.spawnCh <- struct{}{}:
	default:
	}
}

// Run downloads until the file is complete, ctx is cancelled, or a
// connection fails permanently.
func (d *Downloader) Run(ctx context.Context) error {
	flags := os.O_CREATE | os.O_WRONLY
	if !d.o.Resumable || d.o.Size <= 0 {
		flags |= os.O_TRUNC
		d.written.Store(0)
	}
	f, err := os.OpenFile(d.o.Path, flags, 0o644)
	if err != nil {
		return err
	}
	d.file = f
	defer f.Close()
	if d.o.Size > 0 {
		if st, err := f.Stat(); err == nil && st.Size() != d.o.Size {
			// Pre-size the (sparse) file so every connection can WriteAt
			// anywhere without the OS zero-filling the gaps first.
			makeSparse(f)
			if err := f.Truncate(d.o.Size); err != nil {
				return fmt.Errorf("allocate %d bytes: %w", d.o.Size, err)
			}
		}
	}

	runCtx, fail := context.WithCancelCause(ctx)
	defer fail(nil)
	d.ctx, d.fail = runCtx, fail

	d.spawn()
	go func() {
		for {
			select {
			case <-d.spawnCh:
				d.spawn()
			case <-d.finished:
				return
			case <-runCtx.Done():
				return
			}
		}
	}()
	d.wg.Wait()
	close(d.finished)

	if err := context.Cause(runCtx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !d.done() {
		return errors.New("download ended with missing data")
	}
	return nil
}

func (d *Downloader) done() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.segs {
		if s.End < 0 || s.Pos < s.End {
			return false
		}
	}
	return true
}

// spawn starts workers until we reach the target or run out of work.
func (d *Downloader) spawn() {
	for {
		d.mu.Lock()
		if d.active >= d.target {
			d.mu.Unlock()
			return
		}
		seg := d.acquireLocked()
		if seg == nil {
			d.mu.Unlock()
			return
		}
		d.active++
		d.mu.Unlock()
		d.wg.Add(1)
		go d.worker(seg)
	}
}

// acquireLocked hands out an idle segment, or splits the biggest busy one.
func (d *Downloader) acquireLocked() *segment {
	var idle *segment
	for _, s := range d.segs {
		if !s.busy && (s.End < 0 || s.Pos < s.End) {
			if idle == nil || s.Remaining() > idle.Remaining() {
				idle = s
			}
		}
	}
	if idle != nil {
		idle.busy = true
		return idle
	}
	if !d.o.Resumable || d.o.Size <= 0 {
		return nil
	}
	var big *segment
	for _, s := range d.segs {
		if s.busy && (big == nil || s.Remaining() > big.Remaining()) {
			big = s
		}
	}
	// Only split when both halves stay worthwhile and the in-flight buffer
	// of the current owner can't cross the split point.
	if big == nil || big.Remaining() < 2*d.o.MinSegment || big.Remaining() < 4*bufSize {
		return nil
	}
	mid := big.Pos + big.Remaining()/2
	mid -= mid % 4096
	if mid <= big.Pos+bufSize {
		return nil
	}
	ns := &segment{Segment: Segment{Start: mid, End: big.End, Pos: mid}, busy: true}
	big.End = mid
	d.segs = append(d.segs, ns)
	return ns
}

func (d *Downloader) worker(seg *segment) {
	defer d.wg.Done()
	for seg != nil {
		err := d.fetchWithRetry(seg)
		d.mu.Lock()
		seg.busy = false
		if errors.Is(err, errScaledDown) {
			// fetch already released our slot.
			d.mu.Unlock()
			return
		}
		if err != nil {
			d.active--
			d.mu.Unlock()
			if !errors.Is(err, context.Canceled) {
				d.fail(err)
			}
			return
		}
		d.compactLocked()
		if d.active > d.target {
			d.active--
			d.mu.Unlock()
			return
		}
		seg = d.acquireLocked()
		if seg == nil {
			d.active--
		}
		d.mu.Unlock()
	}
}

// compactLocked forgets finished segments; Snapshot reconstructs them.
func (d *Downloader) compactLocked() {
	if !d.o.Resumable || d.o.Size <= 0 {
		return
	}
	kept := d.segs[:0]
	for _, s := range d.segs {
		if s.busy || s.Pos < s.End {
			kept = append(kept, s)
		}
	}
	d.segs = kept
}

var errScaledDown = errors.New("connection count reduced")

func (d *Downloader) fetchWithRetry(seg *segment) error {
	failures := 0
	for {
		before := d.written.Load()
		err := d.fetch(seg)
		if err == nil || errors.Is(err, errScaledDown) {
			return err
		}
		if d.ctx.Err() != nil {
			return context.Canceled
		}
		if errors.Is(err, ErrRangeIgnored) {
			return err
		}
		var he *HTTPError
		if errors.As(err, &he) && !he.Temporary() {
			return err
		}
		if d.written.Load() > before {
			failures = 0 // made progress; this is a fresh streak
		}
		failures++
		if failures > d.o.MaxRetries {
			return fmt.Errorf("giving up after %d retries: %w", d.o.MaxRetries, err)
		}
		// Exponential backoff with jitter, capped at 20s.
		back := time.Duration(1<<min(failures, 5)) * 300 * time.Millisecond
		back += time.Duration(rand.Int63n(int64(back / 2)))
		if back > 20*time.Second {
			back = 20 * time.Second
		}
		select {
		case <-time.After(back):
		case <-d.ctx.Done():
			return context.Canceled
		}
	}
}

func (d *Downloader) fetch(seg *segment) error {
	d.mu.Lock()
	pos, end := seg.Pos, seg.End
	d.mu.Unlock()
	if end >= 0 && pos >= end {
		return nil
	}

	reqCtx, cancel := context.WithCancel(d.ctx)
	defer cancel()
	req, err := d.o.Request.build(reqCtx, http.MethodGet, "")
	if err != nil {
		return err
	}
	ranged := d.o.Resumable && d.o.Size > 0
	if ranged {
		// Ask for everything to the original end; we stop early if the
		// segment gets split while we read.
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", pos, end-1))
	} else if pos > 0 {
		pos = 0
		d.mu.Lock()
		seg.Pos = 0
		d.mu.Unlock()
		d.written.Store(0)
	}

	d.conns.Add(1)
	defer d.conns.Add(-1)

	resp, err := d.o.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return &HTTPError{Status: resp.StatusCode, URL: d.o.Request.URL}
	}
	if ranged && resp.StatusCode != http.StatusPartialContent {
		if pos != 0 {
			return ErrRangeIgnored
		}
	}

	// Stall watchdog: cancel the request if no bytes arrive for a while.
	stall := time.AfterFunc(d.o.StallTimeout, cancel)
	defer stall.Stop()

	body := ratelimit.Reader(reqCtx, resp.Body, d.o.Limiters...)
	buf := make([]byte, bufSize)
	for {
		n, rerr := io.ReadFull(body, buf)
		if n > 0 {
			stall.Reset(d.o.StallTimeout)
			d.mu.Lock()
			at := seg.Pos
			limit := int64(n)
			if seg.End >= 0 && at+limit > seg.End {
				limit = seg.End - at
			}
			seg.Pos += limit // reserve before writing; see acquireLocked
			d.mu.Unlock()
			if limit > 0 {
				if _, werr := d.file.WriteAt(buf[:limit], at); werr != nil {
					d.mu.Lock()
					seg.Pos = at
					d.mu.Unlock()
					d.fail(fmt.Errorf("write: %w", werr))
					return context.Canceled
				}
				d.written.Add(limit)
			}
			d.mu.Lock()
			finished := seg.End >= 0 && seg.Pos >= seg.End
			scaled := !finished && d.active > d.target
			if scaled {
				d.active-- // leave now; the segment goes back to the pool
			}
			d.mu.Unlock()
			if finished {
				return nil
			}
			if scaled {
				return errScaledDown
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) || errors.Is(rerr, io.ErrUnexpectedEOF) {
				d.mu.Lock()
				defer d.mu.Unlock()
				if seg.End < 0 {
					seg.End = seg.Pos // size learned at EOF
					return nil
				}
				if seg.Pos >= seg.End {
					return nil
				}
				return io.ErrUnexpectedEOF
			}
			if reqCtx.Err() != nil && d.ctx.Err() == nil {
				return errors.New("connection stalled")
			}
			return rerr
		}
	}
}
