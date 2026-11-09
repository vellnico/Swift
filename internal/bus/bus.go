// Package bus fans out live events (xray status, metrics, journal) to SSE clients.
package bus

import "sync"

type Message struct {
	Type string
	Data any
}

type Bus struct {
	mu     sync.Mutex
	subs   map[chan Message]struct{}
	sticky map[string]Message
}

func New() *Bus {
	return &Bus{subs: map[chan Message]struct{}{}, sticky: map[string]Message{}}
}

// Publish delivers to every subscriber without blocking; slow clients drop messages.
func (b *Bus) Publish(typ string, data any) {
	b.publish(Message{Type: typ, Data: data}, false)
}

// PublishSticky also remembers the message so new subscribers receive it first.
func (b *Bus) PublishSticky(typ string, data any) {
	b.publish(Message{Type: typ, Data: data}, true)
}

func (b *Bus) publish(m Message, sticky bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if sticky {
		b.sticky[m.Type] = m
	}
	for ch := range b.subs {
		select {
		case ch <- m:
		default:
		}
	}
}

// Subscribe returns a channel of messages and a function that unsubscribes.
func (b *Bus) Subscribe() (<-chan Message, func()) {
	ch := make(chan Message, 64)
	b.mu.Lock()
	for _, m := range b.sticky {
		ch <- m
	}
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}
}

// Last returns the latest sticky message of a type.
func (b *Bus) Last(typ string) (Message, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	m, ok := b.sticky[typ]
	return m, ok
}
