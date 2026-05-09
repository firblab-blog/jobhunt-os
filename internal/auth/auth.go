package auth

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	Scheme            = "pbkdf2-sha256"
	DefaultIterations = 210000
	SaltBytes         = 16
	KeyBytes          = 32
)

var (
	ErrInvalidHash = errors.New("invalid password hash")
)

type PasswordHash struct {
	Iterations int
	Salt       []byte
	Digest     []byte
}

func ParsePasswordHash(encoded string) (PasswordHash, error) {
	parts := strings.Split(strings.TrimSpace(encoded), "$")
	if len(parts) != 4 || parts[0] != Scheme {
		return PasswordHash{}, ErrInvalidHash
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < DefaultIterations {
		return PasswordHash{}, fmt.Errorf("%w: iterations must be at least %d", ErrInvalidHash, DefaultIterations)
	}

	salt, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < SaltBytes {
		return PasswordHash{}, fmt.Errorf("%w: salt must be base64url and at least %d bytes", ErrInvalidHash, SaltBytes)
	}

	digest, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil || len(digest) != KeyBytes {
		return PasswordHash{}, fmt.Errorf("%w: digest must be base64url and %d bytes", ErrInvalidHash, KeyBytes)
	}

	return PasswordHash{
		Iterations: iterations,
		Salt:       salt,
		Digest:     digest,
	}, nil
}

func HashPassword(password string, salt []byte, iterations int) (string, error) {
	if iterations < DefaultIterations {
		return "", fmt.Errorf("%w: iterations must be at least %d", ErrInvalidHash, DefaultIterations)
	}
	if len(salt) < SaltBytes {
		return "", fmt.Errorf("%w: salt must be at least %d bytes", ErrInvalidHash, SaltBytes)
	}

	digest, err := pbkdf2.Key(sha256.New, password, salt, iterations, KeyBytes)
	if err != nil {
		return "", err
	}
	return Scheme + "$" + strconv.Itoa(iterations) + "$" +
		base64.RawURLEncoding.EncodeToString(salt) + "$" +
		base64.RawURLEncoding.EncodeToString(digest), nil
}

func (h PasswordHash) Verify(password string) bool {
	if h.Iterations < DefaultIterations || len(h.Salt) < SaltBytes || len(h.Digest) != KeyBytes {
		return false
	}

	digest, err := pbkdf2.Key(sha256.New, password, h.Salt, h.Iterations, len(h.Digest))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(digest, h.Digest) == 1
}
