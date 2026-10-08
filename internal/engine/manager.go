package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/diiviikk5/Blister/internal/category"
	"github.com/diiviikk5/Blister/internal/config"
	"github.com/diiviikk5/Blister/internal/httpdl"
	"github.com/diiviikk5/Blister/internal/ratelimit"
)

// Driver runs one kind of task until it finishes, fails, or ctx ends.
// Returning nil marks the task completed.
type Driver interface {
	Run(ctx context.Context, j *Job) error
}

// Event names emitted to the UI.
const (
	EvAdded     = "task:added"
	EvUpdated   = "task:updated"
	EvRemoved   = "task:removed"
	EvCompleted = "task:completed"
	EvFailed    = "task:failed"
	EvTick      = "tasks:tick"
)

// Options wires a Manager to the outside world.
type Options struct {
	DataDir  string
	Settings func() config.Settings
	Emit     func(event string, data any)
}

// Manager owns every task and decides which ones run.
type Manager struct {
	o Options

	mu      sync.Mutex
	tasks   map[string]*Task
	order   []string
	jobs    map[string]*Job
	drivers map[Kind]Driver

	global *ratelimit.Limiter
	client *http.Client

	wake    chan struct{}
	saveReq chan struct{}
	quit    chan struct{}
	wg      sync.WaitGroup
	closed  bool
}

// New loads saved tasks and starts the scheduler.
func New(o Options) (*Manager, error) {
	if o.Emit == nil {
		o.Emit = func(string, any) {}
	}
	if o.Settings == nil {
		o.Settings = config.Defaults
	}
	m := &Manager{
		o:       o,
		tasks:   map[string]*Task{},
		jobs:    map[string]*Job{},
		drivers: map[Kind]Driver{},
		global:  ratelimit.New(o.Settings().SpeedLimit),
		wake:    make(chan struct{}, 1),
		saveReq: make(chan struct{}, 1),
		quit:    make(chan struct{}),
	}
	if err := m.rebuildClient(); err != nil {
		return nil, err
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	m.wg.Add(2)
	go m.loop()
	go m.saver()
	m.poke()
	return m, nil
}

// Register installs the driver for a task kind.
func (m *Manager) Register(k Kind, d Driver) {
	m.mu.Lock()
	m.drivers[k] = d
	m.mu.Unlock()
	m.poke()
}

// Client is the shared, tuned HTTP client.
func (m *Manager) Client() *http.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client
}

// GlobalLimiter caps the combined speed of every download.
func (m *Manager) GlobalLimiter() *ratelimit.Limiter { return m.global }

// Settings returns the current settings.
func (m *Manager) Settings() config.Settings { return m.o.Settings() }

// ApplySettings reacts to changed settings.
func (m *Manager) ApplySettings() error {
	m.global.SetRate(m.o.Settings().SpeedLimit)
	err := m.rebuildClient()
	m.poke()
	return err
}

func (m *Manager) rebuildClient() error {
	s := m.o.Settings()
	c, err := httpdl.NewClient(httpdl.ClientOptions{Proxy: s.Proxy, InsecureTLS: s.InsecureTLS})
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.client = c
	m.mu.Unlock()
	return nil
}

// AddRequest describes a new download.
type AddRequest struct {
	URL         string         `json:"url"`
	Kind        Kind           `json:"kind,omitempty"` // auto-detected when empty
	Name        string         `json:"name,omitempty"`
	Dir         string         `json:"dir,omitempty"`
	Connections int            `json:"connections,omitempty"`
	SpeedLimit  int64          `json:"speedLimit,omitempty"`
	Request     httpdl.Request `json:"request,omitempty"`
	Checksum    string         `json:"checksum,omitempty"`
	Paused      bool           `json:"paused,omitempty"`
	Priority    int            `json:"priority,omitempty"`
	AudioOnly   bool           `json:"audioOnly,omitempty"`
	Format      string         `json:"format,omitempty"`
	Size        int64          `json:"size,omitempty"` // known from a prior probe
	Tags        []string       `json:"tags,omitempty"`
}

