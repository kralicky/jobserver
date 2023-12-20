package util_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	mathrand "math/rand"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gmeasure"

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
		It("should split large writes into chunks", func(ctx SpecContext) {
			numReaders := 100
			buf := util.NewStreamBuffer()
			readers := make([]<-chan []byte, numReaders)
			for i := 0; i < numReaders; i++ {
				readers[i] = buf.NewStream(ctx)
			}
			actualReadCounts := make([]int64, numReaders)
			bench := gmeasure.NewExperiment("benchmark")
			AddReportEntry(bench.Name, bench)

			var wg sync.WaitGroup
			totalSize := 10 * 1024 * 1024 // 10MB
			wg.Add(numReaders)
			for i := 0; i < numReaders; i++ {
				i := i
				r := readers[i]
				go func() {
					defer wg.Done()
					var buf []byte
					start := time.Now()
					for {
						bytes, ok := <-r
						if !ok {
							break
						}
						buf = append(buf, bytes...)
					}
					duration := time.Since(start)
					bench.RecordValue("per-stream read rate", float64(len(buf))/duration.Seconds()/1024, gmeasure.Units("KiB/s"))
					actualReadCounts[i] = int64(len(buf))
				}()
			}
			written := 0
			start := time.Now()
			for written < totalSize {
				// read a random number of bytes between 1 and 8KB
				n := min(mathrand.Intn(8*1024), totalSize-written)
				data := make([]byte, n)
				Expect(rand.Read(data)).To(Equal(n))
				Expect(buf.Write(data)).To(Equal(n))
				written += n
			}
			bench.RecordValue("buffer write rate", float64(totalSize)/time.Since(start).Seconds()/1024, gmeasure.Units("KiB/s"))
			buf.Close()
			wg.Wait()
			for i := 0; i < numReaders; i++ {
				Expect(actualReadCounts[i]).To(Equal(int64(totalSize)))
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
	Specify("slow readers should not block fast readers", func(ctx SpecContext) {
		buf := util.NewStreamBuffer()

		fastReader := buf.NewStream(ctx)

		slowReader := buf.NewStream(ctx)

		contents := make([]byte, 1024*1024) // 1MB
		Expect(rand.Read(contents)).To(Equal(len(contents)))

		fastreadC := make(chan []byte)
		slowreadC := make(chan []byte)
		go func() {
			var fastRead []byte
			for {
				bytes, ok := <-fastReader
				if !ok {
					break
				}
				fastRead = append(fastRead, bytes...)
			}
			fastreadC <- fastRead
		}()
		go func() {
			var slowRead []byte
			for {
				bytes, ok := <-slowReader
				if !ok {
					break
				}
				slowRead = append(slowRead, bytes...)
				time.Sleep(10 * time.Millisecond)
			}
			slowreadC <- slowRead
		}()

		n, err := buf.Write(contents)
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(contents)))
		buf.Close()

		start := time.Now()
		fastRead := <-fastreadC
		fastTime := time.Since(start)
		fmt.Fprintln(GinkgoWriter, "fast read took", fastTime)

		slowRead := <-slowreadC
		slowTime := time.Since(start)
		fmt.Fprintln(GinkgoWriter, "slow read took", slowTime)
		// 10x is just a sanity check, in actuality it's on the order of 1000x
		// (unless the race detector is enabled or something)
		Expect(fastTime * 10).To(BeNumerically("<", slowTime))

		Expect(fastRead).To(Equal(contents))
		Expect(slowRead).To(Equal(contents))
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
