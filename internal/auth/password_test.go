package auth

import "testing"

func TestPasswordHasherHashAndVerify(t *testing.T) {
	hasher := NewPasswordHasher(1000, 32, 16)

	hash, err := hasher.Hash("super-secret-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if !hasher.Verify("super-secret-password", hash) {
		t.Fatalf("Verify() = false, want true")
	}

	if hasher.Verify("wrong-password", hash) {
		t.Fatalf("Verify() = true for wrong password")
	}
}
