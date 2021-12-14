package core

import (
	"context"
)

const SecurityContextKey = "security-context"

type SecurityContext struct {
	Token AuthToken `json:"token"`
}

func FromContext(ctx context.Context) (SecurityContext, bool) {
	s, ok := ctx.Value(SecurityContextKey).(SecurityContext)
	return s, ok
}

type User interface {
	GetID() string
}

type AuthToken interface {
	User() User
	Provider() string
}

type UserProvider interface {
	FindUserByID(context.Context, string) (User, error)
}
