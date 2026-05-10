package auth

import (
	"strconv"
	"strings"
	"testing"
)

func TestArgon2idPasswordHashVerify(t *testing.T) {
	t.Parallel()

	password := "correct horse battery staple"
	encoded, err := hashArgon2idPassword(password, []byte("0123456789abcdef"))
	if err != nil {
		t.Fatalf("hashArgon2idPassword() error = %v", err)
	}
	hash, err := ParsePasswordHash(encoded)
	if err != nil {
		t.Fatalf("ParsePasswordHash() error = %v", err)
	}
	if hash.Scheme != Argon2idScheme {
		t.Fatalf("Scheme = %q, want %q", hash.Scheme, Argon2idScheme)
	}

	if !hash.Verify(password) {
		t.Fatalf("Verify(correct password) = false, want true")
	}
	if hash.Verify(password + "/wrong") {
		t.Fatalf("Verify(wrong password) = true, want false")
	}
}

func TestHashPasswordUsesRandomArgon2idSalt(t *testing.T) {
	t.Parallel()

	password := "correct horse battery staple"
	first, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() first error = %v", err)
	}
	second, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() second error = %v", err)
	}

	if first == second {
		t.Fatalf("HashPassword() returned identical hashes with random salts")
	}
	if !strings.HasPrefix(first, Argon2idScheme+"$") {
		t.Fatalf("HashPassword() = %q, want %s prefix", first, Argon2idScheme)
	}
}

func TestPBKDF2PasswordHashVerify(t *testing.T) {
	t.Parallel()

	password := "correct horse battery staple"
	encoded, err := HashPBKDF2Password(password, []byte("0123456789abcdef"), DefaultIterations)
	if err != nil {
		t.Fatalf("HashPBKDF2Password() error = %v", err)
	}
	hash, err := ParsePasswordHash(encoded)
	if err != nil {
		t.Fatalf("ParsePasswordHash() error = %v", err)
	}
	if hash.Scheme != PBKDF2SHA256Scheme {
		t.Fatalf("Scheme = %q, want %q", hash.Scheme, PBKDF2SHA256Scheme)
	}

	if !hash.Verify(password) {
		t.Fatalf("Verify(correct password) = false, want true")
	}
	if hash.Verify(password + "/wrong") {
		t.Fatalf("Verify(wrong password) = true, want false")
	}
}

func TestPasswordPolicy(t *testing.T) {
	t.Parallel()

	for name, password := range map[string]string{
		"minimum": "123456789012345",
		"spaces":  "space rich passphrase with room",
		"long":    strings.Repeat("a", 128),
	} {
		name := name
		password := password
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateLoginPassword(password); err != nil {
				t.Fatalf("ValidateLoginPassword() error = %v", err)
			}
		})
	}

	for name, password := range map[string]string{
		"too short":     "short password",
		"too long":      strings.Repeat("a", MaxPasswordLength+1),
		"not printable": "correct horse\nbattery staple",
	} {
		name := name
		password := password
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateLoginPassword(password); err == nil {
				t.Fatalf("ValidateLoginPassword() error = nil, want error")
			}
		})
	}
}

func TestHashPasswordRejectsWeakParameters(t *testing.T) {
	t.Parallel()

	if _, err := hashArgon2idPassword("correct horse battery staple", []byte("short")); err == nil {
		t.Fatalf("hashArgon2idPassword() with short salt error = nil, want error")
	}
	if _, err := HashPBKDF2Password(t.Name(), []byte("0123456789abcdef"), DefaultIterations-1); err == nil {
		t.Fatalf("HashPBKDF2Password() with low iterations error = nil, want error")
	}
	if _, err := HashPBKDF2Password(t.Name(), []byte("short"), DefaultIterations); err == nil {
		t.Fatalf("HashPBKDF2Password() with short salt error = nil, want error")
	}
}

func TestParsePasswordHashRejectsInvalidHashes(t *testing.T) {
	t.Parallel()

	validArgon2id, err := hashArgon2idPassword("correct horse battery staple", []byte("0123456789abcdef"))
	if err != nil {
		t.Fatalf("hashArgon2idPassword() error = %v", err)
	}
	validPBKDF2, err := HashPBKDF2Password(t.Name(), []byte("0123456789abcdef"), DefaultIterations)
	if err != nil {
		t.Fatalf("HashPBKDF2Password() error = %v", err)
	}
	for _, encoded := range []string{
		"",
		"sha256" + strings.TrimPrefix(validArgon2id, Argon2idScheme),
		"argon2id$v=18$m=19456,t=2,p=1" + validArgon2id[strings.LastIndex(validArgon2id, "$"):],
		"argon2id$v=19$m=1024,t=2,p=1" + validArgon2id[strings.LastIndex(validArgon2id, "$"):],
		"argon2id$v=19$m=19456,t=1,p=1" + validArgon2id[strings.LastIndex(validArgon2id, "$"):],
		"argon2id$v=19$m=19456,t=2,p=0" + validArgon2id[strings.LastIndex(validArgon2id, "$"):],
		"pbkdf2-sha256$1000" + strings.TrimPrefix(validPBKDF2, PBKDF2SHA256Scheme+"$"+strconv.Itoa(DefaultIterations)),
		"pbkdf2-sha256$210000$short" + validPBKDF2[strings.LastIndex(validPBKDF2, "$"):],
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
