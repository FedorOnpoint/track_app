package auth

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserStore struct{ pool *pgxpool.Pool }

func NewUserStore(pool *pgxpool.Pool) *UserStore { return &UserStore{pool: pool} }

func (store *UserStore) CreateUserIfAbsent(ctx context.Context, login, passwordHash string) (bool, error) {
	result, err := store.pool.Exec(ctx, `
		INSERT INTO users (login, password_hash)
		VALUES ($1, $2)
		ON CONFLICT (login) DO NOTHING`, login, passwordHash)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}
