package util_test

import (
	"context"

	"github.com/kralicky/jobserver/pkg/util"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

type testServerStream struct {
	grpc.ServerStream
}

type sampleContextKeyType struct{}

var sampleContextKey = sampleContextKeyType{}

var _ = Describe("GRPC Utils", func() {
	Context("SplitFullyQualifiedMethodName", func() {
		When("the method name is fully qualified", func() {
			It("should split a fully qualified method name into service and method names", func() {
				serviceName, methodName, ok := util.SplitFullyQualifiedMethodName("/foo.bar.Example/Test")
				Expect(ok).To(BeTrue())
				Expect(serviceName).To(Equal("foo.bar.Example"))
				Expect(methodName).To(Equal("Test"))
			})
		})
		When("the method name is not fully qualified", func() {
			It("should return false", func() {
				for _, s := range []string{
					"Test",
					"/Test",
					"Test/",
					"/Test/",
					"/",
					"/foo.bar.Example",
					"foo.bar.Example/Test",
				} {
					_, _, ok := util.SplitFullyQualifiedMethodName(s)
					Expect(ok).To(BeFalse(), "expected %q to be not fully qualified", s)
				}
			})
		})
	})

	Context("ServerStreamWithContext", func() {
		When("creating a server stream with a context", func() {
			It("should wrap the underlying stream with the provided context", func() {
				ctx := context.WithValue(context.Background(), sampleContextKey, "sample-value")
				stream := util.ServerStreamWithContext(ctx, testServerStream{})
				Expect(stream.Context()).To(Equal(ctx))
			})
		})
	})
})
