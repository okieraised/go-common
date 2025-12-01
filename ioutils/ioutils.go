package ioutils

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CopyContext copies from src to dst and stops early if ctx is done.
// Returns bytes written and the first error encountered.
func CopyContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	if ctx == nil {
		return io.Copy(dst, src)
	}
	// Wrap src with a reader that polls ctx between reads.
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		buf := make([]byte, 32*1024)
		for {
			select {
			case <-ctx.Done():
				_ = pw.CloseWithError(ctx.Err())
				return
			default:
			}
			n, err := src.Read(buf)
			if n > 0 {
				if _, err = pw.Write(buf[:n]); err != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					_ = pw.CloseWithError(err)
				}
				return
			}
		}
	}()
	return io.Copy(dst, pr)
}

// CopyWithProgress copies and calls cb(totalBytes) after each chunk. If cb returns an error, copying stops and that error is returned.
func CopyWithProgress(dst io.Writer, src io.Reader, cb func(written int64) error) (int64, error) {
	var written int64
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			written += int64(nw)
			if ew != nil {
				return written, ew
			}
			if nw != nr {
				return written, io.ErrShortWrite
			}
			if cb != nil {
				if err := cb(written); err != nil {
					return written, err
				}
			}
		}
		if er != nil {
			if er == io.EOF {
				return written, nil
			}
			return written, er
		}
	}
}

// ReadAllLimit reads up-to-limit bytes into memory. If the stream exceeds the limit, it returns an error without consuming more than the limit.
func ReadAllLimit(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return io.ReadAll(r)
	}
	var sb strings.Builder
	sb.Grow(int(limit))
	written, err := io.Copy(&sb, &io.LimitedReader{R: r, N: limit})
	if err != nil {
		return nil, err
	}
	// If there is more data, LimitedReader stops at 0 and the next read would
	// return EOF. We detect “truncation” by peeking one extra byte.
	var one [1]byte
	if n, _ := r.Read(one[:]); n > 0 {
		return nil, errors.New("input too large")
	}
	return []byte(sb.String()[:written]), nil
}

// NopWriteCloser wraps an io.Writer to satisfy io.WriteCloser.
type nopWriteCloser struct{ io.Writer }

func (n nopWriteCloser) Close() error { return nil }

func NopWriteCloser(w io.Writer) io.WriteCloser { return nopWriteCloser{w} }

// SafeOpenReader returns os.Stdin if path == "-", else opens path for reading.
func SafeOpenReader(path string) (io.ReadCloser, error) {
	if path == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(path)
}

// SafeOpenWriter returns os.Stdout (wrapped) if path == "-", else creates/truncates the file.
func SafeOpenWriter(path string) (io.WriteCloser, error) {
	if path == "-" {
		return NopWriteCloser(os.Stdout), nil
	}
	return os.Create(path)
}

// EnsureDir makes sure dir exists (mkdir -p).
func EnsureDir(dir string, perm fs.FileMode) error {
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, perm)
}

