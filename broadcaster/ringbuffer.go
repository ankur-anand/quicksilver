package broadcaster

import (
	"context"
	"github.com/ankur-anand/quicksilver/proto/gen/v1/quicksilver"
	"time"
)

const (
	bufferLimit  = 100
	batchTimeout = 500 * time.Millisecond
)

// ringBuffer receives transaction log in inBuf and send it to outBuf either when the bufferLimit has been reached
// or batchTimeout is attained. This helps in maintaining the ordering while streaming the log's.
type ringBuffer struct {
	inBuf  chan *quicksilver.TransactionLogs
	outBuf chan []*quicksilver.TransactionLogs
}

func newRingBuffer() *ringBuffer {
	return &ringBuffer{
		inBuf:  make(chan *quicksilver.TransactionLogs, bufferLimit),
		outBuf: make(chan []*quicksilver.TransactionLogs, bufferLimit),
	}
}

func (r *ringBuffer) close() {
	close(r.inBuf)
	close(r.outBuf)
}

func (r *ringBuffer) observeStream(ctx context.Context) {
	logCache := make([]*quicksilver.TransactionLogs, 0, bufferLimit)
	tick := time.NewTimer(batchTimeout)
	for {
		select {
		// Check if a new messages is available.
		// If so, store it and check if the logCache
		// has exceeded its size limit.
		case log := <-r.inBuf:
			logCache = append(logCache, log)

			if len(logCache) < bufferLimit {
				// break the select
				break
			}
			// stop and drain if possible.
			tick.Stop()
			select {
			case <-tick.C:
			default:
			}

			// send the logCache message to outputBuffer and reset the logCache
			r.outBuf <- logCache
			logCache = logCache[:0]
			tick.Reset(batchTimeout)
			// If the timeout is reached, send the
		// current message logCache, regardless of
		// its size.
		case <-tick.C:
			r.outBuf <- logCache
			logCache = logCache[:0]
		case <-ctx.Done():
			r.close()
			return
		}
	}
}
