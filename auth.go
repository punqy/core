package core

import (
	"context"
)

const ContextKey = "security-context"

type Context struct {
	Token AuthToken `json:"token"`
}

func FromContext(ctx context.Context) (Context, bool) {
	s, ok := ctx.Value(ContextKey).(Context)
	return s, ok
}

type Profile interface {
	SetSecurityContext(securityContext Context)
}

const profileContextKey = "punqy-profile"

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
