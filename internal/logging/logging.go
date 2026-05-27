package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type EventSink interface {
	Warning(string)
	Error(string)
}

var (
	mu        sync.RWMutex
	eventSink EventSink
)

func LogPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "dune-manager-svc.log")
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "dune-manager-svc.log")
}

func Setup(path string, mirrorStdout bool) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	writers := []io.Writer{f}
	if mirrorStdout {
		writers = append(writers, os.Stdout)
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(writers...))
	return f, nil
}

func SetEventSink(sink EventSink) {
	mu.Lock()
	defer mu.Unlock()
	eventSink = sink
}

func Infof(format string, args ...any) {
	log.Printf("INFO %s", fmt.Sprintf(format, args...))
}

func Warningf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("WARN %s", msg)
	mu.RLock()
	sink := eventSink
	mu.RUnlock()
	if sink != nil {
		sink.Warning(msg)
	}
}

func Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("ERROR %s", msg)
	mu.RLock()
	sink := eventSink
	mu.RUnlock()
	if sink != nil {
		sink.Error(msg)
	}
}
