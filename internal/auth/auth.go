package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	Scheme              = Argon2idScheme
	Argon2idScheme      = "argon2id"
	PBKDF2SHA256Scheme  = "pbkdf2-sha256"
	Argon2idVersion     = argon2.Version
	Argon2idMemoryKiB   = 19456
	Argon2idTime        = 2
	Argon2idParallelism = 1
	DefaultIterations   = 210000
	SaltBytes           = 16
	KeyBytes            = 32
	MinPasswordLength   = 15
	MaxPasswordLength   = 1024
)

var (
	ErrInvalidHash  = errors.New("invalid password hash")
	ErrWeakPassword = errors.New("password does not meet policy")
)

type PasswordHash struct {
	Scheme      string
	Version     int
	MemoryKiB   uint32
	Time        uint32
	Parallelism uint8
	Iterations  int
	Salt        []byte
	Digest      []byte
}

func ParsePasswordHash(encoded string) (PasswordHash, error) {
	parts := strings.Split(strings.TrimSpace(encoded), "$")
	if len(parts) == 0 {
		return PasswordHash{}, ErrInvalidHash
	}

	switch parts[0] {
	case Argon2idScheme:
		return parseArgon2idHash(parts)
	case PBKDF2SHA256Scheme:
		return parsePBKDF2Hash(parts)
	default:
		return PasswordHash{}, ErrInvalidHash
	}
}

func parseArgon2idHash(parts []string) (PasswordHash, error) {
	if len(parts) != 5 || parts[1] != "v=19" {
		return PasswordHash{}, ErrInvalidHash
	}

	memory, timeCost, parallelism, err := parseArgon2idParams(parts[2])
	if err != nil {
		return PasswordHash{}, err
	}
	if memory < Argon2idMemoryKiB {
		return PasswordHash{}, fmt.Errorf("%w: memory must be at least %d KiB", ErrInvalidHash, Argon2idMemoryKiB)
	}
	if timeCost < Argon2idTime {
		return PasswordHash{}, fmt.Errorf("%w: time cost must be at least %d", ErrInvalidHash, Argon2idTime)
	}
	if parallelism < Argon2idParallelism {
		return PasswordHash{}, fmt.Errorf("%w: parallelism must be at least %d", ErrInvalidHash, Argon2idParallelism)
	}

	salt, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil || len(salt) < SaltBytes {
		return PasswordHash{}, fmt.Errorf("%w: salt must be base64url and at least %d bytes", ErrInvalidHash, SaltBytes)
	}

	digest, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil || len(digest) != KeyBytes {
		return PasswordHash{}, fmt.Errorf("%w: digest must be base64url and %d bytes", ErrInvalidHash, KeyBytes)
	}

	return PasswordHash{
		Scheme:      Argon2idScheme,
		Version:     Argon2idVersion,
		MemoryKiB:   memory,
		Time:        timeCost,
		Parallelism: parallelism,
		Salt:        salt,
		Digest:      digest,
	}, nil
}

func parseArgon2idParams(encoded string) (uint32, uint32, uint8, error) {
	var memory uint64
	var timeCost uint64
	var parallelism uint64
	seen := map[string]bool{}

	for _, part := range strings.Split(encoded, ",") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return 0, 0, 0, ErrInvalidHash
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return 0, 0, 0, ErrInvalidHash
		}
		switch key {
		case "m":
			memory = parsed
		case "t":
			timeCost = parsed
		case "p":
			if parsed > 255 {
				return 0, 0, 0, ErrInvalidHash
			}
			parallelism = parsed
		default:
			return 0, 0, 0, ErrInvalidHash
		}
		seen[key] = true
	}

	if !seen["m"] || !seen["t"] || !seen["p"] {
		return 0, 0, 0, ErrInvalidHash
	}

	return uint32(memory), uint32(timeCost), uint8(parallelism), nil
}

func parsePBKDF2Hash(parts []string) (PasswordHash, error) {
	if len(parts) != 4 {
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
		Scheme:     PBKDF2SHA256Scheme,
		Iterations: iterations,
		Salt:       salt,
		Digest:     digest,
	}, nil
}

func ValidateLoginPassword(password string) error {
	if !utf8.ValidString(password) {
		return fmt.Errorf("%w: password must be valid UTF-8", ErrWeakPassword)
	}

	length := utf8.RuneCountInString(password)
	if length < MinPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters", ErrWeakPassword, MinPasswordLength)
	}
	if length > MaxPasswordLength {
		return fmt.Errorf("%w: password must be at most %d characters", ErrWeakPassword, MaxPasswordLength)
	}

	for _, r := range password {
		if !unicode.IsPrint(r) {
			return fmt.Errorf("%w: password must contain only printable characters", ErrWeakPassword)
		}
	}

	return nil
}

func HashPassword(password string) (string, error) {
	if err := ValidateLoginPassword(password); err != nil {
		return "", err
	}

	salt, err := randomSalt()
	if err != nil {
		return "", err
	}

	return hashArgon2idPassword(password, salt)
}

func HashArgon2idPassword(password string) (string, error) {
	return HashPassword(password)
}

func randomSalt() ([]byte, error) {
	salt := make([]byte, SaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}

func hashArgon2idPassword(password string, salt []byte) (string, error) {
	if err := ValidateLoginPassword(password); err != nil {
		return "", err
	}
	if len(salt) < SaltBytes {
		return "", fmt.Errorf("%w: salt must be at least %d bytes", ErrInvalidHash, SaltBytes)
	}

	digest := argon2.IDKey([]byte(password), salt, Argon2idTime, Argon2idMemoryKiB, Argon2idParallelism, KeyBytes)
	return Argon2idScheme + "$v=19$m=" + strconv.FormatUint(uint64(Argon2idMemoryKiB), 10) +
		",t=" + strconv.FormatUint(uint64(Argon2idTime), 10) +
		",p=" + strconv.FormatUint(uint64(Argon2idParallelism), 10) + "$" +
		base64.RawURLEncoding.EncodeToString(salt) + "$" +
		base64.RawURLEncoding.EncodeToString(digest), nil
}

func HashPBKDF2Password(password string, salt []byte, iterations int) (string, error) {
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
	return PBKDF2SHA256Scheme + "$" + strconv.Itoa(iterations) + "$" +
		base64.RawURLEncoding.EncodeToString(salt) + "$" +
		base64.RawURLEncoding.EncodeToString(digest), nil
}

func (h PasswordHash) Verify(password string) bool {
	switch h.Scheme {
	case Argon2idScheme:
		return h.verifyArgon2id(password)
	case PBKDF2SHA256Scheme:
		return h.verifyPBKDF2(password)
	default:
		return false
	}
}

func (h PasswordHash) verifyArgon2id(password string) bool {
	if h.Version != Argon2idVersion ||
		h.MemoryKiB < Argon2idMemoryKiB ||
		h.Time < Argon2idTime ||
		h.Parallelism < Argon2idParallelism ||
		len(h.Salt) < SaltBytes ||
		len(h.Digest) != KeyBytes {
		return false
	}

	digest := argon2.IDKey([]byte(password), h.Salt, h.Time, h.MemoryKiB, h.Parallelism, uint32(len(h.Digest)))
	return subtle.ConstantTimeCompare(digest, h.Digest) == 1
}

func (h PasswordHash) verifyPBKDF2(password string) bool {
	if h.Iterations < DefaultIterations || len(h.Salt) < SaltBytes || len(h.Digest) != KeyBytes {
		return false
	}

	digest, err := pbkdf2.Key(sha256.New, password, h.Salt, h.Iterations, len(h.Digest))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(digest, h.Digest) == 1
}
