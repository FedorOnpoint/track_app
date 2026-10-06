package auth

import "context"

type userSessionKey struct{}

func withUserSession(ctx context.Context, session UserSession) context.Context {
	return context.WithValue(ctx, userSessionKey{}, session)
}
func userSessionFromContext(ctx context.Context) (UserSession, bool) {
	session, ok := ctx.Value(userSessionKey{}).(UserSession)
	return session, ok
}
