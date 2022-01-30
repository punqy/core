package core

import (
	"context"
)

const SecurityContextKey = "security-context"

type SecurityContext struct {
	Token GuardToken `json:"token"`
}

func FromContext(ctx context.Context) (SecurityContext, bool) {
	s, ok := ctx.Value(SecurityContextKey).(SecurityContext)
	return s, ok
}

type UserInterface interface {
	GetID() string
	GetPassword() string
	GetUsername() string
}

type GuardToken interface {
	User() UserInterface
	Provider() string
}

type UserProvider interface {
	FindUserByID(ctx context.Context, id string) (UserInterface, error)
	FindUserByUsername(ctx context.Context, username string) (UserInterface, error)
}
