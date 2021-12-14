package core

import (
	"context"
	"regexp"
)

type Firewall interface {
	Handle(req Request, next Handler) Response
	Config() FirewallConfig
}

type firewall struct {
	enabled     bool
	config      FirewallConfig
}

func NewFirewall(enabled bool, firewallConfig FirewallConfig) Firewall {
	return &firewall{
		enabled: enabled,
		config:  firewallConfig,
	}
}

type Area struct {
	Secure        bool
	Pattern       string
	Authenticator Authenticator
}

type Authenticator interface {
	Authorize(r Request) (AuthToken, error)
}

type VoidAuthenticator struct {
}

func (f *firewall) Config() FirewallConfig {
	return f.config
}

func (f *firewall) Handle(req Request, next Handler) Response {
	for _, area := range f.config {
		if !regexp.MustCompile(area.Pattern).MatchString(req.RequestURI) {
			continue
		}
		if !area.Secure {
			return next(req)
		}
		if area.Authenticator == nil {
			panic("Secure area must have an Authenticator.")
		}
		token, err := area.Authenticator.Authorize(req)
		if err != nil {
			return NewErrorJsonResponse(AccessDeniedErr(err.Error()))
		}
		if token == nil {
			return NewErrorJsonResponse(InvalidGrantErr())
		}
		securityContext := Context{
			Token: token,
		}
		ctx := context.WithValue(req.Context(), ContextKey, securityContext)
		req.Request = req.WithContext(ctx)
		if appContext, ok := ctx.Value(profileContextKey).(Profile); ok {
			appContext.SetSecurityContext(securityContext)
		}

		return next(req)
	}
	return NewErrorJsonResponse(AccessDeniedErr())
}