// Add queues a download and returns it.
func (m *Manager) Add(r AddRequest) (*Task, error) {
	r.URL = strings.TrimSpace(r.URL)
	if r.URL == "" {
		return nil, errors.New("empty link")
	}
	if r.Kind == "" {
		r.Kind = DetectKind(r.URL)
	}
	s := m.o.Settings()
	name := r.Name
	if name == "" {
		name = GuessName(r.URL, r.Kind)
	}
	name = httpdl.Sanitize(name)
	cat := category.Detect(name, "")
	if r.Kind == KindMedia || r.Kind == KindHLS {
		cat = category.Video
		if r.AudioOnly {
			cat = category.Audio
		}
	}
	dir := r.Dir
	if dir == "" {
		dir = s.DownloadDir
		if s.CategorizeFolders && r.Kind != KindTorrent {
			dir = filepath.Join(dir, cat.Folder())
		}
	}
	req := r.Request
	req.URL = r.URL
	if req.UserAgent == "" {
		req.UserAgent = s.UserAgent
	}

	t := &Task{
		ID:          newID(),
		Kind:        r.Kind,
		URL:         r.URL,
		Name:        name,
		Dir:         dir,
		Category:    cat,
		Status:      StatusQueued,
		Size:        -1,
		ETA:         -1,
		Connections: r.Connections,
		SpeedLimit:  r.SpeedLimit,
		Priority:    r.Priority,
		Request:     req,
		Checksum:    strings.TrimSpace(r.Checksum),
		CreatedAt:   time.Now(),
		Tags:        r.Tags,
	}
	if r.Size > 0 {
		t.Size = r.Size
	}
	if r.Paused {
		t.Status = StatusPaused
	}
	t.NameFixed = r.Name != ""
	if r.Kind == KindMedia {
		t.Media = &MediaInfo{Format: r.Format, AudioOnly: r.AudioOnly}
	}
	if r.Kind == KindTorrent {
		t.Torrent = &TorrentInfo{}
	}

	m.mu.Lock()
	if r.Kind == KindHTTP || r.Kind == KindHLS {
		t.Name = uniqueName(dir, t.Name, m.nameTakenLocked(dir, ""))
	}
	m.tasks[t.ID] = t
	m.order = append(m.order, t.ID)
	cp := *t
	m.mu.Unlock()

	m.o.Emit(EvAdded, cp)
	m.save()
	m.poke()
	return &cp, nil
}

// nameTakenLocked reports names already claimed by other tasks in dir.
func (m *Manager) nameTakenLocked(dir, except string) func(string) bool {
	return func(n string) bool {
		for id, t := range m.tasks {
			if id != except && t.Dir == dir && strings.EqualFold(t.Name, n) && t.Status != StatusCompleted {
				return true
			}
		}
		return false
	}
}

// List returns every task in queue order.
func (m *Manager) List() []Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Task, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, *m.tasks[id])
	}
	return out
}

// Get returns one task.
func (m *Manager) Get(id string) (Task, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return Task{}, false
	}
	return *t, true
}

// Pause stops a task, keeping what's downloaded.
func (m *Manager) Pause(id string) error {
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return errNotFound(id)
	}
	j := m.jobs[id]
	if t.Status == StatusCompleted {
		m.mu.Unlock()
		return nil
	}
	if j == nil {
		t.Status = StatusPaused
		t.Speed, t.UpSpeed, t.Conns = 0, 0, 0
		cp := *t
		m.mu.Unlock()
		m.o.Emit(EvUpdated, cp)
		m.save()
		return nil
	}
	j.pausing = true
	m.mu.Unlock()
	j.stop()
	return nil
}

// Resume re-queues a paused or failed task (or a finished torrent to seed).
func (m *Manager) Resume(id string) error {
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return errNotFound(id)
	}
	if _, running := m.jobs[id]; running {
		m.mu.Unlock()
		return nil
	}
	if t.Status == StatusCompleted && t.Kind != KindTorrent {
		m.mu.Unlock()
		return nil
	}
	t.Status = StatusQueued
	t.Error = ""
	cp := *t
	m.mu.Unlock()
	m.o.Emit(EvUpdated, cp)
	m.save()
	m.poke()
	return nil
}

