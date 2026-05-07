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

func TestHomeRendersEmptyDashboardStates(t *testing.T) {
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
	got := body.String()
	if !strings.Contains(got, "No next actions due today.") {
		t.Fatalf("body does not contain empty next actions state")
	}
	if !strings.Contains(got, "No active applications yet.") {
		t.Fatalf("body does not contain empty application state")
	}
}

func TestHomeRendersEmptyStoreDashboardStats(t *testing.T) {
	t.Parallel()

	rec := requestWithStore(http.MethodGet, "/", nil, &fakeApplicationStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"No pipeline data yet.",
		"Add an active opportunity to start building the follow-up queue.",
		"Priority mix will appear once there is active work.",
		"No active opportunities to age yet.",
		"No tracked application activity this week yet.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
	if strings.Contains(body, "dashboard-pipeline-segment") {
		t.Fatalf("empty dashboard rendered pipeline count segments")
	}
}

func TestHomeRendersThisWeekMomentumStats(t *testing.T) {
	t.Parallel()

	srv := New(nil).(*Server)
	var body bytes.Buffer

	err := srv.templates.ExecuteTemplate(&body, "home.html", dashboardData{
		Stats: dashboardStats{
			ThisWeekActivity: dashboardThisWeekActivity{
				CreatedApplications: 3,
				UpdatedApplications: 4,
				Events:              2,
				Total:               9,
			},
		},
	})
	if err != nil {
		t.Fatalf("ExecuteTemplate() error = %v", err)
	}
	got := body.String()
	for _, want := range []string{
		"This week",
		"Momentum",
		"New applications",
		"Application updates",
		"Timeline events",
		"9 tracked updates this week.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestHomeRendersThemeFromCookie(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		themeDark:  `<html lang="en" data-theme="dark">`,
		themeLight: `<html lang="en" data-theme="light">`,
		"midnight": `<html lang="en" data-theme="system">`,
	}
	for cookieValue, want := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: themeCookieName, Value: cookieValue})
		rec := httptest.NewRecorder()

		New(nil).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if body := rec.Body.String(); !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestThemeFromRequestParsesCookie(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":        themeSystem,
		"system":  themeSystem,
		" light ": themeLight,
		"DARK":    themeDark,
		"sepia":   themeSystem,
	}
	for cookieValue, want := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if cookieValue != "" {
			req.AddCookie(&http.Cookie{Name: themeCookieName, Value: cookieValue})
		}
		if got := themeFromRequest(req); got != want {
			t.Fatalf("themeFromRequest(%q) = %q, want %q", cookieValue, got, want)
		}
	}
}

func TestThemeUpdateSetsCookieAndRedirects(t *testing.T) {
	t.Parallel()

	rec := request(http.MethodGet, "/theme?theme=dark&return_to="+url.QueryEscape("/applications?q=atlas&status=applied"))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if location := rec.Header().Get("Location"); location != "/applications?q=atlas&status=applied" {
		t.Fatalf("Location = %q, want /applications?q=atlas&status=applied", location)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies len = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != themeCookieName || cookie.Value != themeDark {
		t.Fatalf("theme cookie = %s:%s, want %s:%s", cookie.Name, cookie.Value, themeCookieName, themeDark)
	}
	if cookie.Path != "/" || cookie.MaxAge <= 0 || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("theme cookie attributes = %#v, want persistent root HttpOnly SameSite=Lax cookie", cookie)
	}
}

