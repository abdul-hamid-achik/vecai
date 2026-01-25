package llm

import (
	"context"
	"errors"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

// ErrChunkTimeout is returned when no data is received within the timeout period
var ErrChunkTimeout = errors.New("stream chunk timeout: no data received")

// streamResult holds the result from stream.Next() in the background goroutine
type streamResult struct {
	hasNext bool
	event   anthropic.MessageStreamEventUnion
}

// StreamWrapper wraps a blocking SSE stream to make it interruptible via context
type StreamWrapper struct {
	stream       *ssestream.Stream[anthropic.MessageStreamEventUnion]
	ctx          context.Context
	chunkChan    chan streamResult
	chunkTimeout time.Duration
	started      bool
	lastEvent    anthropic.MessageStreamEventUnion
}

// NewStreamWrapper creates a new wrapper around the stream
func NewStreamWrapper(ctx context.Context, stream *ssestream.Stream[anthropic.MessageStreamEventUnion], timeout time.Duration) *StreamWrapper {
	sw := &StreamWrapper{
		stream:       stream,
		ctx:          ctx,
		chunkChan:    make(chan streamResult, 1),
		chunkTimeout: timeout,
	}
	return sw
}

// startReader starts the background goroutine that reads from the stream
func (sw *StreamWrapper) startReader() {
	if sw.started {
		return
	}
	sw.started = true

	go func() {
		for {
			// This is the blocking call - runs in background
			hasNext := sw.stream.Next()

			var event anthropic.MessageStreamEventUnion
			if hasNext {
				event = sw.stream.Current()
			}

			// Try to send the result, but don't block if context is cancelled
			select {
			case sw.chunkChan <- streamResult{hasNext: hasNext, event: event}:
			case <-sw.ctx.Done():
				return
			}

			// If no more data, exit the goroutine
			if !hasNext {
				return
			}
		}
	}()
}

// Next returns the next event from the stream, or an error if cancelled/timed out
// Returns (event, hasMore, error)
func (sw *StreamWrapper) Next() (anthropic.MessageStreamEventUnion, bool, error) {
	sw.startReader()

	select {
	case result := <-sw.chunkChan:
		if result.hasNext {
			sw.lastEvent = result.event
			return result.event, true, nil
		}
		// Stream ended normally
		return anthropic.MessageStreamEventUnion{}, false, sw.stream.Err()

	case <-sw.ctx.Done():
		// Context cancelled (ESC pressed)
		return anthropic.MessageStreamEventUnion{}, false, sw.ctx.Err()

	case <-time.After(sw.chunkTimeout):
		// Timeout - no data for too long
		return anthropic.MessageStreamEventUnion{}, false, ErrChunkTimeout
	}
}

// Err returns any error from the underlying stream
func (sw *StreamWrapper) Err() error {
	return sw.stream.Err()
}
