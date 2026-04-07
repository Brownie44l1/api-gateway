package middleware

import "context"

type contentKey string

const userContextKey contentKey = "user"

type AuthenticatedUser struct {
	ID string
	Roles []string
}

func UserFromContext(ctx context.Context) (*AuthenticatedUser, bool) {
	user, ok := ctx.Value(userContextKey).(*AuthenticatedUser)
	return user, ok
}