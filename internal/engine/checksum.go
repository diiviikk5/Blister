package engine

import (
	"crypto/md5"  //nolint:gosec // user-supplied legacy checksums
	"crypto/sha1" //nolint:gosec // user-supplied legacy checksums
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// ParseChecksum accepts "algo:hex" or a bare hex digest (algorithm inferred
// from its length).
func ParseChecksum(s string) (algo, digest string, err error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if i := strings.IndexByte(s, ':'); i > 0 {
		algo, digest = s[:i], s[i+1:]
	} else {
		digest = s
		switch len(s) {
		case 32:
			algo = "md5"
		case 40:
			algo = "sha1"
		case 64:
			algo = "sha256"
		case 128:
			algo = "sha512"
		default:
			return "", "", fmt.Errorf("can't tell the checksum type of a %d-char digest", len(s))
		}
	}
	algo = strings.ReplaceAll(algo, "-", "")
	if _, err := newHash(algo); err != nil {
		return "", "", err
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return "", "", fmt.Errorf("checksum isn't hex: %w", err)
	}
	return algo, digest, nil
}

func newHash(algo string) (hash.Hash, error) {
	switch algo {
	case "md5":
		return md5.New(), nil //nolint:gosec
	case "sha1":
		return sha1.New(), nil //nolint:gosec
	case "sha256":
		return sha256.New(), nil
	case "sha512":
		return sha512.New(), nil
	}
	return nil, fmt.Errorf("unsupported checksum %q (use md5, sha1, sha256 or sha512)", algo)
}

// HashFile computes the digest of path with algo.
func HashFile(path, algo string) (string, error) {
	h, err := newHash(algo)
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.CopyBuffer(h, f, make([]byte, 1<<20)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyFile checks path against an "algo:hex" checksum.
func VerifyFile(path, checksum string) (bool, error) {
	algo, want, err := ParseChecksum(checksum)
	if err != nil {
		return false, err
	}
	got, err := HashFile(path, algo)
	if err != nil {
		return false, err
	}
	return got == want, nil
}
