package linkgrab

import (
	"reflect"
	"testing"
)

func TestExpandNumericPadded(t *testing.T) {
	got, err := Expand("https://h/ep[01-03].mkv")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://h/ep01.mkv", "https://h/ep02.mkv", "https://h/ep03.mkv"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestExpandStepAndLetters(t *testing.T) {
	got, _ := Expand("x[0-10:5]")
	if !reflect.DeepEqual(got, []string{"x0", "x5", "x10"}) {
		t.Fatalf("step: %v", got)
	}
	got, _ = Expand("p[c-a]")
	if !reflect.DeepEqual(got, []string{"pc", "pb", "pa"}) {
		t.Fatalf("letters: %v", got)
	}
}

func TestExpandCartesian(t *testing.T) {
	got, _ := Expand("s[1-2]e[1-2]")
	want := []string{"s1e1", "s1e2", "s2e1", "s2e2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestExpandTooMany(t *testing.T) {
	if _, err := Expand("x[0-999999]"); err == nil {
		t.Fatal("expected cap error")
	}
}

func TestExtract(t *testing.T) {
	text := `Grab these: https://a.com/f.zip, and (https://b.org/v.mp4).
magnet:?xt=urn:btih:abc&dn=x also https://a.com/f.zip again
batch https://c.net/img[1-2].png`
	got := Extract(text)
	want := []string{
		"https://a.com/f.zip",
		"https://b.org/v.mp4",
		"magnet:?xt=urn:btih:abc&dn=x",
		"https://c.net/img1.png",
		"https://c.net/img2.png",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}

func TestValid(t *testing.T) {
	for s, want := range map[string]bool{
		"https://x.com/a":          true,
		"ftp://x.com/a":            true,
		"magnet:?xt=urn:btih:abcd": true,
		"file:///etc/passwd":       false,
		"not a url":                false,
	} {
		if Valid(s) != want {
			t.Errorf("Valid(%q) != %v", s, want)
		}
	}
}
