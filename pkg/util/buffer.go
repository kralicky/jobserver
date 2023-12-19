package util

import (
	"container/list"
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
// accumulated in the buffer, the channel will begin to read it immediately,
// then continue to receive any new data written to the buffer until the buffer
// is closed. Each channel returned by NewStream() receives a copy of the
// buffer's full contents, regardless of when it was created.
//
// StreamBuffer implements io.WriteCloser. When Close() is called, all stream
// channels are closed and any subsequent writes to the buffer return
// io.ErrClosedPipe. After the buffer is closed, calls to NewStream return a
// channel that will receive the full contents of the buffer, then be closed.
//
// Care should be taken to ensure that the buffer is always closed when no
// more writes are expected, because the stream channels are expected to be
// read from until they are closed. Leaving the buffer open will cause all
// readers to block indefinitely.
type StreamBuffer struct {
	mu     sync.Mutex
	chunks *list.List
	closed bool
}

const (
	maxChunkSize = 4 * 1024
)

var _ io.WriteCloser = (*StreamBuffer)(nil)

func NewStreamBuffer() *StreamBuffer {
	buf := &StreamBuffer{
		chunks: list.New(),
	}
	buf.chunks.PushBack(newChunk())
	return buf
}

func (b *StreamBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return 0, io.ErrClosedPipe
	}

	tail := b.acquireLastChunkLocked()
	chunk := tail.Value.(*chunk)
	chunk.Append(p)

	return len(p), nil
}

func (b *StreamBuffer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	b.chunks.Back().Value.(*chunk).Seal()
	return nil
}

func (b *StreamBuffer) NewStream(ctx context.Context) <-chan []byte {
	b.mu.Lock()
	defer b.mu.Unlock()

	mu := sync.Mutex{}
	var done bool
	rc := make(chan []byte)
	closeOnce := sync.OnceFunc(func() {
		mu.Lock()
		defer mu.Unlock()
		done = true
		close(rc)
	})

	go func() {
		defer closeOnce()
		for elem := b.headChunk(); elem != nil; elem = b.nextChunk(elem) {
			off := 0
			c := elem.Value.(*chunk)
			for {
				bytes, more := c.Next(off)
				mu.Lock()
				if done {
					mu.Unlock()
					return
				}
				if len(bytes) > 0 {
					rc <- bytes
					off += len(bytes)
				}
				mu.Unlock()
				if !more {
					break
				}
			}
		}
	}()

	context.AfterFunc(ctx, closeOnce)
	return rc
}

func (b *StreamBuffer) acquireLastChunkLocked() *list.Element {
	end := b.chunks.Back()
	if end == nil {
		return b.chunks.PushBack(newChunk())
	}

	endChunk := end.Value.(*chunk)
	if endChunk.Len() >= maxChunkSize {
		nc := b.chunks.PushBack(newChunk())
		endChunk.Seal() // NB: this must be done after the new chunk is added
		return nc
	}
	return end
}

func (b *StreamBuffer) headChunk() *list.Element {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.chunks.Front()
}

func (b *StreamBuffer) nextChunk(after *list.Element) *list.Element {
	b.mu.Lock()
	defer b.mu.Unlock()
	return after.Next()
}

type chunk struct {
	buf    []byte
	sealed bool
	mu     sync.RWMutex
	rcond  sync.Cond
}

func (c *chunk) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.buf)
}

func (c *chunk) Seal() {
	c.mu.Lock()
	c.sealed = true
	c.mu.Unlock()
	c.rcond.Broadcast()
}

func (c *chunk) Append(bytes []byte) {
	c.mu.Lock()
	c.buf = append(c.buf, bytes...)
	c.mu.Unlock()
	c.rcond.Broadcast()
}

func (c *chunk) Next(offset int) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	switch {
	case offset > len(c.buf):
		panic("bug: offset out of range")
	case offset < len(c.buf):
		return slices.Clone(c.buf[offset:]), !c.sealed
	default: // offset == len(c.buf)
		for !c.sealed {
			c.rcond.Wait()
			if offset < len(c.buf) {
				return slices.Clone(c.buf[offset:]), true
			}
		}
		if offset < len(c.buf) {
			return slices.Clone(c.buf[offset:]), false
		}
		return nil, false
	}
}

func newChunk() *chunk {
	c := &chunk{
		buf: make([]byte, 0, maxChunkSize),
	}
	c.rcond.L = c.mu.RLocker()
	return c
}
