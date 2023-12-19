package rbac

import (
	"context"
	"slices"

	rbacv1 "github.com/kralicky/jobserver/pkg/apis/rbac/v1"
	"github.com/kralicky/jobserver/pkg/auth"
	"github.com/kralicky/jobserver/pkg/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type allowedMethodKeyType struct{}

var allowedMethodKey = allowedMethodKeyType{}

func AllowedMethodFromContext(ctx context.Context) *rbacv1.AllowedMethod {
	v, ok := ctx.Value(allowedMethodKey).(*rbacv1.AllowedMethod)
	if !ok {
		panic("bug: allowed method not found in context (required middleware not configured)")
	}
	return v
}

type middleware struct {
	config *rbacv1.Config
}

var _ auth.Middleware = (*middleware)(nil)

func NewAllowedMethodsMiddleware(config *rbacv1.Config) *middleware {
	return &middleware{config: config}
}

// Eval implements auth.Middleware.
func (h *middleware) Eval(ctx context.Context) (context.Context, error) {
	user := auth.AuthenticatedUserFromContext(ctx)
	fullMethodName, ok := grpc.Method(ctx)
	if !ok {
		panic("bug: grpc method not found in context")
	}

	serviceName, methodName, ok := util.SplitFullyQualifiedMethodName(fullMethodName)
	if !ok {
		panic("bug: method name is not fully qualified")
	}
	// find roles that are bound to the subject
	roleIds := make(map[string]struct{})
	for _, rb := range h.config.GetRoleBindings() {
		if slices.Contains(rb.GetUsers(), string(user)) {
			roleIds[rb.GetRoleId()] = struct{}{}
		}
	}
	// for each matching role, check if it contains the method
	for _, role := range h.config.GetRoles() {
		if _, ok := roleIds[role.GetId()]; !ok {
			continue
		}
		if role.GetService() != serviceName {
			continue
		}
		for _, m := range role.GetAllowedMethods() {
			if m.GetName() == methodName {
				return context.WithValue(ctx, allowedMethodKey, proto.Clone(m).(*rbacv1.AllowedMethod)), nil
			}
		}
	}
	return ctx, status.Errorf(codes.PermissionDenied, "user %q is not authorized for method %q", user, fullMethodName)
}
