package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/diiviikk5/Blister/internal/category"
	"github.com/diiviikk5/Blister/internal/httpdl"
)

// HTTPDriver downloads plain http(s) links with many connections.
type HTTPDriver struct{}

// Run implements Driver.
func (HTTPDriver) Run(ctx context.Context, j *Job) error {
	t := j.Task()
	s := j.Settings()

	probe, err := httpdl.ProbeURL(ctx, j.Client(), t.Request)
	if err != nil {
		return err
	}

	fresh := t.Done == 0 && len(t.Segments) == 0
	// A changed file on the server makes our partial data garbage.
	if !fresh && t.ETag != "" && probe.ETag != "" && t.ETag != probe.ETag {
		fresh = true
		t.Segments = nil
		_ = os.Remove(t.PartPath())
	}
	if !fresh && t.Size > 0 && probe.Size > 0 && t.Size != probe.Size {
		fresh = true
		t.Segments = nil
		_ = os.Remove(t.PartPath())
	}

	j.Update(func(tt *Task) {
		tt.Size = probe.Size
		tt.Resumable = probe.Resumable
		tt.ETag = probe.ETag
		tt.Status = StatusDownloading
		if fresh {
			tt.Segments = nil
			tt.Done = 0
			if !tt.NameFixed && probe.FileName != "" && probe.FileName != tt.Name {
				tt.Name = uniqueName(tt.Dir, probe.FileName, j.m.nameTakenLocked(tt.Dir, tt.ID))
			}
			if c := category.Detect(tt.Name, probe.ContentType); c != category.Other {
				tt.Category = c
			}
		}
		if probe.FinalURL != "" {
			// Keep the original link for display/retry, but download from
			// where it redirected to so each connection skips the hop.
			tt.Request.URL = probe.FinalURL
		}
	})
	t = j.Task()
	j.SetSize(t.Size)

	if err := os.MkdirAll(t.Dir, 0o755); err != nil {
		return err
	}

	conns := t.Connections
	if conns <= 0 {
		conns = s.Connections
	}

	for attempt := 0; ; attempt++ {
		d := httpdl.New(httpdl.Options{
			Client:      j.Client(),
			Request:     t.Request,
			Path:        t.PartPath(),
			Size:        t.Size,
			Resumable:   t.Resumable,
			Connections: conns,
			Segments:    t.Segments,
			Limiters:    j.Limiters(),
			MaxRetries:  s.MaxRetries,
		})
		j.OnSnapshot(d.Snapshot)
		j.OnTune(d.SetConnections)
		j.SetDone(d.Written())

		stop := make(chan struct{})
		go func() {
			tk := time.NewTicker(200 * time.Millisecond)
			defer tk.Stop()
			for {
				select {
				case <-tk.C:
					j.SetDone(d.Written())
					j.SetConns(d.ActiveConnections())
				case <-stop:
					return
				}
			}
		}()
		err := d.Run(ctx)
		close(stop)
		j.SetDone(d.Written())
		j.SetConns(0)
		j.OnTune(nil)
		j.OnSnapshot(nil)

		if errors.Is(err, httpdl.ErrRangeIgnored) && attempt == 0 {
			// The server lied about ranges; fall back to one stream.
			j.Update(func(tt *Task) { tt.Resumable, tt.Segments, tt.Done = false, nil, 0 })
			t = j.Task()
			continue
		}
		if err != nil {
			snap := d.Snapshot()
			j.Update(func(tt *Task) {
				if tt.Resumable {
					tt.Segments = snap
				}
				tt.Done = d.Written()
			})
			return err
		}
		if t.Size < 0 {
			j.SetSize(d.Written())
		}
		break
	}

	return finalize(j)
}

// finalize verifies and moves the .part file into place.
func finalize(j *Job) error {
	t := j.Task()
	part := t.PartPath()
	if t.Checksum != "" {
		ok, err := VerifyFile(part, t.Checksum)
		if err != nil {
			return fmt.Errorf("verify: %w", err)
		}
		j.Update(func(tt *Task) { tt.Verified = &ok })
		if !ok {
			return errors.New("checksum mismatch — the file is corrupt or was changed on the server")
		}
	}
	name := t.Name
	if exists(t.Path()) {
		name = uniqueName(t.Dir, t.Name, nil)
	}
	final := joinPath(t.Dir, name)
	if err := moveFile(part, final); err != nil {
		return err
	}
	if name != t.Name {
		j.Update(func(tt *Task) { tt.Name = name })
	}
	return nil
}