// SetURL points a download at a fresh link (e.g. an expired signed URL) and
// keeps its progress. The driver checks the size on resume and starts over
// only if the new link serves a different file.
func (m *Manager) SetURL(id, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("empty link")
	}
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return errNotFound(id)
	}
	if k := DetectKind(raw); k != t.Kind {
		m.mu.Unlock()
		return fmt.Errorf("that's a %s link, but this download is %s", k, t.Kind)
	}
	_, running := m.jobs[id]
	m.mu.Unlock()
	if running {
		if err := m.stopAndWait(id); err != nil {
			return err
		}
	}
	m.mu.Lock()
	t.URL = raw
	t.Request.URL = raw
	t.ETag = "" // another mirror has its own ETag; size still guards us
	if t.Status != StatusCompleted {
		t.Status = StatusQueued
		t.Error = ""
	}
	cp := *t
	m.mu.Unlock()
	m.o.Emit(EvUpdated, cp)
	m.save()
	m.poke()
	return nil
}

// Restart throws away progress and downloads again from scratch.
func (m *Manager) Restart(id string) error {
	if err := m.stopAndWait(id); err != nil {
		return err
	}
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return errNotFound(id)
	}
	_ = os.Remove(t.PartPath())
	t.Segments, t.Done, t.Error, t.Verified = nil, 0, "", nil
	t.Status = StatusQueued
	t.CompletedAt = time.Time{}
	cp := *t
	m.mu.Unlock()
	m.o.Emit(EvUpdated, cp)
	m.save()
	m.poke()
	return nil
}

// Remove deletes a task, optionally with its files.
func (m *Manager) Remove(id string, deleteFiles bool) error {
	if err := m.stopAndWait(id); err != nil {
		return err
	}
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return errNotFound(id)
	}
	delete(m.tasks, id)
	for i, oid := range m.order {
		if oid == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	cp := *t
	m.mu.Unlock()

	_ = os.Remove(cp.PartPath())
	if deleteFiles && cp.Name != "" {
		_ = os.RemoveAll(cp.Path())
	}
	m.o.Emit(EvRemoved, id)
	m.save()
	m.poke()
	return nil
}

func (m *Manager) stopAndWait(id string) error {
	m.mu.Lock()
	j := m.jobs[id]
	if j != nil {
		j.pausing = true
	}
	m.mu.Unlock()
	if j != nil {
		j.stop()
		<-j.finished
	}
	return nil
}

// PauseAll pauses every unfinished task.
func (m *Manager) PauseAll() {
	for _, t := range m.List() {
		if t.Status != StatusCompleted && t.Status != StatusPaused {
			_ = m.Pause(t.ID)
		}
	}
}

// ResumeAll resumes paused and failed tasks.
func (m *Manager) ResumeAll() {
	for _, t := range m.List() {
		if t.Status == StatusPaused || t.Status == StatusError {
			_ = m.Resume(t.ID)
		}
	}
}

// ClearCompleted forgets finished tasks (files stay on disk).
func (m *Manager) ClearCompleted() {
	for _, t := range m.List() {
		if t.Status == StatusCompleted {
			_ = m.Remove(t.ID, false)
		}
	}
}

// Move reorders a task to index idx in the queue.
func (m *Manager) Move(id string, idx int) error {
	m.mu.Lock()
	cur := -1
	for i, oid := range m.order {
		if oid == id {
			cur = i
			break
		}
	}
	if cur < 0 {
		m.mu.Unlock()
		return errNotFound(id)
	}
	m.order = append(m.order[:cur], m.order[cur+1:]...)
	if idx < 0 {
		idx = 0
	}
	if idx > len(m.order) {
		idx = len(m.order)
	}
	m.order = append(m.order[:idx], append([]string{id}, m.order[idx:]...)...)
	m.mu.Unlock()
	m.save()
	m.poke()
	return nil
}

// Tune changes connections and/or the speed limit of a task, live.
func (m *Manager) Tune(id string, connections int, speedLimit int64) error {
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return errNotFound(id)
	}
	if connections > 0 {
		t.Connections = connections
	}
	if speedLimit >= 0 {
		t.SpeedLimit = speedLimit
	}
	j := m.jobs[id]
	var onTune func(int)
	if j != nil {
		onTune = j.onTune
	}
	cp := *t
	m.mu.Unlock()
	if j != nil {
		j.limiter.SetRate(cp.SpeedLimit)
		if connections > 0 && onTune != nil {
			onTune(connections)
		}
	}
	m.o.Emit(EvUpdated, cp)
	m.save()
	return nil
}

