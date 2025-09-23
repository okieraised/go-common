package cryptoutils

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type algo struct {
	name    string
	newHash func() hash.Hash
	hello   string
	empty   string
}

func TestSumHex_BasicVectors(t *testing.T) {
	algs := []algo{
		{
			name:    "SHA256",
			newHash: sha256.New,
			hello:   "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
			empty:   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:    "SHA512",
			newHash: sha512.New,
			hello: "309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f" +
				"989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f",
			empty: "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce" +
				"47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e",
		},
	}

	for _, a := range algs {
		a := a
		t.Run(a.name+"/bytes-empty", func(t *testing.T) {
			if got := SumHexFromBytes(nil, a.newHash); got != a.empty {
				t.Fatalf("empty bytes %s = %s, want %s", a.name, got, a.empty)
			}
		})
		t.Run(a.name+"/bytes-hello", func(t *testing.T) {
			if got := SumHexFromBytes([]byte("hello world"), a.newHash); got != a.hello {
				t.Fatalf("hello bytes %s = %s, want %s", a.name, got, a.hello)
			}
		})
		t.Run(a.name+"/reader-hello", func(t *testing.T) {
			got, err := SumHexFromReader(strings.NewReader("hello world"), a.newHash)
			if err != nil {
				t.Fatalf("reader error: %v", err)
			}
			if got != a.hello {
				t.Fatalf("hello reader %s = %s, want %s", a.name, got, a.hello)
			}
		})
		t.Run(a.name+"/path-hello", func(t *testing.T) {
			dir := t.TempDir()
			fp := filepath.Join(dir, "hello.txt")
			if err := os.WriteFile(fp, []byte("hello world"), 0o600); err != nil {
				t.Fatalf("write temp file: %v", err)
			}
			got, err := SumHexFromPath(fp, a.newHash)
			if err != nil {
				t.Fatalf("path error: %v", err)
			}
			if got != a.hello {
				t.Fatalf("hello path %s = %s, want %s", a.name, got, a.hello)
			}
		})
	}
}

func TestSumHexFromPath_ErrorOnMissingFile(t *testing.T) {
	_, err := SumHexFromPath("no/such/file", sha256.New)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestWrappers_MatchGeneric(t *testing.T) {
	data := bytes.Repeat([]byte{0xAB}, 1024*17+3)
	exp256 := SumHexFromBytes(data, sha256.New)
	exp512 := SumHexFromBytes(data, sha512.New)

	// bytes
	if got := SHA256FromBytes(data); got != exp256 {
		t.Fatalf("SHA256FromBytes mismatch: %s vs %s", got, exp256)
	}
	if got := SHA512FromBytes(data); got != exp512 {
		t.Fatalf("SHA512FromBytes mismatch: %s vs %s", got, exp512)
	}

	// reader
	r := bytes.NewReader(data)
	got256, err := SHA256FromReader(r)
	if err != nil {
		t.Fatalf("SHA256FromReader error: %v", err)
	}
	if got256 != exp256 {
		t.Fatalf("SHA256FromReader mismatch: %s vs %s", got256, exp256)
	}
	r2 := bytes.NewReader(data)
	got512, err := SHA512FromReader(r2)
	if err != nil {
		t.Fatalf("SHA512FromReader error: %v", err)
	}
	if got512 != exp512 {
		t.Fatalf("SHA512FromReader mismatch: %s vs %s", got512, exp512)
	}
}

func TestKnownVector_ManualCheck(t *testing.T) {
	h := sha256.New()
	_, _ = io.WriteString(h, "hello world")
	want := hex.EncodeToString(h.Sum(nil))
	got := SHA256FromBytes([]byte("hello world"))
	if got != want {
		t.Fatalf("manual vector mismatch sha256: got %s want %s", got, want)
	}
}
