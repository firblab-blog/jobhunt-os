package server

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firblab-blog/jobhunt-os/internal/model"
	"github.com/firblab-blog/jobhunt-os/internal/store"
)

func TestHealthz(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	New(nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "ok\n" {
		t.Fatalf("body = %q, want ok", body)
	}
}

func TestHomeRendersDashboard(t *testing.T) {
	t.Parallel()

	rec := request(http.MethodGet, "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"JobHunt OS",
		"Active applications",
		"Northstar Systems",
		`href="/applications"`,
		`href="/documents"`,
		`href="/contacts"`,
		`href="/follow-ups"`,
		`href="/backup"`,
		`href="/applications/new"`,
		`data-status="interviewing"`,
		"Prep system design notes for Northstar Systems.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestHomeRendersEmptyNextActions(t *testing.T) {
	t.Parallel()

	srv := New(nil).(*Server)
	var body bytes.Buffer

	err := srv.templates.ExecuteTemplate(&body, "home.html", dashboardData{
		Metrics: []dashboardMetric{
			{Label: "Active applications", Value: "0"},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTemplate() error = %v", err)
	}
	if got := body.String(); !strings.Contains(got, "No next actions queued.") {
		t.Fatalf("body does not contain empty next actions state")
	}
}

func TestSupportRoutesRenderPages(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"/documents":  "Documents",
		"/contacts":   "Contacts",
		"/follow-ups": "Follow-ups",
		"/backup":     "Backup",
	}
	for target, want := range tests {
		rec := requestWithStore(http.MethodGet, target, nil, &fakeApplicationStore{})

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", target, rec.Code, http.StatusOK)
		}
		if body := rec.Body.String(); !strings.Contains(body, want) {
			t.Fatalf("%s body does not contain %q", target, want)
		}
	}
}

func TestApplicationsIndexRendersList(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	rec := requestWithStore(http.MethodGet, "/applications", nil, &fakeApplicationStore{
		applications: []model.Application{
			{
				ID:        "app_1",
				Company:   "Northstar Systems",
				Role:      "Senior Platform Engineer",
				Status:    model.StatusInterviewing,
				Priority:  model.PriorityHigh,
				UpdatedAt: now,
				NextAction: model.NextAction{
					Summary: "Prep system design notes",
					Due:     &now,
				},
			},
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Northstar Systems",
		"Senior Platform Engineer",
		"Interviewing",
		"High",
		`href="/applications/app_1"`,
		"Prep system design notes",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestApplicationsIndexFiltersList(t *testing.T) {
	t.Parallel()

	rec := requestWithStore(http.MethodGet, "/applications?q=atlas&status=applied", nil, &fakeApplicationStore{
		applications: []model.Application{
			{
				ID:       "app_1",
				Company:  "Northstar Systems",
				Role:     "Senior Platform Engineer",
				Status:   model.StatusInterviewing,
				Priority: model.PriorityHigh,
			},
			{
				ID:       "app_2",
				Company:  "Atlas Cloud",
				Role:     "Staff DevOps Engineer",
				Status:   model.StatusApplied,
				Priority: model.PriorityNormal,
			},
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Atlas Cloud") {
		t.Fatalf("body does not contain filtered application")
	}
	if strings.Contains(body, "Northstar Systems") {
		t.Fatalf("body contains application excluded by filters")
	}
}

func TestDocumentsCreateValidRedirects(t *testing.T) {
	t.Parallel()

	appStore := &fakeApplicationStore{}
	cookie, token := newPageCSRF(t, appStore, "/documents")
	form := url.Values{
		csrfFieldName:   {token},
		"name":          {"Platform resume"},
		"document_type": {string(model.DocumentResume)},
		"storage_path":  {"documents/platform-resume.pdf"},
		"notes":         {"Tailored to platform roles."},
	}
	rec := postFormWithCookie("/documents", form, cookie, appStore)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/documents" {
		t.Fatalf("Location = %q, want /documents", location)
	}
	if len(appStore.createdDocuments) != 1 {
		t.Fatalf("createdDocuments len = %d, want 1", len(appStore.createdDocuments))
	}
}

func TestContactsCreateValidRedirects(t *testing.T) {
	t.Parallel()

	appStore := &fakeApplicationStore{}
	cookie, token := newPageCSRF(t, appStore, "/contacts")
	form := url.Values{
		csrfFieldName:  {token},
		"name":         {"Avery Lee"},
		"organization": {"Northstar Systems"},
		"role":         {"Recruiter"},
		"email":        {"avery@example.test"},
	}
	rec := postFormWithCookie("/contacts", form, cookie, appStore)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/contacts" {
		t.Fatalf("Location = %q, want /contacts", location)
	}
	if len(appStore.createdContacts) != 1 {
		t.Fatalf("createdContacts len = %d, want 1", len(appStore.createdContacts))
	}
}

func TestFollowUpsRenderNextActions(t *testing.T) {
	t.Parallel()

	rec := requestWithStore(http.MethodGet, "/follow-ups", nil, &fakeApplicationStore{
		applications: []model.Application{testApplication()},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"Follow up with recruiter", "Northstar Systems", `href="/applications/app_1"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestExportJSONRendersSnapshot(t *testing.T) {
	t.Parallel()

	rec := requestWithStore(http.MethodGet, "/export.json", nil, &fakeApplicationStore{
		applications: []model.Application{testApplication()},
		documents: []model.Document{
			{ID: "doc_1", Name: "Resume", Type: model.DocumentResume, StoragePath: "resume.pdf"},
		},
		contacts: []model.Contact{
			{ID: "ctc_1", Name: "Avery Lee"},
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	body := rec.Body.String()
	for _, want := range []string{`"version": "1"`, `"applications"`, `"documents"`, `"contacts"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestApplicationsShowRendersFormsAndEmptyStates(t *testing.T) {
	t.Parallel()

	rec := requestWithStore(http.MethodGet, "/applications/app_1", nil, &fakeApplicationStore{
		applications: []model.Application{testApplication()},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`action="/applications/app_1/events"`,
		`action="/applications/app_1/status"`,
		`action="/applications/app_1/documents"`,
		`name="csrf_token"`,
		"No timeline events yet.",
		"No posting PDF attached.",
		"No notes recorded.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestApplicationsShowRendersPostingLinkAndDocuments(t *testing.T) {
	t.Parallel()

	app := testApplication()
	app.PostingURL = "https://jobs.example.com/platform"
	document := model.Document{
		ID:          "doc_1",
		Name:        "Northstar posting",
		Type:        model.DocumentJobPosting,
		StoragePath: "documents/app_1/doc_1.pdf",
		CreatedAt:   time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
	}
	rec := requestWithStore(http.MethodGet, "/applications/app_1", nil, &fakeApplicationStore{
		applications: []model.Application{app},
		appDocuments: []model.ApplicationDocument{
			{ApplicationID: "app_1", Document: document, AttachmentType: model.AttachmentJobPosting},
		},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`href="https://jobs.example.com/platform"`,
		`href="/documents/doc_1/download"`,
		"Northstar posting",
		"Job posting",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
	if strings.Contains(body, "documents/app_1/doc_1.pdf") {
		t.Fatalf("body exposed raw storage path")
	}
}

func TestApplicationsNewRendersCSRF(t *testing.T) {
	t.Parallel()

	rec := requestWithStore(http.MethodGet, "/applications/new", nil, &fakeApplicationStore{})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := len(rec.Result().Cookies()); got == 0 {
		t.Fatalf("cookies len = %d, want CSRF cookie", got)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`name="csrf_token"`,
		`name="company"`,
		`name="role"`,
		`name="status"`,
		`name="priority"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestApplicationsCreateValidRedirectsToDetail(t *testing.T) {
	t.Parallel()

	appStore := &fakeApplicationStore{}
	cookie, token := newFormCSRF(t, appStore)
	form := url.Values{
		csrfFieldName:         {token},
		"company":             {"Northstar Systems"},
		"role":                {"Senior Platform Engineer"},
		"status":              {string(model.StatusApplied)},
		"priority":            {string(model.PriorityHigh)},
		"source":              {"Referral"},
		"location":            {"Remote"},
		"next_action_summary": {"Follow up with recruiter"},
		"next_action_due":     {"2026-05-07"},
		"notes":               {"Private note"},
	}
	rec := postFormWithCookie("/applications", form, cookie, appStore)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/applications/app_created" {
		t.Fatalf("Location = %q, want detail redirect", location)
	}
	if len(appStore.created) != 1 {
		t.Fatalf("created len = %d, want 1", len(appStore.created))
	}
	created := appStore.created[0]
	if created.Company != "Northstar Systems" || created.Role != "Senior Platform Engineer" {
		t.Fatalf("created application = %#v", created)
	}
	if created.NextAction.Due == nil || created.NextAction.Due.Format("2006-01-02") != "2026-05-07" {
		t.Fatalf("created next action due = %v, want 2026-05-07", created.NextAction.Due)
	}
}

func TestApplicationsCreateInvalidRerendersErrors(t *testing.T) {
	t.Parallel()

	appStore := &fakeApplicationStore{}
	cookie, token := newFormCSRF(t, appStore)
	form := url.Values{
		csrfFieldName:     {token},
		"company":         {""},
		"role":            {""},
		"status":          {"not-a-status"},
		"priority":        {string(model.PriorityNormal)},
		"source":          {"Referral"},
		"next_action_due": {"tomorrow"},
	}
	rec := postFormWithCookie("/applications", form, cookie, appStore)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(appStore.created) != 0 {
		t.Fatalf("created len = %d, want 0", len(appStore.created))
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Company is required.",
		"Role is required.",
		"Status must be a valid pipeline state.",
		"Next action due must be a valid date.",
		`value="Referral"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestApplicationsAddEventValidRedirectsToDetail(t *testing.T) {
	t.Parallel()

	appStore := &fakeApplicationStore{
		applications: []model.Application{testApplication()},
	}
	cookie, token := newDetailCSRF(t, appStore, "app_1")
	form := url.Values{
		csrfFieldName: {token},
		"event_type":  {string(model.EventNote)},
		"occurred_at": {"2026-05-06"},
		"summary":     {"Sent a recruiter follow-up."},
		"notes":       {"Mentioned platform role fit."},
	}
	rec := postFormWithCookie("/applications/app_1/events", form, cookie, appStore)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/applications/app_1" {
		t.Fatalf("Location = %q, want detail redirect", location)
	}
	if len(appStore.addedEvents) != 1 {
		t.Fatalf("addedEvents len = %d, want 1", len(appStore.addedEvents))
	}
	added := appStore.addedEvents[0]
	if added.ApplicationID != "app_1" || added.EventType != model.EventNote || added.Summary != "Sent a recruiter follow-up." {
		t.Fatalf("added event = %#v", added)
	}
	if added.OccurredAt.Format("2006-01-02") != "2026-05-06" {
		t.Fatalf("OccurredAt = %v, want 2026-05-06", added.OccurredAt)
	}
}

func TestApplicationsAddEventInvalidRerendersErrors(t *testing.T) {
	t.Parallel()

	appStore := &fakeApplicationStore{
		applications: []model.Application{testApplication()},
	}
	cookie, token := newDetailCSRF(t, appStore, "app_1")
	form := url.Values{
		csrfFieldName: {token},
		"event_type":  {"not-a-type"},
		"occurred_at": {"tomorrow"},
		"summary":     {""},
		"notes":       {"Keep this note visible."},
	}
	rec := postFormWithCookie("/applications/app_1/events", form, cookie, appStore)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(appStore.addedEvents) != 0 {
		t.Fatalf("addedEvents len = %d, want 0", len(appStore.addedEvents))
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Event type must be valid.",
		"Occurred date must be a valid date.",
		"Summary is required.",
		"Keep this note visible.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestApplicationsUpdateStatusAndNextAction(t *testing.T) {
	t.Parallel()

	appStore := &fakeApplicationStore{
		applications: []model.Application{testApplication()},
	}
	cookie, token := newDetailCSRF(t, appStore, "app_1")
	form := url.Values{
		csrfFieldName:         {token},
		"status":              {string(model.StatusInterviewing)},
		"next_action_summary": {"Prep architecture notes"},
		"next_action_due":     {"2026-05-08"},
	}
	rec := postFormWithCookie("/applications/app_1/status", form, cookie, appStore)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/applications/app_1" {
		t.Fatalf("Location = %q, want detail redirect", location)
	}
	if len(appStore.statusUpdates) != 1 {
		t.Fatalf("statusUpdates len = %d, want 1", len(appStore.statusUpdates))
	}
	update := appStore.statusUpdates[0]
	if update.id != "app_1" || update.status != model.StatusInterviewing {
		t.Fatalf("status update = %#v", update)
	}
	if update.nextAction.Summary != "Prep architecture notes" {
		t.Fatalf("NextAction.Summary = %q, want Prep architecture notes", update.nextAction.Summary)
	}
	if update.nextAction.Due == nil || update.nextAction.Due.Format("2006-01-02") != "2026-05-08" {
		t.Fatalf("NextAction.Due = %v, want 2026-05-08", update.nextAction.Due)
	}
}

func TestApplicationsUpdatePostingUploadsPDFAndURL(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	appStore := &fakeApplicationStore{
		applications: []model.Application{testApplication()},
	}
	cookie, token := newDetailCSRF(t, appStore, "app_1")
	form := map[string]string{
		csrfFieldName: token,
		"posting_url": "https://jobs.example.com/platform",
	}
	rec := postMultipartWithCookie(
		"/applications/app_1/documents",
		form,
		"posting.pdf",
		[]byte("%PDF-1.7\nsaved posting\n"),
		cookie,
		appStore,
		dataDir,
	)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/applications/app_1" {
		t.Fatalf("Location = %q, want detail redirect", location)
	}
	updated, err := appStore.GetApplication(context.Background(), "app_1")
	if err != nil {
		t.Fatalf("GetApplication() error = %v", err)
	}
	if updated.PostingURL != "https://jobs.example.com/platform" {
		t.Fatalf("PostingURL = %q", updated.PostingURL)
	}
	if len(appStore.appDocuments) != 1 {
		t.Fatalf("appDocuments len = %d, want 1", len(appStore.appDocuments))
	}
	attached := appStore.appDocuments[0]
	if attached.AttachmentType != model.AttachmentJobPosting || attached.Document.Type != model.DocumentJobPosting {
		t.Fatalf("attached document = %#v", attached)
	}
	if filepath.IsAbs(attached.Document.StoragePath) {
		t.Fatalf("StoragePath = %q, want relative path", attached.Document.StoragePath)
	}
	if !strings.HasPrefix(attached.Document.StoragePath, "documents/app_1/") {
		t.Fatalf("StoragePath = %q, want application document path", attached.Document.StoragePath)
	}
	if _, err := os.Stat(filepath.Join(dataDir, filepath.FromSlash(attached.Document.StoragePath))); err != nil {
		t.Fatalf("uploaded PDF was not saved: %v", err)
	}
}

func TestApplicationsUpdatePostingRejectsNonPDF(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	appStore := &fakeApplicationStore{
		applications: []model.Application{testApplication()},
	}
	cookie, token := newDetailCSRF(t, appStore, "app_1")
	form := map[string]string{
		csrfFieldName: token,
		"posting_url": "https://jobs.example.com/platform",
	}
	rec := postMultipartWithCookie(
		"/applications/app_1/documents",
		form,
		"posting.txt",
		[]byte("not a pdf"),
		cookie,
		appStore,
		dataDir,
	)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(appStore.appDocuments) != 0 {
		t.Fatalf("appDocuments len = %d, want 0", len(appStore.appDocuments))
	}
	if !strings.Contains(rec.Body.String(), "Could not save PDF") {
		t.Fatalf("body does not contain PDF validation error")
	}
}

func TestApplicationDetailPostRequiresCSRF(t *testing.T) {
	t.Parallel()

	for _, target := range []string{
		"/applications/app_1/events",
		"/applications/app_1/status",
	} {
		appStore := &fakeApplicationStore{
			applications: []model.Application{testApplication()},
		}
		form := url.Values{
			"event_type":  {"note"},
			"occurred_at": {"2026-05-06"},
			"summary":     {"Missing CSRF"},
			"status":      {string(model.StatusApplied)},
		}
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		New(appStore).ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want %d", target, rec.Code, http.StatusBadRequest)
		}
		if len(appStore.addedEvents) != 0 || len(appStore.statusUpdates) != 0 {
			t.Fatalf("%s mutated fake store without CSRF", target)
		}
	}
}

func TestApplicationDetailPostNotFound(t *testing.T) {
	t.Parallel()

	for _, target := range []string{
		"/applications/missing/events",
		"/applications/missing/status",
	} {
		appStore := &fakeApplicationStore{
			getErr: store.ErrNotFound,
		}
		cookie, token := newFormCSRF(t, appStore)
		form := url.Values{
			csrfFieldName: {token},
			"event_type":  {string(model.EventNote)},
			"occurred_at": {"2026-05-06"},
			"summary":     {"Missing app"},
			"status":      {string(model.StatusApplied)},
		}
		rec := postFormWithCookie(target, form, cookie, appStore)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", target, rec.Code, http.StatusNotFound)
		}
	}
}

func TestApplicationsShowNotFound(t *testing.T) {
	t.Parallel()

	rec := requestWithStore(http.MethodGet, "/applications/missing", nil, &fakeApplicationStore{
		getErr: store.ErrNotFound,
	})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestNonexistentReturnsNotFound(t *testing.T) {
	t.Parallel()

	rec := request(http.MethodGet, "/nonexistent")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHealthzPostReturnsMethodNotAllowed(t *testing.T) {
	t.Parallel()

	rec := request(http.MethodPost, "/healthz")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestStaticStylesReturnsCSS(t *testing.T) {
	t.Parallel()

	rec := request(http.MethodGet, "/static/styles.css")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/css") {
		t.Fatalf("Content-Type = %q, want text/css", contentType)
	}
	if body := rec.Body.String(); !strings.Contains(body, ":root") {
		t.Fatalf("body does not contain CSS root selector")
	}
}

func TestSecurityHeadersArePresent(t *testing.T) {
	t.Parallel()

	rec := request(http.MethodGet, "/")

	wantHeaders := map[string]string{
		"Content-Security-Policy": "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "same-origin",
	}

	for name, want := range wantHeaders {
		if got := rec.Header().Get(name); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func request(method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()

	New(nil).ServeHTTP(rec, req)

	return rec
}

func requestWithStore(method, target string, body *strings.Reader, appStore store.ApplicationStore) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == nil {
		reader = strings.NewReader("")
	} else {
		reader = body
	}
	req := httptest.NewRequest(method, target, reader)
	rec := httptest.NewRecorder()

	New(appStore).ServeHTTP(rec, req)

	return rec
}

func newFormCSRF(t *testing.T, appStore store.ApplicationStore) (*http.Cookie, string) {
	t.Helper()

	rec := requestWithStore(http.MethodGet, "/applications/new", nil, appStore)
	if rec.Code != http.StatusOK {
		t.Fatalf("new form status = %d, want %d", rec.Code, http.StatusOK)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("new form did not set CSRF cookie")
	}
	token := extractCSRFToken(t, rec.Body.String())
	return cookies[0], token
}

func newDetailCSRF(t *testing.T, appStore store.ApplicationStore, appID string) (*http.Cookie, string) {
	t.Helper()

	rec := requestWithStore(http.MethodGet, "/applications/"+appID, nil, appStore)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("detail did not set CSRF cookie")
	}
	token := extractCSRFToken(t, rec.Body.String())
	return cookies[0], token
}

func newPageCSRF(t *testing.T, appStore store.ApplicationStore, target string) (*http.Cookie, string) {
	t.Helper()

	rec := requestWithStore(http.MethodGet, target, nil, appStore)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s status = %d, want %d; body=%s", target, rec.Code, http.StatusOK, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("%s did not set CSRF cookie", target)
	}
	token := extractCSRFToken(t, rec.Body.String())
	return cookies[0], token
}

func postFormWithCookie(target string, form url.Values, cookie *http.Cookie, appStore store.ApplicationStore) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	New(appStore).ServeHTTP(rec, req)

	return rec
}

func postMultipartWithCookie(target string, fields map[string]string, fileName string, fileContent []byte, cookie *http.Cookie, appStore store.ApplicationStore, dataDir string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		_ = writer.WriteField(name, value)
	}
	if fileName != "" {
		part, _ := writer.CreateFormFile("posting_pdf", fileName)
		_, _ = part.Write(fileContent)
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	NewWithOptions(appStore, Options{DataDir: dataDir}).ServeHTTP(rec, req)

	return rec
}

func extractCSRFToken(t *testing.T, body string) string {
	t.Helper()

	const prefix = `name="csrf_token" value="`
	start := strings.Index(body, prefix)
	if start == -1 {
		t.Fatalf("body does not contain CSRF field")
	}
	start += len(prefix)
	end := strings.Index(body[start:], `"`)
	if end == -1 {
		t.Fatalf("CSRF field value is not closed")
	}
	return body[start : start+end]
}

func testApplication() model.Application {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	return model.Application{
		ID:        "app_1",
		Company:   "Northstar Systems",
		Role:      "Senior Platform Engineer",
		Status:    model.StatusApplied,
		Priority:  model.PriorityHigh,
		CreatedAt: now,
		UpdatedAt: now,
		NextAction: model.NextAction{
			Summary: "Follow up with recruiter",
			Due:     &now,
		},
	}
}

type fakeApplicationStore struct {
	applications     []model.Application
	events           []model.ApplicationEvent
	documents        []model.Document
	appDocuments     []model.ApplicationDocument
	contacts         []model.Contact
	created          []model.Application
	createdDocuments []model.Document
	createdContacts  []model.Contact
	getErr           error
	addedEvents      []model.ApplicationEvent
	statusUpdates    []fakeStatusUpdate
}

type fakeStatusUpdate struct {
	id         string
	status     model.ApplicationStatus
	nextAction model.NextAction
}

func (f *fakeApplicationStore) ListApplications(context.Context) ([]model.Application, error) {
	return append([]model.Application(nil), f.applications...), nil
}

func (f *fakeApplicationStore) GetApplication(_ context.Context, id string) (model.Application, error) {
	if f.getErr != nil {
		return model.Application{}, f.getErr
	}
	for _, app := range f.applications {
		if app.ID == id {
			return app, nil
		}
	}
	for _, app := range f.created {
		if app.ID == id {
			return app, nil
		}
	}
	return model.Application{}, store.ErrNotFound
}

func (f *fakeApplicationStore) ListApplicationEvents(_ context.Context, applicationID string) ([]model.ApplicationEvent, error) {
	var events []model.ApplicationEvent
	for _, event := range f.events {
		if event.ApplicationID == applicationID {
			events = append(events, event)
		}
	}
	return events, nil
}

func (f *fakeApplicationStore) ListApplicationDocuments(_ context.Context, applicationID string) ([]model.ApplicationDocument, error) {
	var documents []model.ApplicationDocument
	for _, document := range f.appDocuments {
		if document.ApplicationID == applicationID {
			documents = append(documents, document)
		}
	}
	return documents, nil
}

func (f *fakeApplicationStore) ListDocuments(context.Context) ([]model.Document, error) {
	return append([]model.Document(nil), f.documents...), nil
}

func (f *fakeApplicationStore) GetDocument(_ context.Context, id string) (model.Document, error) {
	for _, document := range f.documents {
		if document.ID == id {
			return document, nil
		}
	}
	for _, attached := range f.appDocuments {
		if attached.Document.ID == id {
			return attached.Document, nil
		}
	}
	for _, document := range f.createdDocuments {
		if document.ID == id {
			return document, nil
		}
	}
	return model.Document{}, store.ErrNotFound
}

func (f *fakeApplicationStore) CountDocuments(context.Context) (int, error) {
	return len(f.documents), nil
}

func (f *fakeApplicationStore) ListContacts(context.Context) ([]model.Contact, error) {
	return append([]model.Contact(nil), f.contacts...), nil
}

func (f *fakeApplicationStore) CreateApplication(_ context.Context, app model.Application) (model.Application, error) {
	if err := app.ValidateForCreate(); err != nil {
		return model.Application{}, err
	}
	app.ID = "app_created"
	app.CreatedAt = time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	app.UpdatedAt = app.CreatedAt
	f.created = append(f.created, app)
	return app, nil
}

func (f *fakeApplicationStore) UpdateApplicationPostingURL(_ context.Context, id string, postingURL string) (model.Application, error) {
	app, err := f.GetApplication(context.Background(), id)
	if err != nil {
		return model.Application{}, err
	}
	if postingURL != "" && !model.ValidHTTPURL(postingURL) {
		return model.Application{}, errors.New("invalid posting url")
	}
	for i := range f.applications {
		if f.applications[i].ID == id {
			f.applications[i].PostingURL = postingURL
			return f.applications[i], nil
		}
	}
	app.PostingURL = postingURL
	return app, nil
}

func (f *fakeApplicationStore) UpdateApplicationStatusAndNextAction(_ context.Context, id string, status model.ApplicationStatus, nextAction model.NextAction) (model.Application, error) {
	if !status.Valid() {
		return model.Application{}, errors.New("invalid status")
	}
	app, err := f.GetApplication(context.Background(), id)
	if err != nil {
		return model.Application{}, err
	}
	app.Status = status
	app.NextAction = nextAction
	f.statusUpdates = append(f.statusUpdates, fakeStatusUpdate{
		id:         id,
		status:     status,
		nextAction: nextAction,
	})
	return app, nil
}

func (f *fakeApplicationStore) AddApplicationEvent(_ context.Context, event model.ApplicationEvent) (model.ApplicationEvent, error) {
	if !event.EventType.Valid() {
		return model.ApplicationEvent{}, errors.New("invalid event type")
	}
	if event.OccurredAt.IsZero() {
		return model.ApplicationEvent{}, errors.New("occurred at is required")
	}
	if event.Summary == "" {
		return model.ApplicationEvent{}, errors.New("summary is required")
	}
	if _, err := f.GetApplication(context.Background(), event.ApplicationID); err != nil {
		return model.ApplicationEvent{}, err
	}
	event.ID = "evt_added"
	event.CreatedAt = time.Date(2026, 5, 6, 12, 30, 0, 0, time.UTC)
	f.addedEvents = append(f.addedEvents, event)
	return event, nil
}

func (f *fakeApplicationStore) CreateDocument(_ context.Context, document model.Document) (model.Document, error) {
	if err := document.ValidateForCreate(); err != nil {
		return model.Document{}, err
	}
	document.ID = "doc_created"
	document.CreatedAt = time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	document.UpdatedAt = document.CreatedAt
	f.createdDocuments = append(f.createdDocuments, document)
	return document, nil
}

func (f *fakeApplicationStore) AttachDocumentToApplication(_ context.Context, applicationID string, document model.Document, attachmentType model.AttachmentType, notes string) (model.ApplicationDocument, error) {
	if _, err := f.GetApplication(context.Background(), applicationID); err != nil {
		return model.ApplicationDocument{}, err
	}
	if err := document.ValidateForCreate(); err != nil {
		return model.ApplicationDocument{}, err
	}
	if !attachmentType.Valid() {
		return model.ApplicationDocument{}, errors.New("invalid attachment type")
	}
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	if document.CreatedAt.IsZero() {
		document.CreatedAt = now
	}
	if document.UpdatedAt.IsZero() {
		document.UpdatedAt = now
	}
	attached := model.ApplicationDocument{
		ApplicationID:  applicationID,
		Document:       document,
		AttachmentType: attachmentType,
		Notes:          notes,
		CreatedAt:      now,
	}
	f.createdDocuments = append(f.createdDocuments, document)
	f.appDocuments = append(f.appDocuments, attached)
	return attached, nil
}

func (f *fakeApplicationStore) CreateContact(_ context.Context, contact model.Contact) (model.Contact, error) {
	if err := contact.ValidateForCreate(); err != nil {
		return model.Contact{}, err
	}
	contact.ID = "ctc_created"
	contact.CreatedAt = time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	contact.UpdatedAt = contact.CreatedAt
	f.createdContacts = append(f.createdContacts, contact)
	return contact, nil
}