// SelectFiles chooses which files of a torrent to download.
func (m *Manager) SelectFiles(id string, selected []bool) error {
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return errNotFound(id)
	}
	if t.Torrent == nil || len(t.Torrent.Files) != len(selected) {
		m.mu.Unlock()
		return errors.New("file list doesn't match this torrent")
	}
	any := false
	for _, s := range selected {
		any = any || s
	}
	if !any {
		m.mu.Unlock()
		return errors.New("select at least one file")
	}
	files := append([]TorrentFile(nil), t.Torrent.Files...)
	var size int64
	for i := range files {
		files[i].Selected = selected[i]
		if selected[i] {
			size += files[i].Size
		}
	}
	ti := *t.Torrent
	ti.Files = files
	t.Torrent = &ti
	t.Size = size
	var hook func([]bool)
	if j := m.jobs[id]; j != nil {
		hook = j.onFiles
		j.size.Store(size)
	}
	if t.Status == StatusCompleted {
		t.Status = StatusQueued // newly selected files need fetching
	}
	cp := *t
	m.mu.Unlock()
	if hook != nil {
		hook(selected)
	}
	m.o.Emit(EvUpdated, cp)
	m.save()
	m.poke()
	return nil
}

// Rename changes the target file name of a task that hasn't finished.
func (m *Manager) Rename(id, name string) error {
	name = httpdl.Sanitize(name)
	if name == "" {
		return errors.New("empty name")
	}
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return errNotFound(id)
	}
	if _, running := m.jobs[id]; running {
		m.mu.Unlock()
		return errors.New("pause the download before renaming it")
	}
	oldPart, oldPath := t.PartPath(), t.Path()
	t.Name = name
	newPart, newPath := t.PartPath(), t.Path()
	cp := *t
	m.mu.Unlock()
	if exists(oldPart) {
		_ = os.Rename(oldPart, newPart)
	} else if cp.Status == StatusCompleted && exists(oldPath) {
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
	}
	m.o.Emit(EvUpdated, cp)
	m.save()
	return nil
}

// Close pauses running work and persists state.
func (m *Manager) Close() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	jobs := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		j.shuttingDown = true
		jobs = append(jobs, j)
	}
	m.mu.Unlock()
	for _, j := range jobs {
		j.stop()
	}
	for _, j := range jobs {
		select {
		case <-j.finished:
		case <-time.After(5 * time.Second):
		}
	}
	close(m.quit)
	m.wg.Wait()
	_ = m.saveNow()
}

func (m *Manager) poke() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) save() {
	select {
	case m.saveReq <- struct{}{}:
	default:
	}
}

func errNotFound(id string) error { return fmt.Errorf("task %s not found", id) }

// loop runs the scheduler and the telemetry tick.
func (m *Manager) loop() {
	defer m.wg.Done()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	saveEvery := time.NewTicker(5 * time.Second)
	defer saveEvery.Stop()
	for {
		select {
		case <-m.quit:
			return
		case <-m.wake:
			m.schedule()
		case now := <-tick.C:
			m.telemetry(now)
			m.schedule()
		case <-saveEvery.C:
			m.mu.Lock()
			running := len(m.jobs) > 0
			m.mu.Unlock()
			if running {
				m.save()
			}
		}
	}
}

// schedule starts queued tasks while slots are free.
func (m *Manager) schedule() {
	s := m.o.Settings()
	if !InWindow(time.Now(), s.ScheduleStart, s.ScheduleEnd) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	active := 0
	for _, j := range m.jobs {
		if m.tasks[j.id] != nil && m.tasks[j.id].Status != StatusSeeding {
			active++
		}
	}
	var queued []*Task
	for _, id := range m.order {
		if t := m.tasks[id]; t.Status == StatusQueued {
			queued = append(queued, t)
		}
	}
	sort.SliceStable(queued, func(a, b int) bool { return queued[a].Priority < queued[b].Priority })
	for _, t := range queued {
		if active >= s.MaxActive {
			return
		}
		d := m.drivers[t.Kind]
		if d == nil {
			continue
		}
		m.startLocked(t, d)
		active++
	}
}

