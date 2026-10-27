// Package fsafe reads files the way a service on a shared host must: never
// following a link and never blocking on a FIFO or device planted where a
// state file belongs.
package fsafe

import (
	"io"
	"os"
	"syscall"
)

// OpenRegular opens path for reading without ever blocking or following a
// link: a FIFO or device named like a transcript must not hang the poll loop,
// and a symlink must not lead it out of the mounted tree.
func OpenRegular(path string) (*os.File, os.FileInfo, error) {
	if st, err := os.Lstat(path); err != nil {
		return nil, nil, err
	} else if !st.Mode().IsRegular() {
		return nil, nil, os.ErrInvalid
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, nil, err
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		f.Close()
		return nil, nil, os.ErrInvalid
	}
	return f, st, nil
}

// readSmall reads a small regular file. It never follows a symlink and never
// blocks: a FIFO, device or link planted where a state file belongs is refused
// (Lstat, then O_NOFOLLOW|O_NONBLOCK and a second regular-file check).
func ReadSmall(path string, limit int64) ([]byte, error) {
	f, st, err := OpenRegular(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if st.Size() > limit {
		return nil, os.ErrInvalid
	}
	b := make([]byte, st.Size())
	n, err := io.ReadFull(f, b)
	if err != nil && err != io.ErrUnexpectedEOF && !(err == io.EOF && st.Size() == 0) {
		return nil, err
	}
	return b[:n], nil
}
