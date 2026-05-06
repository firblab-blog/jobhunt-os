package server

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultMaxFormBytes int64 = 1 << 20

var errFormTooLarge = errors.New("form body too large")

type formData struct {
	values url.Values
	errors formErrors
}

type formErrors map[string]string

func parseLimitedForm(w http.ResponseWriter, r *http.Request, maxBytes int64) (*formData, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxFormBytes
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseForm(); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return nil, errFormTooLarge
		}
		return nil, err
	}

	return &formData{
		values: trimmedValues(r.PostForm),
		errors: formErrors{},
	}, nil
}

func (f *formData) Value(name string) string {
	if f == nil {
		return ""
	}
	return f.values.Get(name)
}

func (f *formData) RequiredString(name, label string) string {
	value := f.Value(name)
	if value == "" {
		f.errors.Add(name, label+" is required.")
	}
	return value
}

func (f *formData) OptionalDate(name, label string) (time.Time, bool) {
	value := f.Value(name)
	if value == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		f.errors.Add(name, label+" must be a valid date.")
		return time.Time{}, false
	}

	return parsed, true
}

func (f *formData) RequiredDate(name, label string) (time.Time, bool) {
	value := f.RequiredString(name, label)
	if value == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		f.errors.Add(name, label+" must be a valid date.")
		return time.Time{}, false
	}

	return parsed, true
}

func (f *formData) OptionalMoneyCents(name, label string) (int64, bool) {
	value := f.Value(name)
	if value == "" {
		return 0, false
	}

	cents, err := parseMoneyCents(value)
	if err != nil {
		f.errors.Add(name, label+" must be a valid amount.")
		return 0, false
	}

	return cents, true
}

func (f *formData) OptionalCents(name, label string) (int64, bool) {
	value := f.Value(name)
	if value == "" {
		return 0, false
	}

	cents, err := strconv.ParseInt(value, 10, 64)
	if err != nil || cents < 0 {
		f.errors.Add(name, label+" must be a valid amount.")
		return 0, false
	}

	return cents, true
}

func (e formErrors) Add(field, message string) {
	if e == nil {
		return
	}
	if _, exists := e[field]; !exists {
		e[field] = message
	}
}

func (e formErrors) Get(field string) string {
	if e == nil {
		return ""
	}
	return e[field]
}

func (e formErrors) Any() bool {
	return len(e) > 0
}

func trimmedValues(values url.Values) url.Values {
	trimmed := make(url.Values, len(values))
	for name, fieldValues := range values {
		for _, value := range fieldValues {
			trimmed.Add(name, strings.TrimSpace(value))
		}
	}
	return trimmed
}

func parseMoneyCents(value string) (int64, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "$")
	value = strings.ReplaceAll(value, ",", "")
	if value == "" || strings.HasPrefix(value, "-") {
		return 0, fmt.Errorf("invalid money value")
	}

	dollarsPart, centsPart, ok := strings.Cut(value, ".")
	if dollarsPart == "" {
		dollarsPart = "0"
	}
	if !ok {
		centsPart = ""
	}
	if len(centsPart) > 2 {
		return 0, fmt.Errorf("invalid cents precision")
	}
	if !digitsOnly(dollarsPart) || (centsPart != "" && !digitsOnly(centsPart)) {
		return 0, fmt.Errorf("invalid money digits")
	}

	dollars, err := strconv.ParseInt(dollarsPart, 10, 64)
	if err != nil {
		return 0, err
	}
	if dollars > math.MaxInt64/100 {
		return 0, fmt.Errorf("money value too large")
	}

	for len(centsPart) < 2 {
		centsPart += "0"
	}
	cents, err := strconv.ParseInt(centsPart, 10, 64)
	if err != nil {
		return 0, err
	}

	return dollars*100 + cents, nil
}

func digitsOnly(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return value != ""
}