func TestThemeControlEscapesReturnToWithMultipleQueryParams(t *testing.T) {
	t.Parallel()

	rec := requestWithStore(http.MethodGet, "/applications?q=atlas&status=applied", nil, &fakeApplicationStore{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	escapedReturnTo := url.QueryEscape("/applications?q=atlas&status=applied")
	body := rec.Body.String()
	for _, theme := range []string{themeSystem, themeLight, themeDark} {
		want := `/theme?theme=` + theme + `&amp;return_to=` + escapedReturnTo
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestThemeUpdateInvalidValueFallsBackToSystem(t *testing.T) {
	t.Parallel()

	rec := request(http.MethodGet, "/theme?theme=neon&return_to=/")

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies len = %d, want 1", len(cookies))
	}
	if got := cookies[0].Value; got != themeSystem {
		t.Fatalf("theme cookie value = %q, want %q", got, themeSystem)
	}
}

func TestThemeUpdateUsesSafeRedirectTargets(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"/theme?theme=light&return_to=/contacts":                  "/contacts",
		"/theme?theme=light&return_to=https://evil.example/phish": "/",
		"/theme?theme=light&return_to=//evil.example/phish":       "/",
		"/theme?theme=light&return_to=/theme%3Ftheme%3Ddark":      "/",
	}
	for target, want := range tests {
		rec := request(http.MethodGet, target)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%s status = %d, want %d", target, rec.Code, http.StatusSeeOther)
		}
		if location := rec.Header().Get("Location"); location != want {
			t.Fatalf("%s Location = %q, want %q", target, location, want)
		}
	}
}

func TestThemeUpdateFallsBackToSameOriginReferer(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/theme?theme=light", nil)
	req.Header.Set("Referer", "http://example.com/documents?type=resume")
	rec := httptest.NewRecorder()

	New(nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if location := rec.Header().Get("Location"); location != "/documents?type=resume" {
		t.Fatalf("Location = %q, want /documents?type=resume", location)
	}
}

func TestHomeRendersDashboardFromStore(t *testing.T) {
	t.Parallel()

	due := time.Now()
	appStore := &fakeApplicationStore{
		applications: []model.Application{
			{
				ID:        "app_1",
				Company:   "Northstar Systems",
				Role:      "Senior Platform Engineer",
				Status:    model.StatusApplied,
				Priority:  model.PriorityHigh,
				UpdatedAt: due,
				NextAction: model.NextAction{
					Summary: "Follow up with recruiter",
					Due:     &due,
				},
			},
		},
	}

	rec := requestWithStore(http.MethodGet, "/", nil, appStore)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Follow up with recruiter for Northstar Systems.",
		`href="/applications/app_1"`,
		"High priority",
		"Due today",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
}

func TestDashboardQueuePrioritizesActiveWork(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	overdue := now.AddDate(0, 0, -1)
	future := now.AddDate(0, 0, 3)
	items := dashboardQueue([]model.Application{
		{
			ID:        "app_recent",
			Company:   "Recent Co",
			Role:      "Platform Engineer",
			Status:    model.StatusApplied,
			Priority:  model.PriorityHigh,
			UpdatedAt: now,
			NextAction: model.NextAction{
				Summary: "Send note",
			},
		},
		{
			ID:        "app_overdue",
			Company:   "Overdue Co",
			Role:      "Staff Engineer",
			Status:    model.StatusProspect,
			Priority:  model.PriorityLow,
			UpdatedAt: now.Add(-time.Hour),
			NextAction: model.NextAction{
				Summary: "Finish draft",
				Due:     &overdue,
			},
		},
		{
			ID:        "app_future",
			Company:   "Future Co",
			Role:      "Infra Lead",
			Status:    model.StatusInterviewing,
			Priority:  model.PriorityNormal,
			UpdatedAt: now.Add(-2 * time.Hour),
			NextAction: model.NextAction{
				Summary: "Prep interview",
				Due:     &future,
			},
		},
		{
			ID:        "app_archived",
			Company:   "Archived Co",
			Role:      "Developer",
			Status:    model.StatusArchived,
			Priority:  model.PriorityHigh,
			UpdatedAt: now.Add(time.Hour),
		},
	}, now, 5)

	if len(items) != 3 {
		t.Fatalf("dashboardQueue() len = %d, want 3", len(items))
	}
	if got := items[0].ID; got != "app_overdue" {
		t.Fatalf("first queue item = %q, want app_overdue", got)
	}
	if got := items[0].QueueLabel; got != "Overdue" {
		t.Fatalf("first queue label = %q, want Overdue", got)
	}
}

func TestDashboardStatsForCalculatesTrackingCounts(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 7, 16, 0, 0, 0, time.UTC)
	createdThisWeek := time.Date(2026, 5, 4, 14, 0, 0, 0, time.UTC)
	updatedThisWeek := time.Date(2026, 5, 5, 14, 0, 0, 0, time.UTC)
	updatedToday := time.Date(2026, 5, 7, 14, 0, 0, 0, time.UTC)
	lastWeek := time.Date(2026, 5, 1, 14, 0, 0, 0, time.UTC)
	overdue := time.Date(2026, 5, 6, 14, 0, 0, 0, time.UTC)
	dueToday := time.Date(2026, 5, 7, 14, 0, 0, 0, time.UTC)
	upcoming := time.Date(2026, 5, 10, 14, 0, 0, 0, time.UTC)
	stale := now.AddDate(0, 0, -staleActiveDays)

	stats := dashboardStatsFor([]model.Application{
		{
			ID:        "app_prospect",
			Company:   "Prospect Co",
			Role:      "Platform Engineer",
			Status:    model.StatusProspect,
			Priority:  model.PriorityHigh,
			CreatedAt: createdThisWeek,
			UpdatedAt: updatedThisWeek,
			NextAction: model.NextAction{
				Summary: "Finish application draft",
				Due:     &overdue,
			},
		},
		{
			ID:        "app_applied",
			Company:   "Applied Co",
			Role:      "Staff Engineer",
			Status:    model.StatusApplied,
			Priority:  model.PriorityNormal,
			CreatedAt: lastWeek,
			UpdatedAt: updatedToday,
			NextAction: model.NextAction{
				Summary: "Follow up with recruiter",
				Due:     &dueToday,
			},
		},
		{
			ID:        "app_interviewing",
			Company:   "Interview Co",
			Role:      "Infrastructure Lead",
			Status:    model.StatusInterviewing,
			Priority:  model.PriorityLow,
			CreatedAt: lastWeek,
			UpdatedAt: stale,
			NextAction: model.NextAction{
				Summary: "Prep panel interview",
				Due:     &upcoming,
			},
		},
		{
			ID:        "app_offer",
			Company:   "Offer Co",
			Role:      "Engineering Manager",
			Status:    model.StatusOffer,
			Priority:  model.PriorityNormal,
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Hour),
		},
		{
			ID:        "app_rejected",
			Company:   "Rejected Co",
			Role:      "Developer",
			Status:    model.StatusRejected,
			Priority:  model.PriorityHigh,
			CreatedAt: updatedThisWeek,
			UpdatedAt: updatedToday,
		},
	}, []model.ApplicationEvent{
		{
			ID:         "evt_this_week",
			OccurredAt: updatedThisWeek,
		},
		{
			ID:         "evt_last_week",
			OccurredAt: lastWeek,
		},
		{
			ID:         "evt_future",
			OccurredAt: now.Add(time.Hour),
		},
	}, now)

	wantPipelineCounts := map[model.ApplicationStatus]int{
		model.StatusProspect:     1,
		model.StatusApplied:      1,
		model.StatusInterviewing: 1,
		model.StatusOffer:        1,
		model.StatusRejected:     1,
		model.StatusAccepted:     0,
		model.StatusDeclined:     0,
		model.StatusWithdrawn:    0,
		model.StatusArchived:     0,
	}
	for status, want := range wantPipelineCounts {
		if got := dashboardPipelineCount(t, stats.PipelineCounts, status); got != want {
			t.Fatalf("pipeline count %q = %d, want %d", status, got, want)
		}
	}

	if got, want := len(stats.PipelineCounts), len(applicationStatusOptions()); got != want {
		t.Fatalf("pipeline count item len = %d, want %d", got, want)
	}
	if stats.FollowUpHealth.Overdue != 1 || stats.FollowUpHealth.DueToday != 1 ||
		stats.FollowUpHealth.Upcoming != 1 || stats.FollowUpHealth.Unscheduled != 0 ||
		stats.FollowUpHealth.NoNextAction != 1 {
		t.Fatalf("follow-up health = %#v, want overdue=1 today=1 upcoming=1 unscheduled=0 noNextAction=1", stats.FollowUpHealth)
	}
	if stats.PriorityMix.High != 1 || stats.PriorityMix.Normal != 2 || stats.PriorityMix.Low != 1 {
		t.Fatalf("priority mix = %#v, want high=1 normal=2 low=1", stats.PriorityMix)
	}
	if stats.ThisWeekActivity.CreatedApplications != 3 ||
		stats.ThisWeekActivity.UpdatedApplications != 3 ||
		stats.ThisWeekActivity.Events != 1 ||
		stats.ThisWeekActivity.Total != 7 {
		t.Fatalf("this-week activity = %#v, want created=3 updated=3 events=1 total=7", stats.ThisWeekActivity)
	}
	if stats.StaleActiveApplications != 1 {
		t.Fatalf("stale active applications = %d, want 1", stats.StaleActiveApplications)
	}
	if stats.TotalApplications != 5 || stats.ActiveApplications != 4 {
		t.Fatalf("application totals = total:%d active:%d, want total=5 active=4", stats.TotalApplications, stats.ActiveApplications)
	}
}

