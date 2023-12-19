package rbac_test

import (
	"context"
	"testing"

	"github.com/kralicky/jobserver/pkg/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

func TestRbac(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RBAC Suite")
}

type testAuthenticator struct {
	user auth.AuthenticatedUser
}

type testServerTransportStream struct {
	grpc.ServerTransportStream

	method string
}

type testUserAssignable auth.AuthenticatedUser

func (t testUserAssignable) AssignedUser() auth.AuthenticatedUser {
	return auth.AuthenticatedUser(t)
}

func (t *testServerTransportStream) Method() string {
	return t.method
}

func (a *testAuthenticator) Authenticate(context.Context) (auth.AuthenticatedUser, error) {
	if a.user == "" {
		panic("bug: testAuthenticator not configured")
	}
	return a.user, nil
}
