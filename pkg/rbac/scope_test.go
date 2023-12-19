package rbac_test

import (
	"context"

	rbacv1 "github.com/kralicky/jobserver/pkg/apis/rbac/v1"
	"github.com/kralicky/jobserver/pkg/auth"
	"github.com/kralicky/jobserver/pkg/rbac"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ = Describe("Scope", func() {
	var authCtx context.Context
	var scope rbacv1.Scope
	const assignedUser = "assigned-user"
	const clientUser = "client-user"
	sampleItems := []testUserAssignable{
		assignedUser,
		clientUser,
		assignedUser,
		clientUser,
	}
	JustBeforeEach(func() {
		rbacConfig := &rbacv1.Config{
			Roles: []*rbacv1.Roles{
				{
					Id:      "test-role",
					Service: "foo.bar.Example",
					AllowedMethods: []*rbacv1.AllowedMethod{
						{
							Name:  "Test",
							Scope: scope.Enum(),
						},
					},
				},
			},
			RoleBindings: []*rbacv1.RoleBindings{
				{
					Id:     "test-role-binding",
					RoleId: "test-role",
					Users:  []string{assignedUser, clientUser},
				},
			},
		}

		ctx := grpc.NewContextWithServerTransportStream(
			context.Background(),
			&testServerTransportStream{
				method: "/foo.bar.Example/Test",
			},
		)
		var err error
		authCtx, err = auth.NewMiddleware(&testAuthenticator{
			user: clientUser,
		}).Eval(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(auth.AuthenticatedUserFromContext(authCtx)).To(BeEquivalentTo(clientUser))

		authCtx, err = rbac.NewAllowedMethodsMiddleware(rbacConfig).Eval(authCtx)
		Expect(err).NotTo(HaveOccurred())
		Expect(rbac.AllowedMethodFromContext(authCtx)).NotTo(BeNil())
	})

	Context("VerifyScopeForUser", func() {
		When("an allowed method has the AllUsers scope", func() {
			BeforeEach(func() {
				scope = rbacv1.Scope_ALL_USERS
			})
			When("the authenticated user is not the resource's assigned user", func() {
				It("should allow access", func() {
					// the user arg here is the owner of the resource. the client
					// is set up to always be clientUser.
					Expect(rbac.VerifyScopeForUser(authCtx, assignedUser)).To(Succeed())
				})
			})
			When("the authenticated user is the resource's assigned user", func() {
				It("should allow access", func() {
					Expect(rbac.VerifyScopeForUser(authCtx, clientUser)).To(Succeed())
				})
			})
		})
		When("an allowed method has the CurrentUser scope", func() {
			BeforeEach(func() {
				scope = rbacv1.Scope_CURRENT_USER
			})
			When("the authenticated user is not the resource's assigned user", func() {
				It("should deny access", func() {
					err := rbac.VerifyScopeForUser(authCtx, assignedUser)
					Expect(status.Convert(err).Code()).To(Equal(codes.PermissionDenied))
				})
			})
			When("the authenticated user is the resource's assigned user", func() {
				It("should allow access", func() {
					Expect(rbac.VerifyScopeForUser(authCtx, clientUser)).To(Succeed())
				})
			})
		})
	})

	Context("FilterByScope", func() {
		When("an allowed method has the AllUsers scope", func() {
			BeforeEach(func() {
				scope = rbacv1.Scope_ALL_USERS
			})
			It("should return all items", func() {

				filtered, err := rbac.FilterByScope(authCtx, sampleItems)
				Expect(err).NotTo(HaveOccurred())
				Expect(filtered).To(Equal(sampleItems))
			})
		})
		When("an allowed method has the CurrentUser scope", func() {
			BeforeEach(func() {
				scope = rbacv1.Scope_CURRENT_USER
			})
			It("should return only items assigned to the current user", func() {
				filtered, err := rbac.FilterByScope(authCtx, sampleItems)
				Expect(err).NotTo(HaveOccurred())
				Expect(filtered).To(Equal([]testUserAssignable{clientUser, clientUser}))
			})
		})
	})

	When("an allowed method has no scope", func() {
		BeforeEach(func() {
			scope = rbacv1.Scope_NONE
		})

		Specify("VerifyScopeForUser should return ErrScopeNotSupported", func() {
			err := rbac.VerifyScopeForUser(authCtx, assignedUser)
			Expect(err).To(Equal(rbac.ErrScopeNotSupported))
			err = rbac.VerifyScopeForUser(authCtx, clientUser)
			Expect(err).To(Equal(rbac.ErrScopeNotSupported))
		})
		Specify("FilterByScope should return ErrScopeNotSupported", func() {
			_, err := rbac.FilterByScope(authCtx, sampleItems)
			Expect(err).To(Equal(rbac.ErrScopeNotSupported))
		})
	})
})
