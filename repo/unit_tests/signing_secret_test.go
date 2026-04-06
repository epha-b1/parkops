package unit_tests

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"parkops/internal/platform/security"
	"parkops/internal/tracking"
)

func TestSigningSecretEncryptionRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes

	secret := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	enc, err := security.EncryptString(key, secret)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	dec, err := security.DecryptString(key, enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if dec != secret {
		t.Fatalf("round-trip mismatch: %q != %q", dec, secret)
	}
}

func TestDedicatedSecretHMACValidation(t *testing.T) {
	// Simulate: device has a dedicated secret, not plate number
	dedicatedSecret := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"
	plateNumber := "ABC-1234"
	deviceTime := "2026-01-01T10:00:00Z"

	// Sign with dedicated secret
	mac := hmac.New(sha256.New, []byte(dedicatedSecret))
	mac.Write([]byte(deviceTime))
	sig := hex.EncodeToString(mac.Sum(nil))

	// Validate with dedicated secret — should pass
	if !tracking.ValidateDeviceTimeHMAC(deviceTime, sig, dedicatedSecret) {
		t.Fatal("expected HMAC validation to pass with dedicated secret")
	}

	// Validate with plate number — should fail
	if tracking.ValidateDeviceTimeHMAC(deviceTime, sig, plateNumber) {
		t.Fatal("expected HMAC validation to fail with plate number as secret")
	}
}
