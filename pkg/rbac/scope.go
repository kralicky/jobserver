package rbac

import (
	"context"

	rbacv1 "github.com/kralicky/jobserver/pkg/apis/rbac/v1"
	"github.com/kralicky/jobserver/pkg/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrScopeNotSupported = status.Errorf(codes.InvalidArgument, "scope not supported")

// VerifyScopeForUser verifies that the authenticated user in the context is
// allowed to access the resource assigned to 'assignedUser', based on the
// scope of the AllowedMethod in the context.
func VerifyScopeForUser(ctx context.Context, assignedUser auth.AuthenticatedUser) error {
	user := auth.AuthenticatedUserFromContext(ctx)
	am := AllowedMethodFromContext(ctx)
	switch am.GetScope() {
	case rbacv1.Scope_CURRENT_USER:
		if user != assignedUser {
			return status.Errorf(codes.PermissionDenied, "permission denied")
		}
		return nil
	case rbacv1.Scope_ALL_USERS:
		return nil
	case rbacv1.Scope_NONE:
		fallthrough
	default:
		return ErrScopeNotSupported
	}
}

type UserAssignable interface {
	AssignedUser() auth.AuthenticatedUser
}

// FilterByScope filters the given slice of UserAssignable items based on
// the AuthenticatedUser and AllowedMethod in the context.
//
// Specifically, if the list contains objects assigned to multiple users, and
// the AllowedMethod has a scope of CurrentUser, the returned slice will consist
// only of those items assigned to the authenticated user.
//
// If the scope of the method is AllUsers, no filtering is performed.
func FilterByScope[T UserAssignable, S ~[]T](ctx context.Context, items S) ([]T, error) {
	user := auth.AuthenticatedUserFromContext(ctx)
	am := AllowedMethodFromContext(ctx)
	var filtered []T
	switch am.GetScope() {
	case rbacv1.Scope_CURRENT_USER:
		for _, item := range items {
			if item.AssignedUser() == user {
				filtered = append(filtered, item)
			}
		}
	case rbacv1.Scope_ALL_USERS:
		filtered = items
	case rbacv1.Scope_NONE:
		fallthrough
	default:
		return nil, ErrScopeNotSupported
	}
	return filtered, nil
}
