package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	csrfCookieName = "jobhunt_csrf"
	csrfFieldName  = "csrf_token"

	csrfNonceBytes = 32
	csrfTokenTTL   = 2 * time.Hour
)

var (
	errCSRFExpired = errors.New("csrf token expired")
	errCSRFInvalid = errors.New("csrf token invalid")
	errCSRFMissing = errors.New("csrf token missing")

	csrfSecret = mustCSRFSecret()
)

func mustCSRFSecret() []byte {
	secret := make([]byte, sha256.Size)
	if _, err := rand.Read(secret); err != nil {
		panic(err)
	}
	return secret
}

func issueCSRFToken(w http.ResponseWriter, now time.Time) (string, error) {
	token, err := newCSRFToken(now)
	if err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		Expires:  now.Add(csrfTokenTTL),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return token, nil
}

func csrfField(token string) template.HTML {
	return template.HTML(`<input type="hidden" name="` + csrfFieldName + `" value="` + template.HTMLEscapeString(token) + `">`)
}

func verifyCSRF(r *http.Request, now time.Time) error {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return errCSRFMissing
	}

	formToken := strings.TrimSpace(r.PostForm.Get(csrfFieldName))
	if formToken == "" {
		return errCSRFMissing
	}
	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(formToken)) != 1 {
		return errCSRFInvalid
	}

	return validateCSRFToken(cookie.Value, now)
}

func newCSRFToken(now time.Time) (string, error) {
	nonce := make([]byte, csrfNonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	noncePart := base64.RawURLEncoding.EncodeToString(nonce)
	expiryPart := strconv.FormatInt(now.Add(csrfTokenTTL).Unix(), 10)
	macPart := base64.RawURLEncoding.EncodeToString(signCSRFToken(noncePart, expiryPart))

	return noncePart + "." + expiryPart + "." + macPart, nil
}

func validateCSRFToken(token string, now time.Time) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errCSRFInvalid
	}

	nonce, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(nonce) != csrfNonceBytes {
		return errCSRFInvalid
	}

	expiresUnix, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return errCSRFInvalid
	}
	if !now.Before(time.Unix(expiresUnix, 0)) {
		return errCSRFExpired
	}

	mac, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(mac) != sha256.Size {
		return errCSRFInvalid
	}

	expectedMAC := signCSRFToken(parts[0], parts[1])
	if subtle.ConstantTimeCompare(mac, expectedMAC) != 1 {
		return errCSRFInvalid
	}

	return nil
}

func signCSRFToken(noncePart, expiryPart string) []byte {
	mac := hmac.New(sha256.New, csrfSecret)
	_, _ = mac.Write([]byte(noncePart))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(expiryPart))
	return mac.Sum(nil)
}
