package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresSessionStore struct{ pool *pgxpool.Pool }

func NewPostgresSessionStore(pool *pgxpool.Pool) *PostgresSessionStore {
	return &PostgresSessionStore{pool: pool}
}

func (store *PostgresSessionStore) FindUserByLogin(ctx context.Context, login string) (User, error) {
	var user User
	err := store.pool.QueryRow(ctx, `SELECT id, login, password_hash FROM users WHERE login = $1`, login).Scan(&user.ID, &user.Login, &user.PasswordHash)
	return user, err
}

func (store *PostgresSessionStore) CreateSession(ctx context.Context, session Session) error {
	_, err := store.pool.Exec(ctx, `INSERT INTO sessions (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`, session.ID, session.UserID, session.TokenHash, session.ExpiresAt)
	return err
}

func (store *PostgresSessionStore) FindUserByTokenHash(ctx context.Context, hash []byte, now time.Time) (UserSession, error) {
	var result UserSession
	err := store.pool.QueryRow(ctx, `
		SELECT u.id, u.login, u.password_hash, s.id
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > $2`, hash, now).
		Scan(&result.User.ID, &result.User.Login, &result.User.PasswordHash, &result.SessionID)
	return result, err
}

func (store *PostgresSessionStore) RevokeSession(ctx context.Context, id uuid.UUID, now time.Time) error {
	_, err := store.pool.Exec(ctx, `UPDATE sessions SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`, id, now)
	return err
}
