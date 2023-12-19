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
		It("should read data in the same order it was written", func(ctx SpecContext) {
			buf := util.NewStreamBuffer()
			r1 := buf.NewStream(ctx)
			for i := 0; i < 256; i++ {
				n, err := buf.Write([]byte{byte(i)})
				Expect(err).NotTo(HaveOccurred())
				Expect(n).To(Equal(1))
			}
			r2 := buf.NewStream(ctx)

			buf.Close()

			var buf1 []byte
			var buf2 []byte
			for b := range r1 {
				buf1 = append(buf1, b...)
			}
			for b := range r2 {
				buf2 = append(buf2, b...)
			}
			Expect(buf1).To(Equal(buf2))
			for i := 0; i < 256; i++ {
				Expect(buf1[i]).To(Equal(byte(i)))
			}
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
			var recv []byte
			Eventually(r).Should(Receive(&recv))
			Expect(recv).To(Equal([]byte("hello world")))
			Eventually(r).Should(BeClosed())
		})
	})
	When("canceling the context of a stream", func() {
		It("should stop receiving data", func(ctx SpecContext) {
			buf := util.NewStreamBuffer()
			reader := buf.NewStream(ctx)
			cctx, ca := context.WithCancel(ctx)
			cancelReader := buf.NewStream(cctx)

			buf.Write([]byte("hello "))

			var recv []byte
			Eventually(reader).Should(Receive(&recv))
			Expect(recv).To(Equal([]byte("hello ")))

			Eventually(cancelReader).Should(Receive(&recv))
			Expect(recv).To(Equal([]byte("hello ")))

			ca()

			Eventually(cancelReader).Should(BeClosed())

			buf.Write([]byte("world"))
			Eventually(reader).Should(Receive(&recv))
			Expect(recv).To(Equal([]byte("world")))
			buf.Close()
			Eventually(reader).Should(BeClosed())
		})
	})
	When("closing the buffer", func() {
		When("writing to the buffer", func() {
			It("should return an error", func() {
				buf := util.NewStreamBuffer()
				Expect(buf.Close()).To(Succeed())
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
					var recv []byte
					Eventually(reader).Should(Receive(&recv))
					Expect(recv).To(Equal([]byte("hello world")))
					Eventually(reader).Should(BeClosed())
				})
				By("creating a stream before closing", func() {
					buf := util.NewStreamBuffer()
					Expect(buf.Write([]byte("hello"))).To(Equal(5))
					Expect(buf.Write([]byte(" "))).To(Equal(1))
					Expect(buf.Write([]byte("world"))).To(Equal(5))

					reader := buf.NewStream(ctx)

					Expect(buf.Close()).To(Succeed())

					var recv []byte
					Eventually(reader).Should(Receive(&recv))
					Expect(recv).To(Equal([]byte("hello world")))
					Eventually(reader).Should(BeClosed())
				})
			})
			When("reading from existing streams after close", func() {
				It("should read the remaining data, and be closed", func(ctx SpecContext) {
					By("reading half the buffer then closing", func() {
						buf := util.NewStreamBuffer()
						reader := buf.NewStream(ctx)

						Expect(buf.Write([]byte("hello"))).To(Equal(5))

						var recv []byte
						Eventually(reader).Should(Receive(&recv))
						Expect(recv).To(Equal([]byte("hello")))

						Expect(buf.Write([]byte("world"))).To(Equal(5))

						Expect(buf.Close()).To(Succeed())

						Eventually(reader).Should(Receive(&recv))
						Expect(recv).To(Equal([]byte("world")))
						Eventually(reader).Should(BeClosed())
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
