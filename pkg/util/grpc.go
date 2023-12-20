package util

import (
	"context"
	"strings"

	"google.golang.org/grpc"
)

func ServerStreamWithContext(ctx context.Context, stream grpc.ServerStream) grpc.ServerStream {
	return &serverStreamWithContext{
		ServerStream: stream,
		ctx:          ctx,
	}
}

type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

var _ grpc.ServerStream = (*serverStreamWithContext)(nil)

func (s *serverStreamWithContext) Context() context.Context {
	return s.ctx
}

// Split '/pkg.Service/Method' into 'pkg.Service' and 'Method'
func SplitFullyQualifiedMethodName(fqn string) (string, string, bool) {
	parts := strings.Split(fqn, "/")
	if len(parts) != 3 || parts[0] != "" || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}
