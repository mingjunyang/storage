package storage

import (
	"bytes"
	"errors"
	"fmt"
	"hash"
	"io"

	"context"
)

// HashFS creates a content addressable filesystem using hash.Hash
// to sum the content and store it using that name.
func HashFS(h hash.Hash, fs FS, gs GetSetter) FS {
	return &hashFS{
		h:  h,
		fs: fs,
		gs: gs,
	}
}

type hashFS struct {
	h  hash.Hash
	fs FS
	gs GetSetter
}

// GetSetter implements a key-value store which is concurrency safe (can
// be used in multiple go-routines concurrently).
type GetSetter interface {
	Get(key string) (string, error)
	Set(key string, value string) error
	Delete(key string) error
}

// Open implements FS.
func (hfs hashFS) Open(ctx context.Context, path string) (*File, error) {
	v, err := hfs.gs.Get(path)
	if err != nil {
		return nil, err
	}
	return hfs.fs.Open(ctx, v)
}

// Walk implements Walker.
func (hfs hashFS) Walk(ctx context.Context, path string, fn WalkFn) error {
	return errors.New("HashFS.Walk is not implemented")
}

type hashWriteCloser struct {
	buf  *bytes.Buffer
	path string
	ctx  context.Context

	hfs hashFS
}

func (w *hashWriteCloser) Write(b []byte) (int, error) {
	n, err := w.buf.Write(b)
	if err != nil {
		return n, err
	}
	w.hfs.h.Write(b) // never returns an error
	return n, nil
}

func (w *hashWriteCloser) Close() (err error) {
	hashPath := fmt.Sprintf("%x", w.hfs.h.Sum(nil))
	if err := w.hfs.gs.Set(w.path, hashPath); err != nil {
		return err
	}

	fsw, err := w.hfs.fs.Create(w.ctx, hashPath)
	if err != nil {
		return err
	}
	defer func() {
		if err1 := fsw.Close(); err == nil {
			err = err1
		}
	}()

	_, err = io.Copy(fsw, w.buf)
	return err
}

// TODO(trent): make sure that you document the FS.Create method
// to Close it, and check the error. If err != nil then the file might not have
// been written.
func (hfs hashFS) Create(ctx context.Context, path string) (io.WriteCloser, error) {
	return &hashWriteCloser{
		buf:  &bytes.Buffer{},
		path: path,
		ctx:  ctx,
		hfs:  hfs,
	}, nil
}

// Delete implements FS.
func (hfs hashFS) Delete(ctx context.Context, path string) error {
	if err := hfs.fs.Delete(ctx, path); err != nil {
		return err
	}
	return hfs.gs.Delete(path)
}
