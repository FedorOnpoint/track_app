package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRegistrationStore struct{ pool *pgxpool.Pool }

func NewPostgresRegistrationStore(pool *pgxpool.Pool) *PostgresRegistrationStore {
	return &PostgresRegistrationStore{pool: pool}
}

func (s *PostgresRegistrationStore) CreateInvitation(ctx context.Context, hash []byte, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO registration_invitations(token_hash,expires_at) VALUES($1,$2)`, hash, expiresAt)
	return err
}

func (s *PostgresRegistrationStore) TakeInvitationAttempt(ctx context.Context, hash []byte, now time.Time) error {
	result, err := s.pool.Exec(ctx, `UPDATE registration_invitations SET
	 attempts=CASE WHEN attempts_reset_at<=$2 THEN 1 ELSE attempts+1 END,
	 attempts_reset_at=CASE WHEN attempts_reset_at<=$2 THEN $2+interval '15 minutes' ELSE attempts_reset_at END
	 WHERE token_hash=$1 AND used_at IS NULL AND expires_at>GREATEST($2,clock_timestamp()) AND (attempts<10 OR attempts_reset_at<=$2)`, hash, now)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 1 {
		return nil
	}
	var valid bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM registration_invitations WHERE token_hash=$1 AND used_at IS NULL AND expires_at>GREATEST($2,clock_timestamp()))`, hash, now).Scan(&valid); err != nil {
		return err
	}
	if valid {
		return ErrRegistrationRateLimited
	}
	return ErrInvalidInvitation
}

func (s *PostgresRegistrationStore) RegisterWithInvitation(ctx context.Context, user User, session Session, hash []byte, now time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var inviteID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM registration_invitations WHERE token_hash=$1 AND used_at IS NULL AND expires_at>GREATEST($2,clock_timestamp()) FOR UPDATE`, hash, now).Scan(&inviteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidInvitation
	}
	if err != nil {
		return err
	}
	result, err := tx.Exec(ctx, `INSERT INTO users(id,login,password_hash) VALUES($1,$2,$3) ON CONFLICT(login) DO NOTHING`, user.ID, user.Login, user.PasswordHash)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return ErrLoginUnavailable
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sessions(id,user_id,token_hash,expires_at,created_at,last_seen_at) VALUES($1,$2,$3,$4,$5,$5)`, session.ID, user.ID, session.TokenHash, session.ExpiresAt, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE registration_invitations SET used_at=$2,used_by=$3 WHERE id=$1`, inviteID, now, user.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
