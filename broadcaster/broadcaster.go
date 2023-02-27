package broadcaster

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ankur-anand/quicksilver/proto/gen/v1/quicksilverpb"
)

type receiver struct {
	rb         *ringBuffer
	cancelFunc context.CancelFunc
	// ID of the database streams want to receive update from.
	database string
}

// OneToManyBroadcaster fans out the incoming TransactionLogs to all the downstream Subscriber.
// It holds the TransactionLogs in memory and broadcast the TransactionLogs in batch in order it
// is received.
type OneToManyBroadcaster struct {
	ctx     context.Context
	mu      sync.RWMutex
	closed  bool
	streams map[string]*receiver
}

// NewOneToManyBroadcaster returns an initialized OneToManyBroadcaster
func NewOneToManyBroadcaster(ctx context.Context) *OneToManyBroadcaster {
	return &OneToManyBroadcaster{
		ctx:     ctx,
		mu:      sync.RWMutex{},
		streams: make(map[string]*receiver),
	}
}

// Broadcast publishes the TransactionLogs to all the receiver currently registered.
func (b *OneToManyBroadcaster) Broadcast(log *quicksilverpb.TransactionLogs) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.streams) == 0 {
		return
	}

	for _, st := range b.streams {
		// send heartbeat to all the streams.
		if log.LogKind == quicksilverpb.TransactionLogs_HEARTBEAT {
			st.rb.inBuf <- log
			continue
		}
		if log.LogKind == quicksilverpb.TransactionLogs_OPERATIONAL && strings.EqualFold(st.database, log.Database) {
			st.rb.inBuf <- log
		}
	}
}

// Close all receiver registered with the broadcaster.
func (b *OneToManyBroadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for client, ch := range b.streams {
		delete(b.streams, client)
		ch.cancelFunc()
	}
}

// EvictClient removes the specified receiver from receiving new TransactionLogs messages.
func (b *OneToManyBroadcaster) EvictClient(client string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch, ok := b.streams[client]
	if ok {
		delete(b.streams, client)
		ch.cancelFunc()
	}
}

// NewClientBroadcastReceiver returns a new initialized receiver go channel for the client, where TransactionLogs will be sent.
func (b *OneToManyBroadcaster) NewClientBroadcastReceiver(client string, db string) (<-chan []*quicksilverpb.TransactionLogs, error) {
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
	b.streams[client] = &receiver{
		cancelFunc: done,
		rb:         rb,
		database:   db,
	}
	return b.streams[client].rb.outBuf, nil
}
