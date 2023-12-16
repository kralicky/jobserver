package util

import (
	"context"
	"io"
	"slices"
	"sync"
)

// StreamBuffer is an in-memory buffer that can simultaneously be written to
// by a single writer, and read from by multiple readers, such that all readers
// see the same data.
//
// To read from the buffer, call NewStream(). This returns a channel that
// streams a copy of the buffer's contents. If there is already data
// accumulated in the buffer, the channel will receive it immediately, then
// continue to receive any new data written to the buffer until the buffer is
// closed. Each channel returned by NewStream() receives a copy of the buffer's
// full contents, regardless of when it was created.
//
// StreamBuffer implements io.WriteCloser. When Close() is called, all stream
// channels are closed and any subsequent writes to the buffer return
// io.ErrClosedPipe. After the buffer is closed, calls to NewStream return a
// channel that immediately receives a copy of the complete buffer, and is
// then closed.
//
// Care should be taken to ensure that the buffer is always closed when no
// more writes are expected, because the stream channels are expected to be
// read from until they are closed. Leaving the buffer open will cause all
// readers to block indefinitely.
type StreamBuffer struct {
	mu       sync.Mutex
	buf      []byte
	writers  map[int64]chan<- []byte
	writerId int64
	closed   bool
}

var _ io.WriteCloser = (*StreamBuffer)(nil)

func NewStreamBuffer() *StreamBuffer {
	buf := &StreamBuffer{
		buf:     make([]byte, 0, 4096),
		writers: make(map[int64]chan<- []byte),
	}
	return buf
}

func (b *StreamBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return 0, io.ErrClosedPipe
	}
	b.buf = append(b.buf, p...)
	for _, writer := range b.writers {
		select {
		case writer <- p:
		default:
			// TODO: things we could do here:
			// - log the stack trace of the call to NewStream that created this writer
			// - cancel the writer's context
		}
	}
	return len(p), nil
}

func (b *StreamBuffer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}

	b.closed = true
	for _, writer := range b.writers {
		close(writer)
	}
	return nil
}

func (b *StreamBuffer) NewStream(ctx context.Context) <-chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		rc := make(chan []byte, 1)
		rc <- slices.Clone(b.buf)
		close(rc)
		return rc
	}

	b.writerId++
	id := b.writerId

	rc := make(chan []byte, 64)
	if len(b.buf) > 0 {
		rc <- slices.Clone(b.buf)
	}
	b.writers[id] = rc
	context.AfterFunc(ctx, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.closed {
			return
		}
		if c, ok := b.writers[id]; ok {
			delete(b.writers, id)
			close(c)
		}
	})

	return rc
}
