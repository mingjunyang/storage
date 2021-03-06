package storage

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"context"
)

type memFile struct {
	data    []byte
	name    string
	modTime time.Time
}

func (f *memFile) readCloser() io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader(f.data))
}

func (f *memFile) size() int64 {
	return int64(len(f.data))
}

// Mem creates a a basic in-memory implementation of FS.
func Mem() FS {
	return &mem{
		data: make(map[string]*memFile),
	}
}

type mem struct {
	sync.RWMutex

	data map[string]*memFile
}

// Open implements FS.
func (m *mem) Open(_ context.Context, path string) (*File, error) {
	m.RLock()
	f, ok := m.data[path]
	m.RUnlock()

	if ok {
		return &File{
			ReadCloser: f.readCloser(),
			Name:       f.name,
			ModTime:    f.modTime,
			Size:       f.size(),
		}, nil
	}
	return nil, &notExistError{
		Path: path,
	}
}

type writingFile struct {
	*bytes.Buffer
	path string

	m *mem
}

func (wf *writingFile) Close() error {
	wf.m.Lock()
	wf.m.data[wf.path] = &memFile{
		data:    wf.Buffer.Bytes(),
		name:    wf.path,
		modTime: time.Now(),
	}
	wf.m.Unlock()
	return nil
}

// Create implements FS.  NB: Callers must close the io.WriteCloser to create the file.
func (m *mem) Create(_ context.Context, path string) (io.WriteCloser, error) {
	return &writingFile{
		Buffer: &bytes.Buffer{},
		path:   path,
		m:      m,
	}, nil
}

// Delete implements FS.
func (m *mem) Delete(_ context.Context, path string) error {
	m.Lock()
	delete(m.data, path)
	m.Unlock()
	return nil
}

// Walk implements FS.
func (m *mem) Walk(_ context.Context, path string, fn WalkFn) error {
	var list []string
	m.RLock()
	for k := range m.data {
		if strings.HasPrefix(k, path) {
			list = append(list, k)
		}
	}
	m.RUnlock()

	for _, k := range list {
		if err := fn(k); err != nil {
			return err
		}
	}
	return nil
}
