package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Logger writes plain (no ANSI) timestamped lines to a file, safe for
// concurrent use by multiple workers.
type Logger struct {
	mu sync.Mutex
	f  *os.File
}

func NewLogger(path string) (*Logger, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating log file: %w", err)
	}
	return &Logger{f: f}, nil
}

func (l *Logger) Line(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.f, "[%s] %s\n", time.Now().Format("15:04:05"), s)
}

func (l *Logger) Close() error {
	if l.f == nil {
		return nil
	}
	return l.f.Close()
}