// WriteFileAtomic writes data to a path atomically by using a temp file + rename.
// It ensures the parent directory exists. Set fsync=true to fsync the temp file.
func WriteFileAtomic(path string, data []byte, perm fs.FileMode, fsync bool) error {
	dir := filepath.Dir(path)
	if err := EnsureDir(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if fsync {
		if err := tmp.Sync(); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// ReplaceFile attempts to atomically replace dst with src via rename.
// If rename fails due to a cross-device link, it falls back to copy+rename.
func ReplaceFile(dst, src string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else {
		// fallback: copy to temp in dst dir then rename
		dir := filepath.Dir(dst)
		tmp, e := os.CreateTemp(dir, "."+filepath.Base(dst)+".tmp-*")
		if e != nil {
			return fmt.Errorf("replace: %w", err)
		}
		in, e := os.Open(src)
		if e != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			return fmt.Errorf("replace open src: %w", e)
		}
		_, e = io.Copy(tmp, in)
		_ = in.Close()
		if e == nil {
			e = tmp.Sync()
		}
		if e1 := tmp.Close(); e == nil {
			e = e1
		}
		if e != nil {
			_ = os.Remove(tmp.Name())
			return e
		}
		if e := os.Rename(tmp.Name(), dst); e != nil {
			_ = os.Remove(tmp.Name())
			return e
		}
		return nil
	}
}

// DirSize walks dir and returns the total size of regular files.
func DirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, e := d.Info()
			if e != nil {
				return e
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// WalkFiles calls fn for each regular file under the root that matches match(path, info).
// If match is nil, all regular files are visited.
func WalkFiles(root string, match func(path string, info fs.FileInfo) bool, fn func(path string, info fs.FileInfo) error) error {
	return filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && (match == nil || match(path, info)) {
			return fn(path, info)
		}
		return nil
	})
}

// AppendFileWithDir appends data to a file, creating the parent dir as needed.
func AppendFileWithDir(path string, data []byte, perm fs.FileMode) error {
	if err := EnsureDir(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, perm)
	if err != nil {
		return err
	}
	_, wErr := f.Write(data)
	if cErr := f.Close(); wErr == nil {
		wErr = cErr
	}
	return wErr
}

// WriteFileSync creates/truncates and fsyncs a file.
func WriteFileSync(path string, data []byte, perm fs.FileMode) error {
	if err := EnsureDir(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// GzipCompress writes a gzip stream (best speed) from r to w.
func GzipCompress(w io.Writer, r io.Reader) error {
	zw, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
	if err != nil {
		return err
	}
	if _, err := io.Copy(zw, r); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

// GzipDecompress writes the decompressed gzip stream from r to w.
func GzipDecompress(w io.Writer, r io.Reader) error {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer zr.Close()
	_, err = io.Copy(w, zr)
	return err
}

// TarDir archives the directory 'dir' into tar stream 'w'. Paths are relative to dir.
func TarDir(w io.Writer, dir string) error {
	tw := tar.NewWriter(w)
	defer tw.Close()
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		hdr, e := tar.FileInfoHeader(info, "")
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(dir, path)
		if e != nil {
			return e
		}
		// Always use forward slashes in tar headers
		hdr.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if d.Type().IsRegular() {
			f, e := os.Open(path)
			if e != nil {
				return e
			}
			_, e = io.Copy(tw, f)
			_ = f.Close()
			return e
		}
		return nil
	})
}

// UntarToDir extracts a tar stream r into directory 'dst'.
func UntarToDir(r io.Reader, dst string) error {
	if err := EnsureDir(dst, 0o755); err != nil {
		return err
	}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(hdr.Name))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := EnsureDir(target, fs.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := EnsureDir(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fs.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			if ce := f.Close(); err == nil {
				err = ce
			}
			if err != nil {
				return err
			}
		default:
			// skip symlinks and others for safety
		}
	}
}

// ReadLines returns all lines from r (without trailing '\n'). For large files, prefer ScanLinesChan.
func ReadLines(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(r)
	// Raise default token limit (64K) to 4MB
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 4*1024*1024)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

// ScanLinesChan streams lines from r on a channel. Close the done channel to stop early.
func ScanLinesChan(r io.Reader, done <-chan struct{}) (<-chan string, <-chan error) {
	out := make(chan string, 128)
	cErr := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(cErr)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for sc.Scan() {
			select {
			case <-done:
				return
			case out <- sc.Text():
			}
		}
		cErr <- sc.Err()
	}()
	return out, cErr
}

type JSONLinesWriter struct {
	w   *bufio.Writer
	enc *json.Encoder
}

func NewJSONLinesWriter(w io.Writer) *JSONLinesWriter {
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	return &JSONLinesWriter{w: bw, enc: enc}
}
func (j *JSONLinesWriter) Write(v any) error {
	if err := j.enc.Encode(v); err != nil {
		return err
	}
	return nil
}
func (j *JSONLinesWriter) Flush() error {
	return j.w.Flush()
}

// JSONLinesReader decodes one JSON value per line.
type JSONLinesReader struct {
	sc *bufio.Scanner
}

