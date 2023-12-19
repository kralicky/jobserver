package rbac_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rbacv1 "github.com/kralicky/jobserver/pkg/apis/rbac/v1"
	"github.com/kralicky/jobserver/pkg/auth"
	"github.com/kralicky/jobserver/pkg/rbac"
)

var _ = Describe("AllowedMethods Middleware", func() {
	var authCtx context.Context
	var rbacConfig *rbacv1.Config
	var middleware auth.Middleware
	const clientUser = "client-user"
	JustBeforeEach(func() {
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

		middleware = rbac.NewAllowedMethodsMiddleware(rbacConfig)
	})

	When("the user is allowed access to the method", func() {
		BeforeEach(func() {
			rbacConfig = &rbacv1.Config{
				Roles: []*rbacv1.Roles{
					{
						Id:             "test-role",
						Service:        "foo.bar.Example",
						AllowedMethods: []*rbacv1.AllowedMethod{{Name: "Test"}},
					},
				},
				RoleBindings: []*rbacv1.RoleBindings{
					{
						Id:     "test-role-binding",
						RoleId: "test-role",
						Users:  []string{clientUser},
					},
				},
			}
		})
		It("should evaluate without error", func() {
			ctx, err := middleware.Eval(authCtx)
			Expect(err).NotTo(HaveOccurred())
			Expect(rbac.AllowedMethodFromContext(ctx)).To(BeEquivalentTo(rbacConfig.Roles[0].AllowedMethods[0]))
		})
	})
	When("the user's role does not contain the method they are calling", func() {
		BeforeEach(func() {
			rbacConfig = &rbacv1.Config{
				Roles: []*rbacv1.Roles{
					{
						Id:             "test-role",
						Service:        "foo.bar.Example",
						AllowedMethods: []*rbacv1.AllowedMethod{{Name: "WrongMethod"}},
					},
				},
				RoleBindings: []*rbacv1.RoleBindings{
					{
						Id:     "test-role-binding",
						RoleId: "test-role",
						Users:  []string{clientUser},
					},
				},
			}
		})
		It("should return a PermissionDenied error", func() {
			_, err := middleware.Eval(authCtx)
			Expect(status.Code(err)).To(Equal(codes.PermissionDenied))
			Expect(status.Convert(err).Message()).To(Equal(`user "client-user" is not authorized for method "/foo.bar.Example/Test"`))
		})
	})
	When("the user's role contains the method name, but the service name does not match", func() {
		BeforeEach(func() {
			rbacConfig = &rbacv1.Config{
				Roles: []*rbacv1.Roles{
					{
						Id:             "test-role",
						Service:        "foo.bar.WrongService",
						AllowedMethods: []*rbacv1.AllowedMethod{{Name: "Test"}},
					},
				},
				RoleBindings: []*rbacv1.RoleBindings{
					{
						Id:     "test-role-binding",
						RoleId: "test-role",
						Users:  []string{clientUser},
					},
				},
			}
		})
		It("should return a PermissionDenied error", func() {
			_, err := middleware.Eval(authCtx)
			Expect(status.Code(err)).To(Equal(codes.PermissionDenied))
			Expect(status.Convert(err).Message()).To(Equal(`user "client-user" is not authorized for method "/foo.bar.Example/Test"`))
		})
	})
	When("the role contains the method, but the user is not named in the role binding", func() {
		BeforeEach(func() {
			rbacConfig = &rbacv1.Config{
				Roles: []*rbacv1.Roles{
					{
						Id:             "test-role",
						Service:        "foo.bar.Example",
						AllowedMethods: []*rbacv1.AllowedMethod{{Name: "Test"}},
					},
				},
				RoleBindings: []*rbacv1.RoleBindings{
					{
						Id:     "test-role-binding",
						RoleId: "test-role",
						Users:  []string{"wrong-user"},
					},
				},
			}
		})
		It("should return a PermissionDenied error", func() {
			_, err := middleware.Eval(authCtx)
			Expect(status.Code(err)).To(Equal(codes.PermissionDenied))
			Expect(status.Convert(err).Message()).To(Equal(`user "client-user" is not authorized for method "/foo.bar.Example/Test"`))
		})
	})
	When("the rbac config is empty", func() {
		BeforeEach(func() {
			rbacConfig = &rbacv1.Config{}
		})
		It("should return a PermissionDenied error", func() {
			_, err := middleware.Eval(authCtx)
			Expect(status.Code(err)).To(Equal(codes.PermissionDenied))
			Expect(status.Convert(err).Message()).To(Equal(`user "client-user" is not authorized for method "/foo.bar.Example/Test"`))
		})
	})
})
