package auth

import (
	"context"
)

type Authenticator interface {
	Authenticate(ctx context.Context) (AuthenticatedUser, error)
}

type authnUserKeyType struct{}

var authnUserKey authnUserKeyType

type AuthenticatedUser string

// Returns the authorized user name from the context. Must only be called from
// within a server handler, where the server is configured with the authz
// interceptors.
func AuthenticatedUserFromContext(ctx context.Context) AuthenticatedUser {
	v, ok := ctx.Value(authnUserKey).(AuthenticatedUser)
	if !ok {
		panic("bug: no authenticated user found in context (mtls middleware not configured)")
	}
	return v
}