func (m *Manager) startLocked(t *Task, d Driver) {
	ctx, cancel := context.WithCancel(context.Background())
	j := &Job{
		m:        m,
		id:       t.ID,
		cancel:   cancel,
		limiter:  ratelimit.New(t.SpeedLimit),
		finished: make(chan struct{}),
	}
	j.done.Store(t.Done)
	j.size.Store(t.Size)
	j.lastDone = t.Done
	m.jobs[t.ID] = j
	t.Status = StatusStarting
	t.Error = ""
	if t.StartedAt.IsZero() {
		t.StartedAt = time.Now()
	}
	cp := *t
	go func() {
		m.o.Emit(EvUpdated, cp)
		err := d.Run(ctx, j)
		m.finish(j, err)
	}()
}

func (m *Manager) finish(j *Job, err error) {
	m.mu.Lock()
	delete(m.jobs, j.id)
	t, ok := m.tasks[j.id]
	if !ok {
		m.mu.Unlock()
		close(j.finished)
		return
	}
	t.Done = j.done.Load()
	if sz := j.size.Load(); sz > 0 {
		t.Size = sz
	}
	t.Speed, t.UpSpeed, t.Conns, t.ETA = 0, 0, 0, -1
	ev := EvUpdated
	switch {
	case err == nil:
		t.Status = StatusCompleted
		t.CompletedAt = time.Now()
		t.Segments = nil
		if t.Size > 0 {
			t.Done = t.Size
		}
		ev = EvCompleted
	case j.shuttingDown:
		// Leave it to resume on next launch.
		if t.Status != StatusSeeding {
			t.Status = StatusQueued
		}
	case j.pausing || errors.Is(err, context.Canceled):
		t.Status = StatusPaused
	default:
		t.Status = StatusError
		t.Error = humanError(err)
		ev = EvFailed
	}
	cp := *t
	m.mu.Unlock()
	close(j.finished)
	m.o.Emit(ev, cp)
	if ev != EvUpdated {
		m.o.Emit(EvUpdated, cp)
	}
	m.save()
	m.poke()
}

// TickItem is the compact live telemetry for one running task.
type TickItem struct {
	ID      string `json:"id"`
	Status  Status `json:"status"`
	Done    int64  `json:"done"`
	Size    int64  `json:"size"`
	Speed   int64  `json:"speed"`
	UpSpeed int64  `json:"upSpeed"`
	ETA     int64  `json:"eta"`
	Conns   int    `json:"conns"`
	Seeds   int    `json:"seeds"`
	// Map is file coverage in MapCells digits (0 empty .. 9 full) and Heads
	// are the live connections' positions in [0,1], for the segmented bar.
	Map   string    `json:"map,omitempty"`
	Heads []float64 `json:"heads,omitempty"`
}

// MapCells is the resolution of TickItem.Map.
const MapCells = 120

// coverage renders segments as a MapCells-wide fill map plus the positions
// of the segments still being written.
func coverage(size int64, segs []httpdl.Segment) (string, []float64) {
	if size <= 0 || len(segs) == 0 {
		return "", nil
	}
	cell := float64(size) / MapCells
	fill := make([]float64, MapCells)
	var heads []float64
	for _, s := range segs {
		end := s.End
		if end < 0 || end > size {
			end = size
		}
		done := min(s.Pos, end)
		if s.Pos < end {
			heads = append(heads, float64(int(float64(done)/float64(size)*1000))/1000)
		}
		// Spread [s.Start, done) over the cells it touches.
		for a := s.Start; a < done; {
			c := int(float64(a) / cell)
			if c >= MapCells {
				break
			}
			cEnd := int64(float64(c+1) * cell)
			b := min(done, cEnd)
			if b <= a {
				b = a + 1
			}
			fill[c] += float64(b - a)
			a = b
		}
	}
	out := make([]byte, MapCells)
	for i, f := range fill {
		v := int(f / cell * 9)
		out[i] = byte('0' + max(0, min(9, v)))
	}
	return string(out), heads
}

