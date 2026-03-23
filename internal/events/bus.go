package events

import "sync"

// Event is a fan-out message for the dashboard WebSocket hub.
type Event struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// Bus is a simple pub/sub fan-out for dashboard events.
type Bus struct {
	mu   sync.RWMutex
	subs []chan Event
}

// NewBus creates an empty event bus.
func NewBus() *Bus {
	return &Bus{}
}

// Subscribe returns a receive-only channel of events. The caller should drain it or
// the buffer may drop events when full.
func (b *Bus) Subscribe() <-chan Event {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subs = append(b.subs, ch)
	b.mu.Unlock()
	return ch
}

// Publish sends an event to all subscribers. Non-blocking per subscriber: full buffers drop.
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}
