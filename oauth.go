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
	Expired() bool
	GetUserID() *string
	GetClientID() string
}

type OAuthAccessTokenStorage interface {
	GetAccessToken(ctx context.Context, token string) (OAuthAccessToken, error)
	CreateAccessToken(ctx context.Context, user User, client OAuthClient, token string, expiresAt time.Time) error
	CheckCredentials(ctx context.Context, token string) (User, error)
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
	CheckCredentials(ctx context.Context, token string) (User, error)
	CreateRefreshToken(ctx context.Context, user User, client OAuthClient, token string, expiresAt time.Time) error
}

type UserStorage interface {
	CheckCredentials(ctx context.Context, username, password string) (User, error)
}

type oauth struct {
	clientStorage      OAuthClientStorage
	accessTokenStorage OAuthAccessTokenStorage
	tokenStorage       OAuthRefreshTokenStorage
	userStorage        UserStorage
	userProvider       UserProvider
	accessTokenTTL     int
	refreshTokenTTL    int
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
		clientStorage:      storage,
		accessTokenStorage: accessTokenStorage,
		tokenStorage:       refreshTokenStorage,
		userStorage:        userStorage,
		userProvider:       userProvider,
		accessTokenTTL:     accessTokenTTL,
		refreshTokenTTL:    refreshTokenTTL,
	}
}

func (a *oauth) GrantAccessToken(ctx context.Context, req GrantAccessTokenRequest) (GrantAccessTokenResponse, error) {
	var response GrantAccessTokenResponse
	client, err := a.clientStorage.GetClient(ctx, req.ClientId, req.ClientSecret, req.GrantType)
	if err != nil {
		return response, InvalidGrantErr()
	}
	var user User
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

func (a *oauth) grantAccessTokenUserCredentials(ctx context.Context, username string, password string) (User, error) {
	var invalidCredentialsErr = InvalidCredentialsErr()
	user, err := a.userStorage.CheckCredentials(ctx, username, password)
	if err != nil {
		return user, invalidCredentialsErr
	}
	return user, nil
}

func (a *oauth) grantAccessTokenClientCredentials(ctx context.Context, client OAuthClient) (User, error) {
	return nil, nil
}

func (a *oauth) grantAccessTokenRefresh(ctx context.Context, token string) (User, error) {
	var invalidCredentialsErr = InvalidCredentialsErr()
	user, err := a.tokenStorage.CheckCredentials(ctx, token)
	if err != nil {
		return user, invalidCredentialsErr
	}
	return user, nil
}

func (a *oauth) createAccessTokenResponse(ctx context.Context, user User, client OAuthClient) (GrantAccessTokenResponse, error) {
	accessTokenExpiresAt := time.Now().Add(time.Duration(a.accessTokenTTL) * time.Minute)
	refreshTokenExpiresAt := time.Now().Add(time.Duration(a.refreshTokenTTL) * time.Minute)
	response := GrantAccessTokenResponse{
		AccessToken:           RandomString(64),
		RefreshToken:          RandomString(64),
		AccessTokenExpiresAt:  accessTokenExpiresAt.Unix(),
		RefreshTokenExpiresAt: refreshTokenExpiresAt.Unix(),
	}
	if err := a.accessTokenStorage.CreateAccessToken(ctx, user, client, response.AccessToken, accessTokenExpiresAt); err != nil {
		return response, err
	}
	if err := a.tokenStorage.CreateRefreshToken(ctx, user, client, response.AccessToken, refreshTokenExpiresAt); err != nil {
		return response, err
	}
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
	tokenHeader := request.Header.Get("Authorization")
	if tokenHeader == "" {
		return nil, AuthorizationRequiredErr()
	}
	token := strings.Split(tokenHeader, " ")
	if len(token) > 2 {
		return nil, AuthorizationRequiredErr()
	}
	accessToken, err := a.accessTokenStorage.GetAccessToken(request.Context(), token[1])
	if err != nil {
		return nil, err
	}
	if accessToken.Expired() {
		return nil, AuthorizationExpiredErr()
	}
	client, err := a.clientStorage.Find(request.Context(), accessToken.GetClientID())
	if err != nil {
		return nil, err
	}
	if accessToken.GetUserID() == nil {
		return NewOauthToken(client, nil), nil
	}
	user, err := a.userProvider.FindUserByID(request.Context(), *accessToken.GetUserID())
	if err != nil {
		return nil, err
	}
	return NewOauthToken(client, user), nil
}
