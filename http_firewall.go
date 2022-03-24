package core

import (
	"regexp"
)

type Firewall interface {
	Handle(req Request, next Handler) Response
	Config() FirewallConfig
}

type firewall struct {
	enabled bool
	config  FirewallConfig
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
	Authenticate(r Request) (GuardToken, error)
}

type VoidAuthenticator struct {
}

func (f *firewall) Config() FirewallConfig {
	return f.config
}

func (f *firewall) Handle(req Request, next Handler) Response {
	for _, area := range f.config {
		if !regexp.MustCompile(area.Pattern).Match(req.Path()) {
			continue
		}
		if !area.Secure {
			return next(req)
		}
		if area.Authenticator == nil {
			panic("Secure area must have an Authenticator.")
		}
		token, err := area.Authenticator.Authenticate(req)
		if err != nil {
			return NewErrorJSONResponse(UnauthorizedErr(err.Error()))
		}
		if token == nil {
			return NewErrorJSONResponse(InvalidGrantErr())
		}
		securityContext := SecurityContext{
			Token: token,
		}
		req.SetUserValue(SecurityContextKey, securityContext)
		if appContext, ok := req.UserValue(profileContextKey).(*Profile); ok {
			appContext.SetSecurityContext(securityContext)
		}

		return next(req)
	}
	return NewErrorJSONResponse(AccessDeniedErr())
}
