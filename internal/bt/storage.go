package bt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// fileStorage writes torrent data straight into the final files with plain
// file handles. The library's default (mmap) never unmaps when a torrent is
// dropped, which on Windows keeps finished files locked until Blister exits.
// Here every handle belongs to one torrent and is closed when it's dropped.
type fileStorage struct {
	dir string
	pc  storage.PieceCompletion
}

func newStorage(dir string, pc storage.PieceCompletion) storage.ClientImplCloser {
	return &fileStorage{dir: dir, pc: pc}
}

func (s *fileStorage) Close() error { return s.pc.Close() }

type tfile struct {
	path    string
	off     int64
	length  int64
	padding bool
}

type torrentFiles struct {
	files []tfile
	hash  metainfo.Hash
	pc    storage.PieceCompletion

	mu      sync.Mutex
	handles map[int]*os.File
	closed  bool
}

func (s *fileStorage) OpenTorrent(_ context.Context, info *metainfo.Info, ih metainfo.Hash) (storage.TorrentImpl, error) {
	root := filepath.Clean(s.dir)
	name := sanitize(info.BestName())
	t := &torrentFiles{hash: ih, pc: s.pc, handles: map[int]*os.File{}}
	for _, fi := range info.UpvertedFiles() {
		var p string
		if !info.IsDir() {
			p = filepath.Join(root, name)
		} else {
			parts := []string{root, name}
			for _, seg := range fi.BestPath() {
				parts = append(parts, sanitize(seg))
			}
			p = filepath.Join(parts...)
		}
		if rel, err := filepath.Rel(root, p); err != nil || strings.HasPrefix(rel, "..") {
			return storage.TorrentImpl{}, fmt.Errorf("unsafe path in torrent: %q", p)
		}
		tf := tfile{path: p, off: fi.TorrentOffset, length: fi.Length, padding: strings.Contains(fi.Attr, "p")}
		if tf.length == 0 && !tf.padding {
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			if f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
				f.Close()
			}
		}
		t.files = append(t.files, tf)
	}
	return storage.TorrentImpl{
		Piece: func(p metainfo.Piece) storage.PieceImpl { return &piece{t: t, p: p} },
		Close: t.close,
	}, nil
}

// sanitize keeps a torrent-supplied name from escaping the folder or using
// characters Windows rejects.
func sanitize(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		}
		if r < 32 {
			return '_'
		}
		return r
	}, s)
	s = strings.TrimRight(s, ". ")
	if s == "" || s == "." || s == ".." {
		s = "_"
	}
	return s
}

func (t *torrentFiles) close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	var errs []error
	for i, f := range t.handles {
		errs = append(errs, f.Sync(), f.Close())
		delete(t.handles, i)
	}
	return errors.Join(errs...)
}

// handle returns an open handle; create controls whether a missing file is
// created (writes) or reported (reads, so deselected files never appear).
func (t *torrentFiles) handle(i int, create bool) (*os.File, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil, errors.New("torrent storage closed")
	}
	if f, ok := t.handles[i]; ok {
		return f, nil
	}
	flag := os.O_RDWR
	if create {
		flag |= os.O_CREATE
		if err := os.MkdirAll(filepath.Dir(t.files[i].path), 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(t.files[i].path, flag, 0o644)
	if err != nil {
		return nil, err
	}
	t.handles[i] = f
	return f, nil
}

// span calls fn for each file region overlapping [off, off+n).
func (t *torrentFiles) span(off int64, n int, fn func(i int, fileOff int64, lo, hi int) error) error {
	end := off + int64(n)
	for i, f := range t.files {
		fs, fe := f.off, f.off+f.length
		if fe <= off || fs >= end || f.length == 0 {
			continue
		}
		s, e := max(fs, off), min(fe, end)
		if err := fn(i, s-fs, int(s-off), int(e-off)); err != nil {
			return err
		}
	}
	return nil
}

func (t *torrentFiles) readAt(b []byte, off int64) (int, error) {
	n := 0
	err := t.span(off, len(b), func(i int, fo int64, lo, hi int) error {
		if lo != n {
			return io.ErrUnexpectedEOF
		}
		if t.files[i].padding {
			clear(b[lo:hi])
			n = hi
			return nil
		}
		f, err := t.handle(i, false)
		if err != nil {
			return err
		}
		got, err := f.ReadAt(b[lo:hi], fo)
		n += got
		if err != nil {
			if errors.Is(err, io.EOF) {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		return nil
	})
	if err == nil && n < len(b) {
		err = io.EOF
	}
	return n, err
}

func (t *torrentFiles) writeAt(b []byte, off int64) (int, error) {
	n := 0
	err := t.span(off, len(b), func(i int, fo int64, lo, hi int) error {
		if t.files[i].padding {
			n += hi - lo
			return nil
		}
		f, err := t.handle(i, true)
		if err != nil {
			return err
		}
		got, err := f.WriteAt(b[lo:hi], fo)
		n += got
		return err
	})
	return n, err
}

// present reports whether every real file under [off, off+n) exists with at
// least the bytes needed, so stale completion records get rechecked.
func (t *torrentFiles) present(off, n int64) bool {
	ok := true
	_ = t.span(off, int(n), func(i int, fo int64, lo, hi int) error {
		if t.files[i].padding {
			return nil
		}
		st, err := os.Stat(t.files[i].path)
		if err != nil || st.Size() < fo+int64(hi-lo) {
			ok = false
		}
		return nil
	})
	return ok
}

type piece struct {
	t *torrentFiles
	p metainfo.Piece
}

func (p *piece) key() metainfo.PieceKey {
	return metainfo.PieceKey{InfoHash: p.t.hash, Index: p.p.Index()}
}

func (p *piece) ReadAt(b []byte, off int64) (int, error) {
	if left := p.p.Length() - off; left < int64(len(b)) {
		if left <= 0 {
			return 0, io.EOF
		}
		n, err := p.t.readAt(b[:left], p.p.Offset()+off)
		if err == nil {
			err = io.EOF
		}
		return n, err
	}
	return p.t.readAt(b, p.p.Offset()+off)
}

func (p *piece) WriteAt(b []byte, off int64) (int, error) {
	if left := p.p.Length() - off; left < int64(len(b)) {
		b = b[:max(left, 0)]
	}
	return p.t.writeAt(b, p.p.Offset()+off)
}

func (p *piece) MarkComplete() error    { return p.t.pc.Set(p.key(), true) }
func (p *piece) MarkNotComplete() error { return p.t.pc.Set(p.key(), false) }

func (p *piece) Completion() storage.Completion {
	c, err := p.t.pc.Get(p.key())
	if err != nil {
		return storage.Completion{Err: err}
	}
	if c.Ok && c.Complete && !p.t.present(p.p.Offset(), p.p.Length()) {
		// Files were moved or deleted since; download the piece again.
		return storage.Completion{Ok: true, Complete: false}
	}
	return c
}
