package auth

import (
	"context"

	"github.com/google/uuid"
)

type userSessionKey struct{}

func withUserSession(ctx context.Context, session UserSession) context.Context {
	return context.WithValue(ctx, userSessionKey{}, session)
}
func userSessionFromContext(ctx context.Context) (UserSession, bool) {
	session, ok := ctx.Value(userSessionKey{}).(UserSession)
	return session, ok
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	session, ok := userSessionFromContext(ctx)
	return session.User.ID, ok && session.User.ID != uuid.Nil
}

func ContextWithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return withUserSession(ctx, UserSession{User: User{ID: userID}})
}
