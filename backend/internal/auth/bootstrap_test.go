package auth

import (
	"context"
	"testing"
)

type memoryUsers struct{ hash string }

func (users *memoryUsers) CreateUserIfAbsent(_ context.Context, login, passwordHash string) (bool, error) {
	if users.hash != "" {
		return false, nil
	}
	users.hash = passwordHash
	return true, nil
}

type fixedHasher struct{}

func (fixedHasher) Hash(string) (string, error) { return "first-hash", nil }

func TestBootstrapDoesNotReplaceExistingPassword(t *testing.T) {
	users := &memoryUsers{}
	bootstrap := NewBootstrapper(users, fixedHasher{})
	created, err := bootstrap.Create(context.Background(), "fedor", "long-enough-password")
	if err != nil || !created {
		t.Fatalf("first bootstrap = (%v, %v), want created", created, err)
	}
	created, err = bootstrap.Create(context.Background(), "fedor", "different-password")
	if err != nil || created {
		t.Fatalf("second bootstrap = (%v, %v), want existing", created, err)
	}
	if users.hash != "first-hash" {
		t.Fatalf("password hash changed to %q", users.hash)
	}
}

func TestBootstrapRejectsInvalidCredentialsBeforeStorage(t *testing.T) {
	tests := []struct{ login, password string }{{"", "long-enough-password"}, {"bad login", "long-enough-password"}, {"fedor", "short"}}
	for _, test := range tests {
		users := &memoryUsers{}
		_, err := NewBootstrapper(users, fixedHasher{}).Create(context.Background(), test.login, test.password)
		if err == nil {
			t.Errorf("Create(%q, password length %d) accepted", test.login, len(test.password))
		}
		if users.hash != "" {
			t.Error("invalid credentials reached storage")
		}
	}
}
