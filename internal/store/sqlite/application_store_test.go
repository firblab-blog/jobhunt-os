package sqlite

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/firblab-blog/jobhunt-os/internal/model"
	"github.com/firblab-blog/jobhunt-os/internal/store"
)

func TestApplicationStoreCreateThenList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newMigratedApplicationStore(t)

	first, err := st.CreateApplication(ctx, model.Application{
		ID:       "app_first",
		Company:  "Northstar Systems",
		Role:     "Senior Platform Engineer",
		Status:   model.StatusApplied,
		Priority: model.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("CreateApplication(first) error = %v", err)
	}
	second, err := st.CreateApplication(ctx, model.Application{
		ID:      "app_second",
		Company: "Atlas Cloud",
		Role:    "Staff DevOps Engineer",
	})
	if err != nil {
		t.Fatalf("CreateApplication(second) error = %v", err)
	}

	applications, err := st.ListApplications(ctx)
	if err != nil {
		t.Fatalf("ListApplications() error = %v", err)
	}
	if len(applications) != 2 {
		t.Fatalf("ListApplications() len = %d, want 2", len(applications))
	}

	seen := map[string]model.Application{}
	for _, app := range applications {
		seen[app.ID] = app
	}
	if seen[first.ID].Company != "Northstar Systems" {
		t.Fatalf("first company = %q, want Northstar Systems", seen[first.ID].Company)
	}
	if seen[second.ID].Status != model.StatusProspect {
		t.Fatalf("second status = %q, want %q", seen[second.ID].Status, model.StatusProspect)
	}
	if seen[second.ID].Priority != model.PriorityNormal {
		t.Fatalf("second priority = %q, want %q", seen[second.ID].Priority, model.PriorityNormal)
	}
}

func TestApplicationStoreCreateThenGetDetail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newMigratedApplicationStore(t)
	minCents := int64(12000000)
	maxCents := int64(15000000)
	due := time.Date(2026, 5, 8, 14, 30, 45, 987654321, time.FixedZone("offset", -4*60*60))

	created, err := st.CreateApplication(ctx, model.Application{
		ID:       "app_detail",
		Company:  " Signal Works ",
		Role:     " Infrastructure Lead ",
		Status:   model.StatusInterviewing,
		Priority: model.PriorityHigh,
		Source:   "Referral",
		Location: "Remote",
		Compensation: model.Compensation{
			MinCents: &minCents,
			MaxCents: &maxCents,
			Currency: "USD",
			Notes:    "Base only",
		},
		Notes:      "Promising team",
		NextAction: model.NextAction{Due: &due, Summary: "Send prep notes"},
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}

	got, err := st.GetApplication(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetApplication() error = %v", err)
	}

	if got.Company != "Signal Works" {
		t.Fatalf("Company = %q, want Signal Works", got.Company)
	}
	if got.Role != "Infrastructure Lead" {
		t.Fatalf("Role = %q, want Infrastructure Lead", got.Role)
	}
	if got.Compensation.MinCents == nil || *got.Compensation.MinCents != minCents {
		t.Fatalf("MinCents = %v, want %d", got.Compensation.MinCents, minCents)
	}
	if got.NextAction.Due == nil || got.NextAction.Due.Format(sqliteTimestampLayout) != "2026-05-08T18:30:45.987Z" {
		t.Fatalf("NextAction.Due = %v, want normalized UTC milliseconds", got.NextAction.Due)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps were not populated: created=%v updated=%v", got.CreatedAt, got.UpdatedAt)
	}
}

func TestApplicationStoreUpdatePostingURLAndAttachDocument(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newMigratedApplicationStore(t)
	app, err := st.CreateApplication(ctx, model.Application{
		ID:      "app_posting",
		Company: "Northstar Systems",
		Role:    "Senior Platform Engineer",
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}

	updated, err := st.UpdateApplicationPostingURL(ctx, app.ID, "https://jobs.example.com/platform")
	if err != nil {
		t.Fatalf("UpdateApplicationPostingURL() error = %v", err)
	}
	if updated.PostingURL != "https://jobs.example.com/platform" {
		t.Fatalf("PostingURL = %q", updated.PostingURL)
	}

	attached, err := st.AttachDocumentToApplication(ctx, app.ID, model.Document{
		ID:          "doc_posting",
		Name:        "Northstar job posting",
		Type:        model.DocumentJobPosting,
		StoragePath: "documents/app_posting/doc_posting.pdf",
	}, model.AttachmentJobPosting, "")
	if err != nil {
		t.Fatalf("AttachDocumentToApplication() error = %v", err)
	}
	if attached.ApplicationID != app.ID || attached.Document.ID != "doc_posting" {
		t.Fatalf("attached document = %#v", attached)
	}

	documents, err := st.ListApplicationDocuments(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListApplicationDocuments() error = %v", err)
	}
	if len(documents) != 1 {
		t.Fatalf("ListApplicationDocuments() len = %d, want 1", len(documents))
	}
	if documents[0].Document.Type != model.DocumentJobPosting || documents[0].AttachmentType != model.AttachmentJobPosting {
		t.Fatalf("application document = %#v", documents[0])
	}
}

func TestApplicationStoreNotFound(t *testing.T) {
	t.Parallel()

	st := newMigratedApplicationStore(t)

	_, err := st.GetApplication(context.Background(), "missing")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetApplication() error = %v, want ErrNotFound", err)
	}

	occurredAt := time.Date(2026, 5, 6, 12, 15, 20, 0, time.UTC)
	_, err = st.AddApplicationEvent(context.Background(), model.ApplicationEvent{
		ApplicationID: "missing",
		EventType:     model.EventNote,
		OccurredAt:    occurredAt,
		Summary:       "This app does not exist.",
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("AddApplicationEvent(missing app) error = %v, want ErrNotFound", err)
	}
}

func TestApplicationStoreRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newMigratedApplicationStore(t)

	if _, err := st.CreateApplication(ctx, model.Application{Role: "Engineer"}); err == nil {
		t.Fatalf("CreateApplication(missing company) error = nil, want error")
	}
	if _, err := st.CreateApplication(ctx, model.Application{
		Company: "Northstar Systems",
		Role:    "Engineer",
		Status:  "drafting",
	}); err == nil {
		t.Fatalf("CreateApplication(invalid status) error = nil, want error")
	}
	if _, err := st.CreateApplication(ctx, model.Application{
		Company:      "Northstar Systems",
		Role:         "Engineer",
		Compensation: model.Compensation{Currency: "usd"},
	}); err == nil {
		t.Fatalf("CreateApplication(invalid currency) error = nil, want error")
	}

	_, err := st.AddApplicationEvent(ctx, model.ApplicationEvent{
		ApplicationID: "app_missing",
		EventType:     model.EventNote,
		Summary:       "Missing occurred_at.",
	})
	if err == nil {
		t.Fatalf("AddApplicationEvent(missing occurred_at) error = nil, want error")
	}
}

func TestApplicationStoreAddEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newMigratedApplicationStore(t)
	app, err := st.CreateApplication(ctx, model.Application{
		ID:      "app_event",
		Company: "Northstar Systems",
		Role:    "Senior Platform Engineer",
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}

	occurredAt := time.Date(2026, 5, 6, 12, 15, 20, 123456789, time.FixedZone("offset", -4*60*60))
	event, err := st.AddApplicationEvent(ctx, model.ApplicationEvent{
		ID:            "evt_recruiter",
		ApplicationID: app.ID,
		EventType:     model.EventRecruiterScreen,
		OccurredAt:    occurredAt,
		Summary:       "Recruiter screen completed.",
		Notes:         "Follow-up expected next week.",
	})
	if err != nil {
		t.Fatalf("AddApplicationEvent() error = %v", err)
	}

	if event.ApplicationID != app.ID {
		t.Fatalf("ApplicationID = %q, want %q", event.ApplicationID, app.ID)
	}
	if event.OccurredAt.Format(sqliteTimestampLayout) != "2026-05-06T16:15:20.123Z" {
		t.Fatalf("OccurredAt = %v, want normalized UTC milliseconds", event.OccurredAt)
	}
	if event.CreatedAt.IsZero() {
		t.Fatalf("CreatedAt was not populated")
	}

	laterEvent, err := st.AddApplicationEvent(ctx, model.ApplicationEvent{
		ID:            "evt_interview",
		ApplicationID: app.ID,
		EventType:     model.EventInterview,
		OccurredAt:    occurredAt.Add(24 * time.Hour),
		Summary:       "Interview scheduled.",
	})
	if err != nil {
		t.Fatalf("AddApplicationEvent(later) error = %v", err)
	}

	events, err := st.ListApplicationEvents(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListApplicationEvents() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("ListApplicationEvents() len = %d, want 2", len(events))
	}
	if events[0].ID != laterEvent.ID || events[1].ID != event.ID {
		t.Fatalf("ListApplicationEvents() order = [%q, %q], want reverse chronological", events[0].ID, events[1].ID)
	}
}

func TestApplicationStoreUpdateStatusAndNextActionAppendsEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openTestDB(t)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	st := NewStore(db)
	app, err := st.CreateApplication(ctx, model.Application{
		ID:      "app_update",
		Company: "Atlas Cloud",
		Role:    "Staff DevOps Engineer",
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	due := time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC)

	updated, err := st.UpdateApplicationStatusAndNextAction(ctx, app.ID, model.StatusInterviewing, model.NextAction{
		Due:     &due,
		Summary: "Prep architecture notes",
	})
	if err != nil {
		t.Fatalf("UpdateApplicationStatusAndNextAction() error = %v", err)
	}

	if updated.Status != model.StatusInterviewing {
		t.Fatalf("Status = %q, want %q", updated.Status, model.StatusInterviewing)
	}
	if updated.NextAction.Summary != "Prep architecture notes" {
		t.Fatalf("NextAction.Summary = %q, want Prep architecture notes", updated.NextAction.Summary)
	}

	events, err := st.ListApplicationEvents(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListApplicationEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("ListApplicationEvents() len = %d, want 1", len(events))
	}
	if events[0].EventType != model.EventNote {
		t.Fatalf("generated EventType = %q, want %q", events[0].EventType, model.EventNote)
	}
	if !strings.Contains(events[0].Summary, "Status changed to interviewing.") || !strings.Contains(events[0].Summary, "Next action: Prep architecture notes.") {
		t.Fatalf("generated summary = %q", events[0].Summary)
	}
}

func newMigratedApplicationStore(t *testing.T) *Store {
	t.Helper()

	db := openTestDB(t)
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return NewStore(db)
}
