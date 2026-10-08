package rules

import "testing"

func TestMatch(t *testing.T) {
	gh := Rule{Enabled: true, Hosts: "github.com"}
	if !gh.Matches(Item{URL: "https://objects.github.com/x.zip"}) {
		t.Fatal("bare domain should match subdomains")
	}
	if gh.Matches(Item{URL: "https://notgithub.com/x"}) {
		t.Fatal("must not match a different domain with the same suffix")
	}
	glob := Rule{Enabled: true, Hosts: "cdn*.example.org"}
	if !glob.Matches(Item{URL: "https://cdn3.example.org/a"}) {
		t.Fatal("glob host")
	}

	arch := Rule{Enabled: true, Exts: "zip, .7z rar"}
	if !arch.Matches(Item{URL: "https://x/y", Name: "pack.7Z"}) || arch.Matches(Item{Name: "movie.mkv"}) {
		t.Fatal("extension match")
	}
	if !arch.Matches(Item{URL: "https://x/dl/file.rar?token=1"}) {
		t.Fatal("extension from URL when name is empty")
	}

	big := Rule{Enabled: true, MinSize: 1 << 30}
	if big.Matches(Item{Size: -1}) || big.Matches(Item{Size: 1 << 20}) || !big.Matches(Item{Size: 2 << 30}) {
		t.Fatal("size bounds")
	}

	vids := Rule{Enabled: true, Kinds: []string{"media", "hls"}}
	if !vids.Matches(Item{Kind: "media"}) || vids.Matches(Item{Kind: "http"}) {
		t.Fatal("kind")
	}

	off := Rule{Hosts: "github.com"}
	if off.Matches(Item{URL: "https://github.com/x"}) {
		t.Fatal("disabled rules never match")
	}
}

func TestEvaluateMerges(t *testing.T) {
	rs := []Rule{
		{ID: "a", Enabled: true, Exts: "iso", Dir: `D:\ISOs`, Connections: 32, Tags: []string{"linux"}},
		{ID: "b", Enabled: true, Hosts: "slow.example", SpeedLimit: 1 << 20, Paused: true, Tags: []string{"night"}},
		{ID: "c", Enabled: true, Exts: "exe"},
	}
	e := Evaluate(rs, Item{URL: "https://slow.example/arch.iso"})
	if e.Dir != `D:\ISOs` || e.Connections != 32 || e.SpeedLimit != 1<<20 || !e.Paused {
		t.Fatalf("bad effect %+v", e)
	}
	if len(e.Tags) != 2 || len(e.Matched) != 2 {
		t.Fatalf("tags/matched %+v", e)
	}
}

func TestExpand(t *testing.T) {
	got := Expand(`7z x {path} -o{dir}`, map[string]string{"path": `C:\a b\x.7z`, "dir": `C:\a b`})
	if got != `7z x "C:\a b\x.7z" -o"C:\a b"` {
		t.Fatal(got)
	}
}
