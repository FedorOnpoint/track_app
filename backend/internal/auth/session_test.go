package auth

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestGenerateSessionTokenHas256BitsAndStoresOnlyHash(t *testing.T) {
	first, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}
	second, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken second: %v", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(first.Plaintext)
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if len(raw) != 32 {
		t.Fatalf("token entropy bytes = %d, want 32", len(raw))
	}
	if len(first.Hash) != 32 {
		t.Fatalf("hash bytes = %d, want 32", len(first.Hash))
	}
	if bytes.Equal(raw, first.Hash) {
		t.Fatal("stored hash equals plaintext token bytes")
	}
	if first.Plaintext == second.Plaintext {
		t.Fatal("two generated tokens are equal")
	}
}
