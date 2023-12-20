package util

import (
	"container/list"
	"context"
	"io"
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
	chunksMu sync.RWMutex
	chunks   *list.List
	closed   bool

	// A list of channels that the buffer will attempt to write to whenever
	// new data is written to the buffer. This is used to notify stream readers
	// that are reading in real time and are awaiting new data.
	notifiers map[int64]chan<- struct{}
	nextID    int64 // notifier ID counter

	// guards reads and writes to the current chunk
	rtMu sync.RWMutex
}

const maxChunkSize = 4096

var _ io.WriteCloser = (*StreamBuffer)(nil)

func NewStreamBuffer() *StreamBuffer {
	buf := &StreamBuffer{
		chunks:    list.New(),
		notifiers: make(map[int64]chan<- struct{}),
	}
	buf.chunks.PushBack(newChunk())
	return buf
}

func (b *StreamBuffer) Write(p []byte) (n int, err error) {
	b.chunksMu.Lock()
	defer b.chunksMu.Unlock()

	if b.closed {
		return 0, io.ErrClosedPipe
	}
	lenP := len(p)

	// if necessary, split p across multiple chunks
	for len(p) > 0 {
		tail := b.acquireLastChunkLocked()
		chunk := tail.Value.(*chunk)
		b.rtMu.Lock()
		// check if p would fit in the current chunk without overflowing
		remainingSpace := cap(chunk.buf) - len(chunk.buf)
		if len(p) <= remainingSpace {
			chunk.buf = append(chunk.buf, p...)
			p = nil
		} else {
			chunk.buf = append(chunk.buf, p[:remainingSpace]...)
			p = p[remainingSpace:]
		}
		b.rtMu.Unlock()
		for _, nc := range b.notifiers {
			select {
			case nc <- struct{}{}:
			default:
			}
		}
	}

	return lenP, nil
}

func (b *StreamBuffer) Close() error {
	b.chunksMu.Lock()
	defer b.chunksMu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	close(b.chunks.Back().Value.(*chunk).sealed)
	return nil
}

func (b *StreamBuffer) NewStream(ctx context.Context) <-chan []byte {
	b.chunksMu.Lock()
	defer b.chunksMu.Unlock()

	rc := make(chan []byte, 1)

	notifier := make(chan struct{}, 1)
	b.nextID++
	id := b.nextID
	b.notifiers[id] = notifier

	context.AfterFunc(ctx, func() {
		b.chunksMu.Lock()
		defer b.chunksMu.Unlock()
		delete(b.notifiers, id)
		close(notifier)
	})

	go func() {
		defer close(rc)
		for elem := b.firstChunk(); elem != nil; elem = b.nextChunkUnsafe(elem) {
			off := 0
			c := elem.Value.(*chunk)
		CHUNK:
			for {
				data, stat := c.Next(ctx, off, &b.rtMu, notifier)
				if len(data) > 0 {
					rc <- data
					off += len(data)
				}
				switch stat {
				case ReadAgain:
					continue
				case ReadComplete:
					break CHUNK
				case Canceled:
					return
				}
			}
		}
	}()
	return rc
}

func (b *StreamBuffer) firstChunk() *list.Element {
	b.chunksMu.RLock()
	defer b.chunksMu.RUnlock()
	return b.chunks.Front()
}

// NB: this method *does not lock* and imposes the following input constraint:
// For any input 'elem', elem.Next() must always return the same result. It
// can return nil, but only in the case where no more chunks will ever be
// added to the buffer (i.e. the buffer is closed).
//
// The buffer ensures this by pushing a new chunk onto the end of the list
// before sealing the previous chunk, which is the only scenario in which this
// method is called by stream readers.
func (b *StreamBuffer) nextChunkUnsafe(elem *list.Element) *list.Element {
	return elem.Next()
}

func (b *StreamBuffer) acquireLastChunkLocked() *list.Element {
	end := b.chunks.Back()
	if end == nil {
		panic("bug: chunks list is empty")
	}

	endChunk := end.Value.(*chunk)
	if endChunk.Len(&b.rtMu) >= maxChunkSize {
		nc := b.chunks.PushBack(newChunk())
		close(endChunk.sealed)
		return nc
	}
	return end
}

type chunk struct {
	buf    []byte
	sealed chan struct{}
}

func (c *chunk) Len(rtMu *sync.RWMutex) int {
	rtMu.RLock()
	defer rtMu.RUnlock()
	return len(c.buf)
}

type status int

const (
	ReadAgain status = iota
	ReadComplete
	Canceled
)

func (c *chunk) Next(ctx context.Context, offset int, rtMu *sync.RWMutex, rtWait <-chan struct{}) ([]byte, status) {
	select {
	case <-c.sealed:
		return c.nextSealed(offset) // fast path for sealed chunks, no locking required
	default:
		rtMu.RLock()
		switch {
		case offset > len(c.buf):
			panic("bug: offset out of range")
		case offset < len(c.buf):
			defer rtMu.RUnlock()
			return c.buf[offset:], ReadAgain
		default: // offset == len(c.buf)
			rtMu.RUnlock()
			select {
			case <-ctx.Done():
				return nil, Canceled
			case <-c.sealed:
				return c.nextSealed(offset)
			case <-rtWait:
				return nil, ReadAgain
			}
		}
	}
}

func (c *chunk) nextSealed(offset int) ([]byte, status) {
	return c.buf[offset:], ReadComplete
}

func newChunk() *chunk {
	c := &chunk{
		buf:    make([]byte, 0, maxChunkSize),
		sealed: make(chan struct{}),
	}
	return c
}
