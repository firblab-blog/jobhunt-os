package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCSRFTokenGenerationAndValidation(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_800_000_000, 0)
	rec := httptest.NewRecorder()

	token, err := issueCSRFToken(rec, now, false)
	if err != nil {
		t.Fatalf("issueCSRFToken() error = %v", err)
	}
	if token == "" {
		t.Fatalf("token is empty")
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies len = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != csrfCookieName {
		t.Fatalf("cookie name = %q, want %q", cookie.Name, csrfCookieName)
	}
	if cookie.Value != token {
		t.Fatalf("cookie token does not match issued token")
	}
	if cookie.Path != "/" {
		t.Fatalf("cookie Path = %q, want /", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatalf("cookie HttpOnly = false, want true")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Secure {
		t.Fatalf("cookie Secure = true, want false by default")
	}

	field := string(csrfField(token))
	if !strings.Contains(field, `type="hidden"`) ||
		!strings.Contains(field, `name="`+csrfFieldName+`"`) ||
		!strings.Contains(field, `value="`+token+`"`) {
		t.Fatalf("csrfField() = %q", field)
	}

	req := csrfRequest(token, cookie)
	if err := verifyCSRF(req, now.Add(time.Minute)); err != nil {
		t.Fatalf("verifyCSRF() error = %v", err)
	}
}

func TestCSRFRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_800_000_000, 0)
	token, err := newCSRFToken(now)
	if err != nil {
		t.Fatalf("newCSRFToken() error = %v", err)
	}

	req := csrfRequest(token, &http.Cookie{Name: csrfCookieName, Value: token})
	err = verifyCSRF(req, now.Add(csrfTokenTTL+time.Second))
	if !errors.Is(err, errCSRFExpired) {
		t.Fatalf("verifyCSRF() error = %v, want %v", err, errCSRFExpired)
	}
}

func TestCSRFRejectsTamperedToken(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_800_000_000, 0)
	token, err := newCSRFToken(now)
	if err != nil {
		t.Fatalf("newCSRFToken() error = %v", err)
	}
	tampered := tamperToken(token)

	req := csrfRequest(tampered, &http.Cookie{Name: csrfCookieName, Value: tampered})
	err = verifyCSRF(req, now.Add(time.Minute))
	if !errors.Is(err, errCSRFInvalid) {
		t.Fatalf("verifyCSRF() error = %v, want %v", err, errCSRFInvalid)
	}
}

func TestCSRFRejectsMissingCookieOrFormToken(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_800_000_000, 0)
	token, err := newCSRFToken(now)
	if err != nil {
		t.Fatalf("newCSRFToken() error = %v", err)
	}

	reqWithoutCookie := csrfRequest(token, nil)
	if err := verifyCSRF(reqWithoutCookie, now.Add(time.Minute)); !errors.Is(err, errCSRFMissing) {
		t.Fatalf("missing cookie error = %v, want %v", err, errCSRFMissing)
	}

	reqWithoutForm := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	reqWithoutForm.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	if err := reqWithoutForm.ParseForm(); err != nil {
		t.Fatalf("ParseForm() error = %v", err)
	}
	if err := verifyCSRF(reqWithoutForm, now.Add(time.Minute)); !errors.Is(err, errCSRFMissing) {
		t.Fatalf("missing form error = %v, want %v", err, errCSRFMissing)
	}
}

func csrfRequest(token string, cookie *http.Cookie) *http.Request {
	body := url.Values{csrfFieldName: {token}}.Encode()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	_ = req.ParseForm()
	return req
}

func tamperToken(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(parts[0]) == 0 {
		return token + "x"
	}
	if parts[0][0] == 'A' {
		parts[0] = "B" + parts[0][1:]
	} else {
		parts[0] = "A" + parts[0][1:]
	}
	return strings.Join(parts, ".")
}
