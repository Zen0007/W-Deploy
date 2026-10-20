package worker

import (
	"sync"

	"W-dev/internal/service"
)

// Queue adalah antrean deploy berbasis channel (implementasi service.DeployEnqueuer).
type Queue struct {
	mu     sync.RWMutex
	closed bool
	ch     chan service.DeployJob
}

func NewQueue(buffer int) *Queue {
	if buffer <= 0 {
		buffer = 64
	}
	return &Queue{ch: make(chan service.DeployJob, buffer)}
}

// Enqueue non-blocking: mengembalikan ErrQueueFull jika antrean penuh / ditutup.
func (q *Queue) Enqueue(j service.DeployJob) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.closed {
		return service.ErrQueueFull
	}
	select {
	case q.ch <- j:
		return nil
	default:
		return service.ErrQueueFull
	}
}

// Chan dipakai worker untuk mengonsumsi job.
func (q *Queue) Chan() <-chan service.DeployJob { return q.ch }

// Close menutup antrean; worker keluar setelah job tersisa selesai.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.ch)
	}
}
