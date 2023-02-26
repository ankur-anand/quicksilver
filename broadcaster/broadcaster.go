package broadcaster

import (
	"context"
	"fmt"
	"github.com/ankur-anand/quicksilver/proto/gen/v1/quicksilver"
	"strings"
	"sync"
)

type clientStream struct {
	rb         *ringBuffer
	cancelFunc context.CancelFunc
	// ID of the database streams want to receive update from.
	database string
}

// BroadcastToSubscriber fans out the incoming TransactionLogs to all the downstream Subscriber.
type BroadcastToSubscriber struct {
	ctx     context.Context
	mu      sync.RWMutex
	closed  bool
	streams map[string]*clientStream
}

func NewBroadcaster(ctx context.Context) *BroadcastToSubscriber {
	return &BroadcastToSubscriber{
		ctx:     ctx,
		mu:      sync.RWMutex{},
		streams: make(map[string]*clientStream),
	}
}

// Broadcast publishes the log to all the streams currently registered for the database log.
func (b *BroadcastToSubscriber) Broadcast(log *quicksilver.TransactionLogs) {
	b.mu.RLock()
	if len(b.streams) == 0 {
		b.mu.RUnlock()
		return
	}

	for _, st := range b.streams {
		// send heartbeat to all the streams.
		if log.Action == quicksilver.TransactionLogs_HEARTBEAT {
			st.rb.inBuf <- log
			continue
		}
		if strings.EqualFold(st.database, log.Database) {
			st.rb.inBuf <- log
		}
	}
	b.mu.RUnlock()
}

// Close closes the channels to all subscribers registered with the publisher.
func (b *BroadcastToSubscriber) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	for client, ch := range b.streams {
		delete(b.streams, client)
		ch.cancelFunc()
	}
	b.mu.Unlock()
}

// EvictClient removes the specified client from receiving any more clientStream log messages.
func (b *BroadcastToSubscriber) EvictClient(client string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch, ok := b.streams[client]
	if ok {
		delete(b.streams, client)
		ch.cancelFunc()
	}
}

// NewClientSubscriptionStream returns a new initialized clientStream for the client, where TransactionLogs will be sent.
func (b *BroadcastToSubscriber) NewClientSubscriptionStream(client string, db string) (<-chan []*quicksilver.TransactionLogs, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, fmt.Errorf("error broadcaster")
	}
	_, ok := b.streams[client]
	if ok {
		return nil, fmt.Errorf("duplicate client")
	}
	rb := newRingBuffer()
	ctx, done := context.WithCancel(b.ctx)
	go rb.observeStream(ctx)
	// we create buffered channel
	b.streams[client] = &clientStream{
		cancelFunc: done,
		rb:         rb,
		database:   db,
	}
	return b.streams[client].rb.outBuf, nil
}
