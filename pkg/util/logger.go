package util

import (
	"log"
	"os"
)

// Logger sederhana berbasis stdlib dengan prefix level.
type Logger struct {
	l     *log.Logger
	debug bool
}

// NewLogger membuat logger; level "debug" mengaktifkan Debugf.
func NewLogger(level string) *Logger {
	return &Logger{
		l:     log.New(os.Stdout, "", log.LstdFlags),
		debug: level == "debug",
	}
}

func (l *Logger) Infof(format string, args ...any)  { l.l.Printf("[INFO] "+format, args...) }
func (l *Logger) Warnf(format string, args ...any)  { l.l.Printf("[WARN] "+format, args...) }
func (l *Logger) Errorf(format string, args ...any) { l.l.Printf("[ERROR] "+format, args...) }
func (l *Logger) Debugf(format string, args ...any) {
	if l.debug {
		l.l.Printf("[DEBUG] "+format, args...)
	}
}