// Tick is the payload of EvTick.
type Tick struct {
	Items   []TickItem `json:"items"`
	Speed   int64      `json:"speed"`
	UpSpeed int64      `json:"upSpeed"`
	Active  int        `json:"active"`
}

func (m *Manager) telemetry(now time.Time) {
	m.mu.Lock()
	tick := Tick{}
	for id, j := range m.jobs {
		t := m.tasks[id]
		if t == nil {
			continue
		}
		j.sample(now)
		t.Done = j.done.Load()
		if sz := j.size.Load(); sz != 0 {
			t.Size = sz
		}
		t.Speed = j.speed
		t.UpSpeed = j.upSpeed
		t.Conns = int(j.conns.Load())
		t.Seeds = int(j.seeds.Load())
		t.Uploaded = j.uploaded.Load()
		t.ETA = -1
		if t.Speed > 0 && t.Size > 0 {
			t.ETA = (t.Size - t.Done) / t.Speed
		}
		item := TickItem{
			ID: id, Status: t.Status, Done: t.Done, Size: t.Size, Speed: t.Speed,
			UpSpeed: t.UpSpeed, ETA: t.ETA, Conns: t.Conns, Seeds: t.Seeds,
		}
		if j.snapshot != nil {
			item.Map, item.Heads = coverage(t.Size, j.snapshot())
		}
		tick.Items = append(tick.Items, item)
		tick.Speed += t.Speed
		tick.UpSpeed += t.UpSpeed
		if t.Status != StatusSeeding {
			tick.Active++
		}
	}
	m.mu.Unlock()
	sort.Slice(tick.Items, func(a, b int) bool { return tick.Items[a].ID < tick.Items[b].ID })
	m.o.Emit(EvTick, tick)
}

func humanError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no such host"):
		return "Couldn't find the server — check the link or your connection"
	case strings.Contains(msg, "connection refused"):
		return "The server refused the connection"
	case strings.Contains(msg, "There is not enough space on the disk"), strings.Contains(msg, "no space left"):
		return "Not enough disk space"
	case strings.Contains(msg, "certificate"):
		return "The server's security certificate isn't trusted (" + msg + ")"
	}
	return msg
}

// --- persistence ---

func (m *Manager) tasksFile() string {
	return filepath.Join(m.o.DataDir, "tasks.json")
}

type savedState struct {
	Version int     `json:"version"`
	Tasks   []*Task `json:"tasks"`
}

func (m *Manager) load() error {
	if m.o.DataDir == "" {
		return nil
	}
	b, err := os.ReadFile(m.tasksFile())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var st savedState
	if err := json.Unmarshal(b, &st); err != nil {
		_ = os.Rename(m.tasksFile(), m.tasksFile()+".bad")
		return nil
	}
	for _, t := range st.Tasks {
		if t == nil || t.ID == "" {
			continue
		}
		t.Speed, t.UpSpeed, t.Conns, t.Seeds, t.ETA = 0, 0, 0, 0, -1
		switch t.Status {
		case StatusStarting, StatusDownloading:
			t.Status = StatusQueued
		case StatusSeeding:
			t.Status = StatusQueued
		}
		m.tasks[t.ID] = t
		m.order = append(m.order, t.ID)
	}
	return nil
}

func (m *Manager) saver() {
	defer m.wg.Done()
	for {
		select {
		case <-m.quit:
			return
		case <-m.saveReq:
			_ = m.saveNow()
			// Coalesce bursts of changes.
			select {
			case <-time.After(750 * time.Millisecond):
			case <-m.quit:
				return
			}
		}
	}
}

func (m *Manager) saveNow() error {
	if m.o.DataDir == "" {
		return nil
	}
	m.mu.Lock()
	st := savedState{Version: 1}
	for _, id := range m.order {
		t := *m.tasks[id]
		if j := m.jobs[id]; j != nil {
			t.Done = j.done.Load()
			if j.snapshot != nil {
				t.Segments = j.snapshot()
			}
		}
		st.Tasks = append(st.Tasks, &t)
	}
	m.mu.Unlock()
	return config.WriteJSON(m.tasksFile(), st)
}
