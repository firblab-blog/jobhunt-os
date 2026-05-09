package auth

import (
	"strconv"
	"strings"
	"testing"
)

func TestPasswordHashVerify(t *testing.T) {
	t.Parallel()

	password := t.Name()
	encoded, err := HashPassword(password, []byte("0123456789abcdef"), DefaultIterations)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	hash, err := ParsePasswordHash(encoded)
	if err != nil {
		t.Fatalf("ParsePasswordHash() error = %v", err)
	}

	if !hash.Verify(password) {
		t.Fatalf("Verify(correct password) = false, want true")
	}
	if hash.Verify(password + "/wrong") {
		t.Fatalf("Verify(wrong password) = true, want false")
	}
}

func TestHashPasswordRejectsWeakParameters(t *testing.T) {
	t.Parallel()

	if _, err := HashPassword(t.Name(), []byte("0123456789abcdef"), DefaultIterations-1); err == nil {
		t.Fatalf("HashPassword() with low iterations error = nil, want error")
	}
	if _, err := HashPassword(t.Name(), []byte("short"), DefaultIterations); err == nil {
		t.Fatalf("HashPassword() with short salt error = nil, want error")
	}
}

func TestParsePasswordHashRejectsInvalidHashes(t *testing.T) {
	t.Parallel()

	valid, err := HashPassword(t.Name(), []byte("0123456789abcdef"), DefaultIterations)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	for _, encoded := range []string{
		"",
		"sha256" + strings.TrimPrefix(valid, Scheme),
		"pbkdf2-sha256$1000" + strings.TrimPrefix(valid, Scheme+"$"+strconv.Itoa(DefaultIterations)),
		"pbkdf2-sha256$210000$short" + valid[strings.LastIndex(valid, "$"):],
		"pbkdf2-sha256$210000$MDEyMzQ1Njc4OWFiY2RlZg$short",
	} {
		encoded := encoded
		t.Run(encoded, func(t *testing.T) {
			t.Parallel()

			if _, err := ParsePasswordHash(encoded); err == nil {
				t.Fatalf("ParsePasswordHash(%q) error = nil, want error", encoded)
			}
		})
	}
}
