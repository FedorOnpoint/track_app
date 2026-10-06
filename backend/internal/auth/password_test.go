package auth

import (
	"strings"
	"testing"
)

func TestArgon2IDHashAndVerify(t *testing.T) {
	service := NewPasswordService()
	hash, err := service.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if strings.Contains(hash, "correct horse") {
		t.Fatal("hash contains plaintext password")
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected hash parameters: %s", hash)
	}
	matched, err := service.Verify("correct horse battery staple", hash)
	if err != nil || !matched {
		t.Fatalf("Verify correct = (%v, %v)", matched, err)
	}
	matched, err = service.Verify("wrong password", hash)
	if err != nil || matched {
		t.Fatalf("Verify wrong = (%v, %v)", matched, err)
	}
}

func TestPasswordVerifyRejectsMalformedHash(t *testing.T) {
	if matched, err := NewPasswordService().Verify("password", "not-a-phc-hash"); err == nil || matched {
		t.Fatalf("Verify malformed = (%v, %v), want safe failure", matched, err)
	}
}
