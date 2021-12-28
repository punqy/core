package core

import (
	"context"
	"strings"
	"time"
)

type GrantType string

func (g GrantType) String() string {
	return string(g)
}

const (
	GrantTypeRefreshToken GrantType = "refresh_token"
	ClientCredentials     GrantType = "client_credentials"
	GrantTypePassword     GrantType = "password"
)

type GrantAccessTokenRequest struct {
	GrantType    GrantType
	ClientSecret string
	RefreshToken string
	ClientId     string
	Username     string
	Password     string
}

type GrantAccessTokenResponse struct {
	AccessToken           string
	RefreshToken          string
	AccessTokenExpiresAt  int64
	RefreshTokenExpiresAt int64
}

type OAuth interface {
	GrantAccessToken(ctx context.Context, req GrantAccessTokenRequest) (GrantAccessTokenResponse, error)
}

type OAuthClient interface {
	GetID() string
}

type OAuthClientStorage interface {
	FindOneByClientIdSecretAndGrantType(ctx context.Context, cID, sec string, gt GrantType) (OAuthClient, error)
	Find(ctx context.Context, id string) (OAuthClient, error)
}

type OAuthAccessTokenValues struct {
	Token     string
	UserId    *string
	ClientId  string
	ExpiresAt time.Time
}

type OAuthAccessToken interface {
	Expired() bool
	SetExpired()
	GetUserID() *string
	GetClientID() string
}

type OAuthAccessTokenStorage interface {
	Create(ctx context.Context, entity OAuthAccessTokenValues) error
	FindOneByToken(ctx context.Context, token string) (OAuthAccessToken, error)
}

type OAuthRefreshTokenValues struct {
	Token     string
	UserId    *string
	ClientId  string
	ExpiresAt time.Time
}

type OAuthRefreshToken interface {
	Expired() bool
	SetExpired()
	GetUserID() *string
	GetClientID() string
}

type OAuthRefreshTokenStorage interface {
	Create(ctx context.Context, entity OAuthRefreshTokenValues) error
	Update(ctx context.Context, entity OAuthRefreshToken) error
	FindOneByToken(ctx context.Context, token string) (OAuthRefreshToken, error)
}

type oauth struct {
	cs              OAuthClientStorage
	ats             OAuthAccessTokenStorage
	rts             OAuthRefreshTokenStorage
	pe              PasswordEncoder
	up              UserProvider
	accessTokenTTL  int
	refreshTokenTTL int
}

func NewOAuth(
	cs OAuthClientStorage,
	ats OAuthAccessTokenStorage,
	rts OAuthRefreshTokenStorage,
	pe PasswordEncoder,
	up UserProvider,
	accessTokenTTL int,
	refreshTokenTTL int,
) OAuth {
	return &oauth{cs: cs, ats: ats, rts: rts, pe: pe, up: up, accessTokenTTL: accessTokenTTL, refreshTokenTTL: refreshTokenTTL}
}

func (a *oauth) GrantAccessToken(ctx context.Context, req GrantAccessTokenRequest) (GrantAccessTokenResponse, error) {
	var response GrantAccessTokenResponse
	client, err := a.cs.FindOneByClientIdSecretAndGrantType(ctx, req.ClientId, req.ClientSecret, req.GrantType)
	if err != nil {
		return response, InvalidGrantErr()
	}
	switch req.GrantType {
	case GrantTypePassword:
		return a.grantAccessTokenUserCredentials(ctx, client, req.Username, req.Password)
	case GrantTypeRefreshToken:
		return a.grantAccessTokenRefresh(ctx, req.RefreshToken)
	case ClientCredentials:
		return a.grantAccessTokenClientCredentials(ctx, client.GetID())
	default:
		return response, UnknownGrantTypeErr()
	}
}

func (a *oauth) grantAccessTokenUserCredentials(ctx context.Context, client OAuthClient, username string, password string) (GrantAccessTokenResponse, error) {
	var response GrantAccessTokenResponse
	var invalidCredentialsErr = InvalidCredentialsErr()
	user, err := a.up.FindUserByUsername(ctx, username)
	if err != nil {
		return response, invalidCredentialsErr
	}
	if valid, _ := a.pe.IsPasswordValid(user.GetPassword(), password); !valid {
		return response, invalidCredentialsErr
	}
	uid := user.GetID()
	return a.createAccessTokenResponse(ctx, &uid, client.GetID())
}

