package auth

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func NewMTLSAuthenticator() Authenticator {
	return &mtlsAuthenticator{}
}

type mtlsAuthenticator struct{}

// Authenticate implements Authenticator.
func (*mtlsAuthenticator) Authenticate(ctx context.Context) (AuthenticatedUser, error) {
	info, ok := peer.FromContext(ctx)
	if !ok {
		return "", status.Errorf(codes.Internal, "no peer info found")
	}
	if info.AuthInfo == nil {
		return "", status.Errorf(codes.Unauthenticated, "no auth info found")
	}
	switch ai := info.AuthInfo.(type) {
	case credentials.TLSInfo:
		var subject string
		for _, vc := range ai.State.VerifiedChains {
			if len(vc) == 0 {
				continue
			}
			leaf := vc[0]
			cn := leaf.Subject.CommonName
			if cn == "" {
				continue
			}
			subject = cn
			break
		}
		if subject == "" {
			return "", status.Errorf(codes.Unauthenticated, "no subject common name found in any verified chains")
		}
		return AuthenticatedUser(subject), nil
	default:
		return "", status.Errorf(codes.Unauthenticated, "unknown auth type: %s", info.AuthInfo.AuthType())
	}
}
