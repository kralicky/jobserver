package auth

import (
	"context"

	"github.com/kralicky/jobserver/pkg/util"
	"google.golang.org/grpc"
)

type Middleware interface {
	Eval(ctx context.Context) (context.Context, error)
}

func NewMiddleware(authenticator Authenticator) Middleware {
	return &authnMiddleware{authenticator: authenticator}
}

type authnMiddleware struct {
	authenticator Authenticator
}

var _ Middleware = (*authnMiddleware)(nil)

// Eval implements Middleware.
func (m *authnMiddleware) Eval(ctx context.Context) (context.Context, error) {
	user, err := m.authenticator.Authenticate(ctx)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, authnUserKey, user), nil
}

func UnaryServerInterceptor(middlewares []Middleware) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		for _, m := range middlewares {
			var err error
			if ctx, err = m.Eval(ctx); err != nil {
				return nil, err
			}
		}
		return handler(ctx, req)
	}
}

func StreamServerInterceptor(middlewares []Middleware) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		for _, m := range middlewares {
			var err error
			if ctx, err = m.Eval(ctx); err != nil {
				return err
			}
		}
		return handler(srv, util.ServerStreamWithContext(ctx, ss))
	}
}
