package httputils

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ReadAll reads the entire file content into memory.
// Use with caution for large uploads.
func ReadAll(fh *multipart.FileHeader) ([]byte, error) {
	if fh == nil {
		return nil, errors.New("nil file header")
	}
	file, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// ReadChunked calls fn for each chunk read from the file.
// Useful for processing large files without loading into memory.
func ReadChunked(fh *multipart.FileHeader, bufSize int, fn func([]byte) error) error {
	if fh == nil {
		return errors.New("nil file header")
	}
	if bufSize <= 0 {
		bufSize = 32 * 1024
	}
	file, err := fh.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	buf := make([]byte, bufSize)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			if e := fn(buf[:n]); e != nil {
				return e
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// SaveTo saves the file to the given destination path.
// It creates parent directories if needed.
func SaveTo(fh *multipart.FileHeader, dstPath string, perm os.FileMode) error {
	if fh == nil {
		return errors.New("nil file header")
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

// SaveToDir saves the file into dir, using its original filename (sanitized).
func SaveToDir(fh *multipart.FileHeader, dir string, perm os.FileMode) (string, error) {
	if fh == nil {
		return "", errors.New("nil file header")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := SanitizeFilename(fh.Filename)
	dst := filepath.Join(dir, name)
	return dst, SaveTo(fh, dst, perm)
}

// DetectContentType returns the MIME type by sniffing up to the first 512 bytes.
func DetectContentType(fh *multipart.FileHeader) (string, error) {
	if fh == nil {
		return "", errors.New("nil file header")
	}
	file, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	if n == 0 {
		return "", errors.New("empty file")
	}
	return http.DetectContentType(buf[:n]), nil
}

// OpenAndDetect opens the file and also returns its content type (first 512 bytes read).
// The returned io.ReadCloser starts *after* the bytes used for detection.
func OpenAndDetect(fh *multipart.FileHeader) (io.ReadCloser, string, error) {
	if fh == nil {
		return nil, "", errors.New("nil file header")
	}
	file, err := fh.Open()
	if err != nil {
		return nil, "", err
	}

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	ct := http.DetectContentType(buf[:n])

	// Create a reader that replays buf[:n] then the rest of file
	pr, pw := io.Pipe()
	go func() {
		defer file.Close()
		if n > 0 {
			if _, err := pw.Write(buf[:n]); err != nil {
				pw.CloseWithError(err)
				return
			}
		}
		_, err := io.Copy(pw, file)
		pw.CloseWithError(err)
	}()
	return pr, ct, nil
}

// Extension returns the file extension (lowercase, no dot).
func Extension(fh *multipart.FileHeader) string {
	if fh == nil {
		return ""
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fh.Filename), "."))
	return ext
}

// SanitizeFilename removes potentially dangerous path separators and control chars.
func SanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.TrimSpace(name)
	// replace spaces with underscores
	name = strings.ReplaceAll(name, " ", "_")
	// strip control chars
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "" {
		name = "file"
	}
	return name
}

// ValidateMaxSize checks if file size is within maxBytes.
func ValidateMaxSize(fh *multipart.FileHeader, maxBytes int64) error {
	if fh == nil {
		return errors.New("nil file header")
	}
	if fh.Size > maxBytes {
		return fmt.Errorf("file size %d exceeds max %d bytes", fh.Size, maxBytes)
	}
	return nil
}

// ValidateContentType ensures the file's content type is one of the allowed list.
func ValidateContentType(fh *multipart.FileHeader, allowed []string) (string, error) {
	ct, err := DetectContentType(fh)
	if err != nil {
		return "", err
	}
	for _, a := range allowed {
		if strings.EqualFold(ct, a) {
			return ct, nil
		}
	}
	return ct, fmt.Errorf("unsupported content type: %s", ct)
}