func (a *oauth) grantAccessTokenClientCredentials(ctx context.Context, clientID string) (GrantAccessTokenResponse, error) {
	return a.createAccessTokenResponse(ctx, nil, clientID)
}

func (a *oauth) grantAccessTokenRefresh(ctx context.Context, token string) (GrantAccessTokenResponse, error) {
	var response GrantAccessTokenResponse
	refreshToken, err := a.rts.FindOneByToken(ctx, token)
	if err != nil {
		return response, err
	}
	if refreshToken.Expired() {
		return response, UnauthorizedErr()
	}
	refreshToken.SetExpired()
	if err := a.rts.Update(ctx, refreshToken); err != nil {
		return response, err
	}
	return a.createAccessTokenResponse(ctx, refreshToken.GetUserID(), refreshToken.GetClientID())
}

func (a *oauth) createAccessTokenResponse(ctx context.Context, userId *string, clientId string) (GrantAccessTokenResponse, error) {
	response := GrantAccessTokenResponse{}
	atv := OAuthAccessTokenValues{
		Token:     RandomString(64),
		UserId:    userId,
		ClientId:  clientId,
		ExpiresAt: time.Now().Add(time.Duration(a.accessTokenTTL) * time.Minute),
	}
	if err := a.ats.Create(ctx, atv); err != nil {
		return response, err
	}
	rtv := OAuthRefreshTokenValues{
		Token:     RandomString(64),
		UserId:    userId,
		ClientId:  clientId,
		ExpiresAt: time.Now().Add(time.Duration(a.refreshTokenTTL) * time.Minute),
	}
	if err := a.rts.Create(ctx, rtv); err != nil {
		return response, err
	}
	response.AccessToken = atv.Token
	response.AccessTokenExpiresAt = atv.ExpiresAt.Unix()
	response.RefreshToken = rtv.Token
	response.RefreshTokenExpiresAt = rtv.ExpiresAt.Unix()

	return response, nil
}

type OauthAuthToken struct {
	client OAuthClient
	user   User
}

func NewOauthToken(client OAuthClient, user User) OauthAuthToken {
	return OauthAuthToken{
		client: client,
		user:   user,
	}
}

func (o *OauthAuthToken) Client() OAuthClient {
	return o.client
}

func (o OauthAuthToken) User() User {
	return nil
}

func (o OauthAuthToken) Provider() string {
	return "oauth"
}

type OAuthAuthenticator interface {
	Authorize(request Request) (AuthToken, error)
}

type oauthAuthenticator struct {
	ats OAuthAccessTokenStorage
	cs  OAuthClientStorage
	up  UserProvider
}

func NewOAuthAuthenticator(ats OAuthAccessTokenStorage, cs OAuthClientStorage, up UserProvider) OAuthAuthenticator {
	return &oauthAuthenticator{ats: ats, cs: cs, up: up}
}

func (a *oauthAuthenticator) Authorize(request Request) (AuthToken, error) {
	tokenHeader := request.Header.Get("Authorization")
	if tokenHeader == "" {
		return nil, AuthorizationRequiredErr()
	}
	token := strings.Split(tokenHeader, " ")
	if len(token) > 2 {
		return nil, AuthorizationRequiredErr()
	}
	accessToken, err := a.ats.FindOneByToken(request.Context(), token[1])
	if err != nil {
		return nil, err
	}
	if accessToken.Expired() {
		return nil, AuthorizationExpiredErr()
	}
	client, err := a.cs.Find(request.Context(), accessToken.GetClientID())
	if err != nil {
		return nil, err
	}
	if accessToken.GetUserID() == nil {
		return NewOauthToken(client, nil), nil
	}
	user, err := a.up.FindUserByID(request.Context(), *accessToken.GetUserID())
	if err != nil {
		return nil, err
	}
	return NewOauthToken(client, user), nil
}
