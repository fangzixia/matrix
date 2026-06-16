package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type rotateWriter struct {
	path       string
	maxBytes   int64
	maxBackups int
	mu         sync.Mutex
	file       *os.File
	size       int64
}

func newRotateWriter(path string, maxSizeMB, maxBackups int) (*rotateWriter, error) {
	if maxSizeMB <= 0 {
		maxSizeMB = 100
	}
	if maxBackups <= 0 {
		maxBackups = 7
	}
	w := &rotateWriter{
		path:       path,
		maxBytes:   int64(maxSizeMB) * 1024 * 1024,
		maxBackups: maxBackups,
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotateWriter) open() error {
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.file = f
	w.size = info.Size()
	return nil
}

func (w *rotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.maxBytes > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotateWriter) rotate() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	for i := w.maxBackups - 1; i >= 1; i-- {
		src := w.backupPath(i)
		dst := w.backupPath(i + 1)
		if _, err := os.Stat(src); err == nil {
			_ = os.Remove(dst)
			_ = os.Rename(src, dst)
		}
	}
	if _, err := os.Stat(w.path); err == nil {
		_ = os.Rename(w.path, w.backupPath(1))
	}
	w.size = 0
	return w.open()
}

func (w *rotateWriter) backupPath(n int) string {
	return fmt.Sprintf("%s.%d", w.path, n)
}

func (w *rotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func openLogWriter(path string, maxSizeMB, maxBackups int) (io.WriteCloser, error) {
	if maxSizeMB <= 0 && maxBackups <= 0 {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	return newRotateWriter(path, maxSizeMB, maxBackups)
}
