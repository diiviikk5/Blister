package category

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		name, mime string
		want       Category
	}{
		{"movie.MKV", "", Video},
		{"https://x.io/a/song.flac?sig=abc", "", Audio},
		{"setup.exe", "", Program},
		{"backup.part1.rar", "", Archive},
		{"backup.7z.001", "", Archive},
		{"ubuntu.iso", "", Archive},
		{"report", "application/pdf", Document},
		{"blob", "video/mp4; codecs=avc1", Video},
		{"thing", "application/x-bittorrent", Torrent},
		{"noext", "application/octet-stream", Other},
	}
	for _, c := range cases {
		if got := Detect(c.name, c.mime); got != c.want {
			t.Errorf("Detect(%q,%q)=%s want %s", c.name, c.mime, got, c.want)
		}
	}
}
