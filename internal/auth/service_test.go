package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("fluffy-secret")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}

	ok, err := VerifyPassword("fluffy-secret", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected password to verify")
	}

	ok, err = VerifyPassword("wrong", hash)
	if err != nil {
		t.Fatalf("VerifyPassword() returned error for wrong password: %v", err)
	}
	if ok {
		t.Fatal("expected wrong password to fail")
	}
}
