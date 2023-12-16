package util_test

import (
	"context"
	"io"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kralicky/jobserver/pkg/util"
)

var _ = Describe("StreamBuffer", func() {
	When("writing to the buffer", func() {
		It("should succeed and not block", func() {
			buf := util.NewStreamBuffer()
			n, err := buf.Write([]byte("hello"))
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(5))

			n, err = buf.Write([]byte("world"))
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(5))
		})
		It("should duplicate writes to all streams", func(ctx SpecContext) {
			buf := util.NewStreamBuffer()
			results := make([][]byte, 100)
			var wg sync.WaitGroup
			wg.Add(100)
			for i := 0; i < 100; i++ {
				i := i
				go func() {
					defer wg.Done()
					reader := buf.NewStream(ctx)
					var buf []byte
					for {
						bytes, ok := <-reader
						if !ok {
							break
						}
						buf = append(buf, bytes...)
					}
					results[i] = buf
				}()
			}
			n, err := buf.Write([]byte("hello"))
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(5))
			n, err = buf.Write([]byte(" "))
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(1))
			n, err = buf.Write([]byte("world"))
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(5))
			buf.Close()
			r := buf.NewStream(ctx)
			wg.Wait()
			for _, result := range results {
				Expect(result).To(Equal([]byte("hello world")))
			}
			Expect(r).To(Receive(Equal([]byte("hello world"))))
			Expect(r).To(BeClosed())
		})
	})
	When("canceling the context of a stream", func() {
		It("should stop receiving data", func(ctx SpecContext) {
			buf := util.NewStreamBuffer()
			reader := buf.NewStream(ctx)
			cctx, ca := context.WithCancel(ctx)
			cancelReader := buf.NewStream(cctx)

			buf.Write([]byte("hello"))
			buf.Write([]byte(" "))
			ca()

			Expect(reader).To(Receive(Equal([]byte("hello"))))
			Expect(reader).To(Receive(Equal([]byte(" "))))

			Expect(cancelReader).To(Receive(Equal([]byte("hello"))))
			Expect(cancelReader).To(Receive(Equal([]byte(" "))))

			Eventually(cancelReader).Should(BeClosed())

			buf.Write([]byte("world"))
			Expect(reader).To(Receive(Equal([]byte("world"))))
			buf.Close()
			Expect(reader).To(BeClosed())
		})
	})
	When("closing the buffer", func() {
		When("writing to the buffer", func() {
			It("should return an error", func() {
				buf := util.NewStreamBuffer()
				Expect(buf.Close()).NotTo(HaveOccurred())

				_, err := buf.Write([]byte("hello"))
				Expect(err).To(MatchError(io.ErrClosedPipe))
			})
		})
		When("creating a new stream after close", func() {
			It("should read the entire buffer at once, and be closed", func(ctx SpecContext) {
				By("creating a stream after closing", func() {
					buf := util.NewStreamBuffer()
					Expect(buf.Write([]byte("hello"))).To(Equal(5))
					Expect(buf.Write([]byte(" "))).To(Equal(1))
					Expect(buf.Write([]byte("world"))).To(Equal(5))

					Expect(buf.Close()).To(Succeed())

					reader := buf.NewStream(ctx)
					Expect(reader).To(Receive(Equal([]byte("hello world"))))
					Expect(reader).To(BeClosed())
				})
				By("creating a stream before closing", func() {
					buf := util.NewStreamBuffer()
					Expect(buf.Write([]byte("hello"))).To(Equal(5))
					Expect(buf.Write([]byte(" "))).To(Equal(1))
					Expect(buf.Write([]byte("world"))).To(Equal(5))

					reader := buf.NewStream(ctx)

					Expect(buf.Close()).To(Succeed())

					Expect(reader).To(Receive(Equal([]byte("hello world"))))
					Expect(reader).To(BeClosed())
				})
			})
			When("reading from existing streams after close", func() {
				It("should read the remaining data, and be closed", func(ctx SpecContext) {
					By("reading half the buffer then closing", func() {
						buf := util.NewStreamBuffer()
						reader := buf.NewStream(ctx)

						Expect(buf.Write([]byte("hello"))).To(Equal(5))
						Expect(buf.Write([]byte(" "))).To(Equal(1))

						Expect(reader).To(Receive(Equal([]byte("hello"))))
						Expect(reader).To(Receive(Equal([]byte(" "))))

						Expect(buf.Write([]byte("world"))).To(Equal(5))

						Expect(buf.Close()).To(Succeed())

						Expect(reader).To(Receive(Equal([]byte("world"))))
						Expect(reader).To(BeClosed())
					})
				})
			})
		})
		When("the buffer is already closed", func() {
			It("should be a no-op", func() {
				buf := util.NewStreamBuffer()
				Expect(buf.Close()).To(Succeed())
				Expect(buf.Close()).To(Succeed())
			})
		})
	})
})
