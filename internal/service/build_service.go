package service

import (
	"W-dev/internal/projectconfig"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
)

// LogWriter adalah io.Writer yang menangkap output per baris
// dan mengirimkannya ke channel + buffer internal.
type LogWriter struct {
	mu      sync.Mutex
	buf     bytes.Buffer // sisa baris yang belum lengkap
	full    bytes.Buffer // seluruh log yang sudah lengkap (dibatasi maxSize)
	ch      chan<- string
	maxSize int
}

func NewLogWriter(ch chan<- string, maxSize int) *LogWriter {
	return &LogWriter{ch: ch, maxSize: maxSize}
}

// Write diimplementasikan sebagai io.Writer.
// pack CLI mengirim output per baris; kita flush setiap newline.
func (w *LogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf.Write(p)

	// Flush baris yang sudah lengkap
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			// Belum ada newline, taruh kembali
			w.buf.WriteString(line)
			break
		}
		trimmed := line
		if len(trimmed) > 0 && trimmed[len(trimmed)-1] == '\n' {
			trimmed = trimmed[:len(trimmed)-1]
		}
		// Trim \r (progress bar pack)
		trimmed = trimCR(trimmed)

		w.full.WriteString(trimmed)
		w.full.WriteByte('\n')
		if w.maxSize > 0 && w.full.Len() > w.maxSize {
			// Pertahankan ekor log (informasi error biasanya ada di akhir).
			tail := append([]byte(nil), w.full.Bytes()[w.full.Len()-w.maxSize/2:]...)
			w.full.Reset()
			w.full.Write(tail)
		}

		if w.ch != nil {
			select {
			case w.ch <- trimmed:
			default:
				// Channel penuh; drop log agar tidak blocking build
			}
		}
	}

	// Batasi ukuran buffer untuk mencegah OOM
	if w.maxSize > 0 && w.buf.Len() > w.maxSize {
		w.buf.Reset()
	}

	return len(p), nil
}

// String mengembalikan seluruh log yang terkumpul.
func (w *LogWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.full.String() + w.buf.String()
}

func trimCR(s string) string {
	// Hapus semua \r (dari progress bar pack)
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\r' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

// BuildByStrategy memilih dan menjalankan strategi build sesuai ProjectType.
func (s *BuildService) BuildByStrategy(ctx context.Context, in BuildInput) (*BuildPlanResult, error) {
	if in.Config == nil {
		return nil, errors.New("BuildByStrategy: resolved config wajib ada")
	}

	var strategy BuildStrategy
	switch in.Config.ProjectType {
	case projectconfig.TypeStatic:
		strategy = &StaticStrategy{}
	case projectconfig.TypeSPA:
		strategy = &SPAStrategy{}
	case projectconfig.TypeNode:
		strategy = &NodeStrategy{build: s}
	case projectconfig.TypeCustom:
		strategy = NewCustomStrategy(&NodeStrategy{build: s})
	default:
		return nil, fmt.Errorf("project type tidak didukung: %s", in.Config.ProjectType)
	}

	log.Printf("[build] strategy=%s image=%s", strategy.Name(), in.ImageName)
	return strategy.Build(ctx, in)
}
