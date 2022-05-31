package core

import (
	"regexp"
)

const (
	BeforeAuthEventName = "core.firewall.before_auth"
	AfterAuthEventName  = "core.firewall.after_auth"
)

type BeforeAuthenticateEvent struct {
	Area    Area
	Request Request
}

func (b BeforeAuthenticateEvent) GetName() string {
	return BeforeAuthEventName
}

type AfterAuthenticateEvent struct {
	Area    Area
	Request Request
	Token   GuardToken
}

func (b AfterAuthenticateEvent) GetName() string {
	return AfterAuthEventName
}

type Firewall interface {
	Handle(req Request, next Handler) Response
	Config() FirewallConfig
}

type firewall struct {
	enabled    bool
	config     FirewallConfig
	dispatcher EventDispatcher
}

func NewFirewall(enabled bool, firewallConfig FirewallConfig, dispatcher EventDispatcher) Firewall {
	return &firewall{
		enabled:    enabled,
		config:     firewallConfig,
		dispatcher: dispatcher,
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
		if err := dispatchEventSilent(req, f.dispatcher, BeforeAuthenticateEvent{Area: area, Request: req}); err != nil {
			return NewErrorJSONResponse(InternalServerErr(err.Error()))
		}
		token, err := area.Authenticator.Authenticate(req)
		if err != nil {
			return NewErrorJSONResponse(UnauthorizedErr(err.Error()))
		}
		if token == nil {
			return NewErrorJSONResponse(InvalidGrantErr())
		}
		if err := dispatchEventSilent(req, f.dispatcher, AfterAuthenticateEvent{Area: area, Request: req, Token: token}); err != nil {
			return NewErrorJSONResponse(InternalServerErr(err.Error()))
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
