package middleware

import "golang.org/x/net/context"

type AccessTokenSigner interface {
	ValidAccessToken(token string) bool
	ExtractAccessTokenIdentifiers(token string) (sub, sid string, err error)
}

type SessionChecker interface {
	IsSessionActive(ctx context.Context, sid, userID string) (bool, error)
}
