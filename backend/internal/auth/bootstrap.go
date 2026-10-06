package auth

import (
	"context"
	"errors"
	"regexp"
)

var (
	loginPattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)
	ErrInvalidLogin    = errors.New("login must contain 3-64 lowercase letters, digits, dots, dashes or underscores")
	ErrInvalidPassword = errors.New("password must contain 12-1024 bytes")
)

type UserCreator interface {
	CreateUserIfAbsent(ctx context.Context, login, passwordHash string) (bool, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type Bootstrapper struct {
	users  UserCreator
	hasher PasswordHasher
}

func NewBootstrapper(users UserCreator, hasher PasswordHasher) *Bootstrapper {
	return &Bootstrapper{users: users, hasher: hasher}
}

func (bootstrapper *Bootstrapper) Create(ctx context.Context, login, password string) (bool, error) {
	if !loginPattern.MatchString(login) {
		return false, ErrInvalidLogin
	}
	if len(password) < 12 || len(password) > 1024 {
		return false, ErrInvalidPassword
	}
	hash, err := bootstrapper.hasher.Hash(password)
	if err != nil {
		return false, err
	}
	return bootstrapper.users.CreateUserIfAbsent(ctx, login, hash)
}
