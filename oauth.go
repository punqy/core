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

type TokenValues struct {
	Token     string
	ExpiresAt time.Time
}

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
	Find(ctx context.Context, id string) (OAuthClient, error)
	GetClient(ctx context.Context, id string, secret string, grantType GrantType) (OAuthClient, error)
}

type OAuthAccessTokenValues struct {
	Token     string
	UserId    *string
	ClientId  string
	ExpiresAt time.Time
}

type OAuthAccessToken interface {
	GetUserID() *string
	GetClientID() string
}

type OAuthAccessTokenStorage interface {
	CheckCredentials(ctx context.Context, token string) (OAuthAccessToken, error)
	CreateAccessToken(ctx context.Context, user UserInterface, client OAuthClient) (TokenValues, error)
}

type OAuthRefreshTokenValues struct {
	Token     string
	UserId    *string
	ClientId  string
	ExpiresAt time.Time
}

type OAuthRefreshToken interface {
	GetUserID() *string
	GetClientID() string
}

type OAuthRefreshTokenStorage interface {
	CheckCredentials(ctx context.Context, token string) (OAuthRefreshToken, error)
	CreateRefreshToken(ctx context.Context, user UserInterface, client OAuthClient) (TokenValues, error)
}

type UserStorage interface {
	CheckCredentials(ctx context.Context, username, password string) (UserInterface, error)
}

type oauth struct {
	clientStorage       OAuthClientStorage
	accessTokenStorage  OAuthAccessTokenStorage
	refreshTokenStorage OAuthRefreshTokenStorage
	userStorage         UserStorage
	userProvider        UserProvider
	accessTokenTTL      int
	refreshTokenTTL     int
}

func NewOAuth(
	storage OAuthClientStorage,
	accessTokenStorage OAuthAccessTokenStorage,
	refreshTokenStorage OAuthRefreshTokenStorage,
	userStorage UserStorage,
	userProvider UserProvider,
	accessTokenTTL int,
	refreshTokenTTL int,
) OAuth {
	return &oauth{
		clientStorage:       storage,
		accessTokenStorage:  accessTokenStorage,
		refreshTokenStorage: refreshTokenStorage,
		userStorage:         userStorage,
		userProvider:        userProvider,
		accessTokenTTL:      accessTokenTTL,
		refreshTokenTTL:     refreshTokenTTL,
	}
}

func (a *oauth) GrantAccessToken(ctx context.Context, req GrantAccessTokenRequest) (GrantAccessTokenResponse, error) {
	var response GrantAccessTokenResponse
	client, err := a.clientStorage.GetClient(ctx, req.ClientId, req.ClientSecret, req.GrantType)
	if err != nil {
		return response, InvalidGrantErr()
	}
	var user UserInterface
	switch req.GrantType {
	case GrantTypePassword:
		user, err = a.grantAccessTokenUserCredentials(ctx, req.Username, req.Password)
	case GrantTypeRefreshToken:
		user, err = a.grantAccessTokenRefresh(ctx, req.RefreshToken)
	case ClientCredentials:
		user, err = a.grantAccessTokenClientCredentials(ctx, client)
	default:
		return response, UnknownGrantTypeErr()
	}
	if err != nil {
		return response, err
	}
	return a.createAccessTokenResponse(ctx, user, client)
}

func (a *oauth) grantAccessTokenUserCredentials(ctx context.Context, username string, password string) (UserInterface, error) {
	return a.userStorage.CheckCredentials(ctx, username, password)
}

func (a *oauth) grantAccessTokenClientCredentials(ctx context.Context, client OAuthClient) (UserInterface, error) {
	return nil, nil
}

func (a *oauth) grantAccessTokenRefresh(ctx context.Context, token string) (UserInterface, error) {
	tok, err := a.refreshTokenStorage.CheckCredentials(ctx, token)
	if err != nil {
		return nil, err
	}
	if tok.GetUserID() != nil {
		return a.userProvider.FindUserByID(ctx, *tok.GetUserID())
	}

	return nil, nil
}

func (a *oauth) createAccessTokenResponse(ctx context.Context, user UserInterface, client OAuthClient) (GrantAccessTokenResponse, error) {
	var response GrantAccessTokenResponse
	at, err := a.accessTokenStorage.CreateAccessToken(ctx, user, client)
	if err != nil {
		return response, err
	}
	rt, err := a.refreshTokenStorage.CreateRefreshToken(ctx, user, client)
	if err != nil {
		return response, err
	}
	response.AccessToken = at.Token
	response.AccessTokenExpiresAt = at.ExpiresAt.Unix()
	response.RefreshToken = rt.Token
	response.RefreshTokenExpiresAt = rt.ExpiresAt.Unix()

	return response, nil
}

type OauthAuthToken struct {
	client OAuthClient
	user   UserInterface
}

func NewOauthToken(client OAuthClient, user UserInterface) OauthAuthToken {
	return OauthAuthToken{
		client: client,
		user:   user,
	}
}

func (o *OauthAuthToken) Client() OAuthClient {
	return o.client
}

func (o OauthAuthToken) User() UserInterface {
	return o.user
}

func (o OauthAuthToken) Provider() string {
	return "oauth"
}

type OAuthAuthenticator interface {
	Authenticate(request Request) (GuardToken, error)
}

type oauthAuthenticator struct {
	accessTokenStorage OAuthAccessTokenStorage
	clientStorage      OAuthClientStorage
	userProvider       UserProvider
}

func NewOAuthAuthenticator(ats OAuthAccessTokenStorage, cs OAuthClientStorage, up UserProvider) OAuthAuthenticator {
	return &oauthAuthenticator{accessTokenStorage: ats, clientStorage: cs, userProvider: up}
}

func (a *oauthAuthenticator) Authenticate(request Request) (GuardToken, error) {
	tokenHeader := request.Request.Header.Peek("Authorization")
	if tokenHeader == nil || len(tokenHeader) == 0 {
		return nil, AuthorizationRequiredErr()
	}
	token := strings.Split(string(tokenHeader), " ")
	if len(token) > 2 {
		return nil, AuthorizationRequiredErr()
	}
	accessToken, err := a.accessTokenStorage.CheckCredentials(request, token[1])
	if err != nil {
		return nil, err
	}
	client, err := a.clientStorage.Find(request, accessToken.GetClientID())
	if err != nil {
		return nil, err
	}
	if accessToken.GetUserID() == nil {
		return NewOauthToken(client, nil), nil
	}
	user, err := a.userProvider.FindUserByID(request, *accessToken.GetUserID())
	if err != nil {
		return nil, err
	}
	return NewOauthToken(client, user), nil
}
