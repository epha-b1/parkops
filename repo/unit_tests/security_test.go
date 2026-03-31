package unit_tests

import (
	"bytes"
	"testing"

	"parkops/internal/platform/security"
)

func TestPasswordPolicy(t *testing.T) {
	t.Parallel()

	if err := security.ValidatePasswordPolicy("short123"); err == nil {
		t.Fatal("expected short password to fail")
	}

	if err := security.ValidatePasswordPolicy("alllettersonly"); err == nil {
		t.Fatal("expected no-digit password to fail")
	}

	if err := security.ValidatePasswordPolicy("123456789012"); err == nil {
		t.Fatal("expected no-letter password to fail")
	}

	if err := security.ValidatePasswordPolicy("validPassword123"); err != nil {
		t.Fatalf("expected valid password to pass: %v", err)
	}
}

func TestHashPasswordAndVerify(t *testing.T) {
	t.Parallel()

	hash, err := security.HashPassword("validPassword123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	ok, err := security.VerifyPassword(hash, "validPassword123")
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if !ok {
		t.Fatal("expected password verification to pass")
	}

	ok, err = security.VerifyPassword(hash, "wrongPassword123")
	if err != nil {
		t.Fatalf("verify password wrong input: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password verification to fail")
	}
}

func TestEncryptionRoundTrip(t *testing.T) {
	t.Parallel()

	key := []byte("0123456789abcdef0123456789abcdef")
	enc, err := security.EncryptString(key, "sensitive-note")
	if err != nil {
		t.Fatalf("encrypt string: %v", err)
	}

	if bytes.Contains([]byte(enc), []byte("sensitive-note")) {
		t.Fatal("encrypted output leaked plaintext")
	}

	plain, err := security.DecryptString(key, enc)
	if err != nil {
		t.Fatalf("decrypt string: %v", err)
	}
	if plain != "sensitive-note" {
		t.Fatalf("unexpected decrypted value: %s", plain)
	}
}
