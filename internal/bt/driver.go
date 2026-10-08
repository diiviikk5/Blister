package bt

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/types"

	"github.com/diiviikk5/Blister/internal/engine"
)

// Driver runs torrent tasks. The client is created lazily so users who never
// touch torrents don't pay for a listening socket and DHT.
type Driver struct {
	New func() (*Client, error)

	once   sync.Once
	client *Client
	err    error
}

// Client returns the shared client, starting it on first use.
func (d *Driver) Client() (*Client, error) {
	d.once.Do(func() { d.client, d.err = d.New() })
	return d.client, d.err
}

// Close stops the client if it was started.
func (d *Driver) Close() {
	if d.client != nil {
		d.client.Close()
	}
}

// pieceCells is how many cells the UI piece map gets.
const pieceCells = 320

// Run implements engine.Driver.
func (d *Driver) Run(ctx context.Context, j *engine.Job) error {
	c, err := d.Client()
	if err != nil {
		return err
	}
	task := j.Task()
	t, err := c.Add(ctx, task.URL, task.Dir, j.Client())
	if err != nil {
		return err
	}
	defer t.Drop()

	j.Update(func(tt *engine.Task) {
		if tt.Torrent == nil {
			tt.Torrent = &engine.TorrentInfo{}
		}
		tt.Torrent.InfoHash = t.InfoHash().HexString()
	})

	// Metadata: instant for .torrent files, needs peers for magnets.
	select {
	case <-t.GotInfo():
	case <-ctx.Done():
		return ctx.Err()
	}

	files := t.Files()
	prev := j.Task()
	selected := make([]bool, len(files))
	for i := range files {
		selected[i] = true
		if prev.Torrent != nil && len(prev.Torrent.Files) == len(files) {
			selected[i] = prev.Torrent.Files[i].Selected
		}
	}
	apply := func(sel []bool) {
		for i, f := range files {
			if sel[i] {
				f.SetPriority(types.PiecePriorityNormal)
			} else {
				f.SetPriority(types.PiecePriorityNone)
			}
		}
	}
	var selMu sync.Mutex
	apply(selected)
	j.OnFiles(func(sel []bool) {
		if len(sel) != len(files) {
			return
		}
		selMu.Lock()
		copy(selected, sel)
		selMu.Unlock()
		apply(sel)
	})
	defer j.OnFiles(nil)

	snapshotFiles := func() ([]engine.TorrentFile, int64, int64) {
		selMu.Lock()
		defer selMu.Unlock()
		out := make([]engine.TorrentFile, len(files))
		var size, done int64
		for i, f := range files {
			out[i] = engine.TorrentFile{
				Path:     f.DisplayPath(),
				Size:     f.Length(),
				Done:     f.BytesCompleted(),
				Selected: selected[i],
			}
			if selected[i] {
				size += out[i].Size
				done += out[i].Done
			}
		}
		return out, size, done
	}

	tf, size, done := snapshotFiles()
	j.SetSize(size)
	j.SetDone(done)
	j.Update(func(tt *engine.Task) {
		tt.Name = t.Name()
		tt.Size = size
		tt.Status = engine.StatusDownloading
		tt.Resumable = true
		tt.Torrent.Files = tf
		tt.Torrent.Pieces = t.NumPieces()
		tt.Torrent.PieceMap = pieceMap(t)
	})

	seedRatio := j.Settings().SeedRatio
	seedAfter := j.Settings().SeedAfter
	baseUp := uploaded(t)
	seeding := false

	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	slow := 0
	for {
		select {
		case <-ctx.Done():
			if seeding {
				return nil // stopping a seed leaves the task completed
			}
			return ctx.Err()
		case <-tick.C:
		}

		st := t.Stats()
		up := uploaded(t) - baseUp + prev.Uploaded
		j.SetConns(st.ActivePeers)
		j.SetSeeds(st.ConnectedSeeders)
		j.SetUploaded(up)

		tf, size, done = snapshotFiles()
		j.SetSize(size)
		j.SetDone(done)

		// Heavier UI state every ~2s.
		slow++
		if slow%4 == 0 {
			pm := pieceMap(t)
			ratio := 0.0
			if size > 0 {
				ratio = float64(up) / float64(size)
			}
			j.Update(func(tt *engine.Task) {
				tt.Torrent.Files = tf
				tt.Torrent.PieceMap = pm
				tt.Torrent.Ratio = ratio
				tt.Uploaded = up
				tt.Done = done
			})
		}

		complete := size > 0 && done >= size
		if complete && !seeding {
			if !seedAfter || seedRatio <= 0 {
				j.Update(func(tt *engine.Task) {
					tt.Torrent.Files = tf
					tt.Torrent.PieceMap = strings.Repeat("9", len(tt.Torrent.PieceMap))
				})
				return nil
			}
			seeding = true
			j.Update(func(tt *engine.Task) {
				tt.Torrent.Files = tf
				tt.Torrent.PieceMap = pieceMap(t)
				tt.Done = done
			})
			j.SetStatus(engine.StatusSeeding)
		}
		if seeding {
			if !complete {
				// User selected more files while seeding.
				seeding = false
				j.SetStatus(engine.StatusDownloading)
				continue
			}
			if size > 0 && float64(up)/float64(size) >= seedRatio {
				return nil
			}
		}
	}
}

func uploaded(t *torrent.Torrent) int64 {
	st := t.Stats()
	return st.BytesWrittenData.Int64()
}

// pieceMap down-samples piece completion into pieceCells digits, each 0-9
// meaning the fraction of pieces complete in that cell.
func pieceMap(t *torrent.Torrent) string {
	n := t.NumPieces()
	if n == 0 {
		return ""
	}
	cells := pieceCells
	if n < cells {
		cells = n
	}
	have := make([]bool, n)
	idx := 0
	for _, run := range t.PieceStateRuns() {
		for k := 0; k < run.Length && idx < n; k++ {
			have[idx] = run.Complete
			idx++
		}
	}
	var b strings.Builder
	b.Grow(cells)
	for c := 0; c < cells; c++ {
		lo, hi := c*n/cells, (c+1)*n/cells
		if hi <= lo {
			hi = lo + 1
		}
		got := 0
		for p := lo; p < hi; p++ {
			if have[p] {
				got++
			}
		}
		b.WriteByte(byte('0' + got*9/(hi-lo)))
	}
	return b.String()
}
