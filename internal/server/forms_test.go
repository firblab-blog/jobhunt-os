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

func TestParseLimitedFormRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	values := url.Values{"company": {strings.Repeat("a", 32)}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	_, err := parseLimitedForm(rec, req, 8)
	if !errors.Is(err, errFormTooLarge) {
		t.Fatalf("parseLimitedForm() error = %v, want %v", err, errFormTooLarge)
	}
}

func TestFormFieldHelpers(t *testing.T) {
	t.Parallel()

	values := url.Values{
		"company":    {"  Northstar Systems  "},
		"empty":      {"   "},
		"start_date": {"2026-05-06"},
		"bad_date":   {"05/06/2026"},
		"salary":     {" $1,234.50 "},
		"bad_money":  {"12.345"},
		"cents":      {"1099"},
		"bad_cents":  {"10.5"},
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	form, err := parseLimitedForm(rec, req, defaultMaxFormBytes)
	if err != nil {
		t.Fatalf("parseLimitedForm() error = %v", err)
	}

	if got := form.RequiredString("company", "Company"); got != "Northstar Systems" {
		t.Fatalf("RequiredString(company) = %q, want trimmed value", got)
	}
	if got := form.RequiredString("empty", "Empty"); got != "" {
		t.Fatalf("RequiredString(empty) = %q, want empty", got)
	}
	if got := form.errors.Get("empty"); got != "Empty is required." {
		t.Fatalf("empty error = %q", got)
	}

	date, ok := form.OptionalDate("start_date", "Start date")
	if !ok || !date.Equal(time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("OptionalDate(start_date) = %v, %v", date, ok)
	}
	if _, ok := form.OptionalDate("bad_date", "Bad date"); ok {
		t.Fatalf("OptionalDate(bad_date) ok = true, want false")
	}
	if got := form.errors.Get("bad_date"); got != "Bad date must be a valid date." {
		t.Fatalf("bad_date error = %q", got)
	}
	requiredDate, ok := form.RequiredDate("start_date", "Start date")
	if !ok || !requiredDate.Equal(time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("RequiredDate(start_date) = %v, %v", requiredDate, ok)
	}
	if _, ok := form.RequiredDate("empty", "Empty date"); ok {
		t.Fatalf("RequiredDate(empty) ok = true, want false")
	}

	cents, ok := form.OptionalMoneyCents("salary", "Salary")
	if !ok || cents != 123450 {
		t.Fatalf("OptionalMoneyCents(salary) = %d, %v, want 123450, true", cents, ok)
	}
	if _, ok := form.OptionalMoneyCents("bad_money", "Bad money"); ok {
		t.Fatalf("OptionalMoneyCents(bad_money) ok = true, want false")
	}
	if got := form.errors.Get("bad_money"); got != "Bad money must be a valid amount." {
		t.Fatalf("bad_money error = %q", got)
	}

	rawCents, ok := form.OptionalCents("cents", "Cents")
	if !ok || rawCents != 1099 {
		t.Fatalf("OptionalCents(cents) = %d, %v, want 1099, true", rawCents, ok)
	}
	if _, ok := form.OptionalCents("bad_cents", "Bad cents"); ok {
		t.Fatalf("OptionalCents(bad_cents) ok = true, want false")
	}
	if got := form.errors.Get("bad_cents"); got != "Bad cents must be a valid amount." {
		t.Fatalf("bad_cents error = %q", got)
	}

	if !form.errors.Any() {
		t.Fatalf("form errors Any() = false, want true")
	}
}
