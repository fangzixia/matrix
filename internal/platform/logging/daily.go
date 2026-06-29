package logging

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type dailyWriter struct {
	logDir        string
	category      string
	retentionDays int
	mu            sync.Mutex
	file          *os.File
	currentDate   string
	now           func() time.Time
}

func newDailyWriter(logDir, category string, retentionDays int) (*dailyWriter, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	w := &dailyWriter{
		logDir:        logDir,
		category:      category,
		retentionDays: retentionDays,
		now:           time.Now,
	}
	if err := os.MkdirAll(filepath.Join(logDir, category), 0o755); err != nil {
		return nil, err
	}
	if err := w.cleanupOld(); err != nil {
		return nil, err
	}
	if err := w.ensureOpen(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *dailyWriter) ensureOpen() error {
	today := w.now().Format("2006-01-02")
	if w.file != nil && w.currentDate == today {
		return nil
	}
	return w.openForDate(today)
}

func (w *dailyWriter) openForDate(date string) error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	dir := filepath.Join(w.logDir, w.category)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, date+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	w.currentDate = date
	return nil
}

func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	today := w.now().Format("2006-01-02")
	if w.currentDate != today {
		if err := w.openForDate(today); err != nil {
			return 0, err
		}
		_ = w.cleanupOldLocked()
	}
	if w.file == nil {
		if err := w.ensureOpen(); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *dailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	w.currentDate = ""
	return err
}

func (w *dailyWriter) cleanupOld() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.cleanupOldLocked()
}

func (w *dailyWriter) cleanupOldLocked() error {
	dir := filepath.Join(w.logDir, w.category)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := w.now().AddDate(0, 0, -w.retentionDays)
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasSuffix(name, ".log") {
			continue
		}
		dateStr := strings.TrimSuffix(name, ".log")
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
	return nil
}

func openDailyWriter(logDir, category string, retentionDays int) (io.WriteCloser, error) {
	return newDailyWriter(logDir, category, retentionDays)
}