func TestDashboardStatsForKeepsUnscheduledActionsSeparate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 7, 16, 0, 0, 0, time.UTC)
	stats := dashboardStatsFor([]model.Application{
		{
			ID:        "app_unscheduled",
			Company:   "Northstar Systems",
			Role:      "Senior Platform Engineer",
			Status:    model.StatusApplied,
			Priority:  model.PriorityNormal,
			CreatedAt: now,
			UpdatedAt: now,
			NextAction: model.NextAction{
				Summary: "Follow up with recruiter",
			},
		},
	}, nil, now)

	if stats.FollowUpHealth.Unscheduled != 1 {
		t.Fatalf("unscheduled follow-up count = %d, want 1", stats.FollowUpHealth.Unscheduled)
	}
	if stats.FollowUpHealth.Upcoming != 0 {
		t.Fatalf("upcoming follow-up count = %d, want 0", stats.FollowUpHealth.Upcoming)
	}
}

func dashboardPipelineCount(t *testing.T, counts []dashboardStatCount, status model.ApplicationStatus) int {
	t.Helper()
	for _, count := range counts {
		if count.Key == string(status) {
			return count.Count
		}
	}
	t.Fatalf("pipeline count for %q not found", status)
	return 0
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

	dataDir := t.TempDir()
	appStore := &fakeApplicationStore{}
	cookie, token := newPageCSRF(t, appStore, "/documents")
	form := map[string]string{
		csrfFieldName:   token,
		"name":          "Platform resume",
		"document_type": string(model.DocumentResume),
		"notes":         "Tailored to platform roles.",
	}
	rec := postMultipartFileWithCookie(
		"/documents",
		form,
		"document_file",
		"platform-resume.pdf",
		[]byte("%PDF-1.7\nresume\n"),
		cookie,
		appStore,
		dataDir,
	)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/documents" {
		t.Fatalf("Location = %q, want /documents", location)
	}
	if len(appStore.createdDocuments) != 1 {
		t.Fatalf("createdDocuments len = %d, want 1", len(appStore.createdDocuments))
	}
	document := appStore.createdDocuments[0]
	if document.Type != model.DocumentResume || document.Name != "Platform resume" {
		t.Fatalf("created document = %#v", document)
	}
	if filepath.IsAbs(document.StoragePath) {
		t.Fatalf("StoragePath = %q, want relative path", document.StoragePath)
	}
	if !strings.HasPrefix(document.StoragePath, "documents/library/") {
		t.Fatalf("StoragePath = %q, want library document path", document.StoragePath)
	}
	if _, err := os.Stat(filepath.Join(dataDir, filepath.FromSlash(document.StoragePath))); err != nil {
		t.Fatalf("uploaded PDF was not saved: %v", err)
	}
}