func NewJSONLinesReader(r io.Reader) *JSONLinesReader {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	return &JSONLinesReader{sc: sc}
}

func (j *JSONLinesReader) Next(v any) (bool, error) {
	if !j.sc.Scan() {
		return false, j.sc.Err()
	}
	return true, json.Unmarshal(j.sc.Bytes(), v)
}

var (
	ErrLocked = errors.New("ioutils: already locked")
)

// LockFile creates a lock by O_CREATE|O_EXCL on path and returns an Unlock func.
// The lock is cooperative (advisory) and portable; stale locks are possible if a
// process crashes. The file contains the PID and timestamp for debugging.
func LockFile(path string) (unlock func() error, err error) {
	if err := EnsureDir(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if e != nil {
		if os.IsExist(e) {
			return nil, ErrLocked
		}
		return nil, e
	}
	// write PID + TS
	content := fmt.Sprintf("pid=%d time=%s\n", os.Getpid(), time.Now().Format(time.RFC3339Nano))
	_, _ = f.WriteString(content)
	if cerr := f.Close(); cerr != nil {
		_ = os.Remove(path)
		return nil, cerr
	}
	return func() error { return os.Remove(path) }, nil
}

// TryLockFile tries to lock and returns (unlock, locked, err).
func TryLockFile(path string) (func() error, bool, error) {
	unlock, err := LockFile(path)
	if err == nil {
		return unlock, true, nil
	}
	if errors.Is(err, ErrLocked) {
		return nil, false, nil
	}
	return nil, false, err
}

type WriterSeeker struct {
	buf bytes.Buffer
	pos int
}

// Write writings to the buffer of this WriterSeeker instance
func (ws *WriterSeeker) Write(p []byte) (n int, err error) {
	// If the offset is past the end of the buffer, grow the buffer with null bytes.
	if extra := ws.pos - ws.buf.Len(); extra > 0 {
		if _, err := ws.buf.Write(make([]byte, extra)); err != nil {
			return n, err
		}
	}

	// If the offset isn't at the end of the buffer, write as much as we can.
	if ws.pos < ws.buf.Len() {
		n = copy(ws.buf.Bytes()[ws.pos:], p)
		p = p[n:]
	}

	// If there are remaining bytes, append them to the buffer.
	if len(p) > 0 {
		var bn int
		bn, err = ws.buf.Write(p)
		n += bn
	}

	ws.pos += n
	return n, err
}

// Seek seeks in the buffer of this WriterSeeker instance
func (ws *WriterSeeker) Seek(offset int64, whence int) (int64, error) {
	newPos, offs := 0, int(offset)
	switch whence {
	case io.SeekStart:
		newPos = offs
	case io.SeekCurrent:
		newPos = ws.pos + offs
	case io.SeekEnd:
		newPos = ws.buf.Len() + offs
	}
	if newPos < 0 {
		return 0, errors.New("negative result pos")
	}
	ws.pos = newPos
	return int64(newPos), nil
}

// Reader returns an io.Reader. Use it, for example, with io.Copy, to copy the content of the WriterSeeker buffer to an io.Writer
func (ws *WriterSeeker) Reader() io.Reader {
	return bytes.NewReader(ws.buf.Bytes())
}

// Close :
func (ws *WriterSeeker) Close() error {
	return nil
}

// BytesReader returns a *byte.Reader. Use it when you need a reader that implements the io.ReadSeeker interface
func (ws *WriterSeeker) BytesReader() *bytes.Reader {
	return bytes.NewReader(ws.buf.Bytes())
}

func (ws *WriterSeeker) Size() int64 {
	return int64(ws.buf.Len())
}

// TeeReadCloser behaves like io.TeeReader but closes the underlying reader.
type TeeReadCloser struct {
	R io.ReadCloser
	W io.Writer
}

func (t *TeeReadCloser) Read(p []byte) (int, error) {
	n, err := t.R.Read(p)
	if n > 0 {
		if _, ew := t.W.Write(p[:n]); ew != nil && err == nil {
			err = ew
		}
	}
	return n, err
}
func (t *TeeReadCloser) Close() error {
	return t.R.Close()
}
