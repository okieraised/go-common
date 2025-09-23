package cryptoutils

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"io"
	"os"
)

// SumHexFromReader computes a hex digest using the provided hash constructor.
func SumHexFromReader(r io.Reader, newHash func() hash.Hash) (string, error) {
	h := newHash()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SumHexFromBytes computes a hex digest from a byte slice using the provided hash constructor.
func SumHexFromBytes(b []byte, newHash func() hash.Hash) string {
	h := newHash()
	_, _ = h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// SumHexFromPath computes a hex digest of a file at path using the provided hash constructor.
func SumHexFromPath(path string, newHash func() hash.Hash) (ret string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	h := newHash()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func SHA256FromReader(r io.Reader) (string, error) { return SumHexFromReader(r, sha256.New) }
func SHA256FromBytes(b []byte) string              { return SumHexFromBytes(b, sha256.New) }
func SHA256FromPath(path string) (string, error)   { return SumHexFromPath(path, sha256.New) }

func SHA512FromReader(r io.Reader) (string, error) { return SumHexFromReader(r, sha512.New) }
func SHA512FromBytes(b []byte) string              { return SumHexFromBytes(b, sha512.New) }
func SHA512FromPath(path string) (string, error)   { return SumHexFromPath(path, sha512.New) }