func TestDocumentsCreateRejectsNonPDF(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	appStore := &fakeApplicationStore{}
	cookie, token := newPageCSRF(t, appStore, "/documents")
	form := map[string]string{
		csrfFieldName:   token,
		"name":          "Platform resume",
		"document_type": string(model.DocumentResume),
	}
	rec := postMultipartFileWithCookie(
		"/documents",
		form,
		"document_file",
		"platform-resume.txt",
		[]byte("not a pdf"),
		cookie,
		appStore,
		dataDir,
	)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(appStore.createdDocuments) != 0 {
		t.Fatalf("createdDocuments len = %d, want 0", len(appStore.createdDocuments))
	}
	if !strings.Contains(rec.Body.String(), "Choose a valid PDF under 20 MB.") {
		t.Fatalf("body does not contain PDF validation error")
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
		`href="/documents/doc_1"`,
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

func TestDocumentsShowEmbedsInlinePDF(t *testing.T) {
	t.Parallel()

	document := model.Document{
		ID:          "doc_1",
		Name:        "Platform resume",
		Type:        model.DocumentResume,
		StoragePath: "documents/library/doc_1.pdf",
		UpdatedAt:   time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
	}
	rec := requestWithStore(http.MethodGet, "/documents/doc_1", nil, &fakeApplicationStore{
		documents: []model.Document{document},
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Document preview",
		`src="/documents/doc_1/download"`,
		`href="/documents/doc_1/download"`,
		"Platform resume",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q", want)
		}
	}
	if strings.Contains(body, "documents/library/doc_1.pdf") {
		t.Fatalf("body exposed raw storage path")
	}
}

func TestDocumentsDownloadAllowsInAppPreviewFrame(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	documentPath := filepath.Join(dataDir, "documents", "library", "doc_1.pdf")
	if err := os.MkdirAll(filepath.Dir(documentPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(documentPath, []byte("%PDF-1.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	document := model.Document{
		ID:          "doc_1",
		Name:        "Platform resume",
		Type:        model.DocumentResume,
		StoragePath: "documents/library/doc_1.pdf",
		UpdatedAt:   time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC),
	}
	req := httptest.NewRequest(http.MethodGet, "/documents/doc_1/download", nil)
	rec := httptest.NewRecorder()
	NewWithOptions(&fakeApplicationStore{
		documents: []model.Document{document},
	}, Options{DataDir: dataDir}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "inline") {
		t.Fatalf("Content-Disposition = %q, want inline", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("X-Frame-Options = %q, want empty for in-app preview", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); strings.Contains(got, "frame-ancestors") {
		t.Fatalf("Content-Security-Policy = %q, want no frame-ancestors for in-app preview", got)
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
		`name="posting_pdf"`,
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

func TestApplicationsCreateWithPostingPDFRedirectsToDetail(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	appStore := &fakeApplicationStore{}
	cookie, token := newFormCSRF(t, appStore)
	fields := map[string]string{
		csrfFieldName:         token,
		"company":             "Northstar Systems",
		"role":                "Senior Platform Engineer",
		"status":              string(model.StatusApplied),
		"priority":            string(model.PriorityHigh),
		"source":              "Company site",
		"posting_url":         "https://jobs.example.com/platform",
		"location":            "Remote",
		"next_action_summary": "Apply",
	}
	rec := postMultipartWithCookie(
		"/applications",
		fields,
		"posting.pdf",
		[]byte("%PDF-1.7\nsaved posting\n"),
		cookie,
		appStore,
		dataDir,
	)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/applications/app_created" {
		t.Fatalf("Location = %q, want detail redirect", location)
	}
	if len(appStore.created) != 1 {
		t.Fatalf("created len = %d, want 1", len(appStore.created))
	}
	if appStore.created[0].PostingURL != "https://jobs.example.com/platform" {
		t.Fatalf("PostingURL = %q", appStore.created[0].PostingURL)
	}
	if len(appStore.appDocuments) != 1 {
		t.Fatalf("appDocuments len = %d, want 1", len(appStore.appDocuments))
	}
	attached := appStore.appDocuments[0]
	if attached.ApplicationID != "app_created" || attached.Document.Type != model.DocumentJobPosting {
		t.Fatalf("attached document = %#v", attached)
	}
	if _, err := os.Stat(filepath.Join(dataDir, filepath.FromSlash(attached.Document.StoragePath))); err != nil {
		t.Fatalf("uploaded PDF was not saved: %v", err)
	}
}

func TestApplicationsCreateRejectsInvalidPostingPDF(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	appStore := &fakeApplicationStore{}
	cookie, token := newFormCSRF(t, appStore)
	fields := map[string]string{
		csrfFieldName: token,
		"company":     "Northstar Systems",
		"role":        "Senior Platform Engineer",
		"status":      string(model.StatusApplied),
		"priority":    string(model.PriorityHigh),
		"posting_url": "https://jobs.example.com/platform",
	}
	rec := postMultipartWithCookie(
		"/applications",
		fields,
		"posting.txt",
		[]byte("not a pdf"),
		cookie,
		appStore,
		dataDir,
	)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if len(appStore.created) != 0 {
		t.Fatalf("created len = %d, want 0", len(appStore.created))
	}
	if len(appStore.appDocuments) != 0 {
		t.Fatalf("appDocuments len = %d, want 0", len(appStore.appDocuments))
	}
	if !strings.Contains(rec.Body.String(), "Choose a valid PDF under 20 MB.") {
		t.Fatalf("body does not contain PDF validation error")
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
	return postMultipartFileWithCookie(target, fields, "posting_pdf", fileName, fileContent, cookie, appStore, dataDir)
}

func postMultipartFileWithCookie(target string, fields map[string]string, fileField string, fileName string, fileContent []byte, cookie *http.Cookie, appStore store.ApplicationStore, dataDir string) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		_ = writer.WriteField(name, value)
	}
	if fileName != "" {
		part, _ := writer.CreateFormFile(fileField, fileName)
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
