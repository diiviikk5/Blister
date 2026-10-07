package engine

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diiviikk5/Blister/internal/config"
	"github.com/diiviikk5/Blister/internal/httpdl"
	"github.com/diiviikk5/Blister/internal/ratelimit"
)

// Job is a running task as seen by its driver. Drivers report progress
// through the atomic setters; the manager samples them for telemetry.
type Job struct {
	m       *Manager
	id      string
	cancel  context.CancelFunc
	limiter *ratelimit.Limiter

	done     atomic.Int64
	size     atomic.Int64
	conns    atomic.Int32
	seeds    atomic.Int32
	uploaded atomic.Int64

	// Set by the driver so live tuning and saving reach it.
	onTune   func(connections int)
	snapshot func() []httpdl.Segment

	// Sampled under m.mu.
	lastDone, lastUp int64
	lastAt           time.Time
	speed, upSpeed   int64

	pausing      bool
	shuttingDown bool
	stopOnce     sync.Once
	finished     chan struct{}
}

func (j *Job) stop() { j.stopOnce.Do(j.cancel) }

// ID of the task.
func (j *Job) ID() string { return j.id }

// Task returns a copy of the task's current state.
func (j *Job) Task() Task {
	t, _ := j.m.Get(j.id)
	return t
}

// Update mutates the stored task and pushes it to the UI.
func (j *Job) Update(fn func(t *Task)) {
	j.m.mu.Lock()
	t, ok := j.m.tasks[j.id]
	if !ok {
		j.m.mu.Unlock()
		return
	}
	fn(t)
	cp := *t
	j.m.mu.Unlock()
	j.m.o.Emit(EvUpdated, cp)
	j.m.save()
}

// SetStatus is a shorthand for Update that only changes the status.
func (j *Job) SetStatus(s Status) {
	j.Update(func(t *Task) { t.Status = s })
	j.m.poke()
}

// SetDone records bytes completed.
func (j *Job) SetDone(n int64) { j.done.Store(n) }

// AddDone adds to bytes completed.
func (j *Job) AddDone(n int64) { j.done.Add(n) }

// SetSize records the total size (-1 unknown).
func (j *Job) SetSize(n int64) { j.size.Store(n) }

// SetConns records live connection / peer count.
func (j *Job) SetConns(n int) { j.conns.Store(int32(n)) }

// SetSeeds records connected seeders (torrents).
func (j *Job) SetSeeds(n int) { j.seeds.Store(int32(n)) }

// SetUploaded records total bytes uploaded (torrents).
func (j *Job) SetUploaded(n int64) { j.uploaded.Store(n) }

// OnTune registers a callback for live connection changes.
func (j *Job) OnTune(fn func(int)) {
	j.m.mu.Lock()
	j.onTune = fn
	j.m.mu.Unlock()
}

// OnSnapshot registers a provider of resumable segment state.
func (j *Job) OnSnapshot(fn func() []httpdl.Segment) {
	j.m.mu.Lock()
	j.snapshot = fn
	j.m.mu.Unlock()
}

// Limiters returns the global and per-task limiters, in that order.
func (j *Job) Limiters() []*ratelimit.Limiter {
	return []*ratelimit.Limiter{j.m.global, j.limiter}
}

// Client is the shared HTTP client.
func (j *Job) Client() *http.Client { return j.m.Client() }

// Settings returns current settings.
func (j *Job) Settings() config.Settings { return j.m.o.Settings() }

// TakenName reports whether another unfinished task already uses name in dir.
func (j *Job) TakenName(dir string) func(string) bool {
	j.m.mu.Lock()
	defer j.m.mu.Unlock()
	f := j.m.nameTakenLocked(dir, j.id)
	return func(n string) bool {
		j.m.mu.Lock()
		defer j.m.mu.Unlock()
		return f(n)
	}
}

// sample updates smoothed speeds; called with m.mu held.
func (j *Job) sample(now time.Time) {
	done, up := j.done.Load(), j.uploaded.Load()
	if j.lastAt.IsZero() {
		j.lastAt, j.lastDone, j.lastUp = now, done, up
		return
	}
	dt := now.Sub(j.lastAt).Seconds()
	if dt <= 0 {
		return
	}
	inst := float64(done-j.lastDone) / dt
	upInst := float64(up-j.lastUp) / dt
	if inst < 0 {
		inst = 0
	}
	if upInst < 0 {
		upInst = 0
	}
	j.speed = ewma(j.speed, inst)
	j.upSpeed = ewma(j.upSpeed, upInst)
	j.lastAt, j.lastDone, j.lastUp = now, done, up
}

// ewma smooths speed so the UI number doesn't jitter, while still reacting
// within a couple of seconds. Large drops snap down faster.
func ewma(prev int64, inst float64) int64 {
	if prev == 0 {
		return int64(inst)
	}
	alpha := 0.35
	if inst < float64(prev)*0.25 {
		alpha = 0.6
	}
	return int64(alpha*inst + (1-alpha)*float64(prev))
}
