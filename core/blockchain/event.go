package blockchain

import (
	"fmt"
	"sync"
)

type BlockRecEvent struct {
	BlkHeight uint64
}

type EventFeed[T any] struct {
	subs map[string]chan<- T
	mu   sync.Mutex
}

type EventBus struct {
	BlockFeed *EventFeed[BlockRecEvent]
}

func NewEventFeed[T any]() *EventFeed[T] {
	return &EventFeed[T]{
		subs: make(map[string]chan<- T),
	}
}

func (ef *EventFeed[T]) Subscribe(id string, ch chan<- T) error {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	if _, exists := ef.subs[id]; exists {
		return fmt.Errorf("Subscriber with the id %s already present\n", id)
	}
	ef.subs[id] = ch
	return nil
}

func (ef *EventFeed[T]) UnSubscribe(id string) {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	delete(ef.subs, id)
}

func (ef *EventFeed[T]) Send(event T) {
	ef.mu.Lock()
	defer ef.mu.Unlock()
	for k, ch := range ef.subs {
		fmt.Println(k, len(ch))
		select {
		case ch <- event:
		default:
			log.Warn("Event skipped - event channel full")
		}
	}
}

func NewEventBus() *EventBus {
	return &EventBus{
		BlockFeed: NewEventFeed[BlockRecEvent](),
	}
}
