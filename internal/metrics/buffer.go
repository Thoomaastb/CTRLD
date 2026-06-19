package metrics

import (
	"sync"
	"time"
)

const (
	// BufferSize: 60 Einträge bei 1s Interval = 60s History
	BufferSize = 60
)

// Buffer ist ein thread-sicherer Rolling-Buffer für Snapshots.
type Buffer struct {
	mu       sync.RWMutex
	data     []*Snapshot
	head     int
	count    int
}

// NewBuffer erstellt einen neuen Rolling-Buffer.
func NewBuffer() *Buffer {
	return &Buffer{
		data: make([]*Snapshot, BufferSize),
	}
}

// Push fügt einen Snapshot in den Buffer ein.
func (b *Buffer) Push(s *Snapshot) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.data[b.head] = s
	b.head = (b.head + 1) % BufferSize
	if b.count < BufferSize {
		b.count++
	}
}

// Latest gibt den neuesten Snapshot zurück.
func (b *Buffer) Latest() *Snapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}
	idx := (b.head - 1 + BufferSize) % BufferSize
	return b.data[idx]
}

// History gibt alle gespeicherten Snapshots in chronologischer Reihenfolge zurück.
func (b *Buffer) History() []*Snapshot {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	result := make([]*Snapshot, b.count)
	start := (b.head - b.count + BufferSize) % BufferSize

	for i := 0; i < b.count; i++ {
		result[i] = b.data[(start+i)%BufferSize]
	}
	return result
}

// Since gibt alle Snapshots nach einem Zeitpunkt zurück.
func (b *Buffer) Since(t time.Time) []*Snapshot {
	all := b.History()
	var result []*Snapshot
	for _, s := range all {
		if s != nil && s.Timestamp.After(t) {
			result = append(result, s)
		}
	}
	return result
}
