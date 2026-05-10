package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/firblab-blog/jobhunt-os/internal/model"
	"github.com/firblab-blog/jobhunt-os/internal/store"
)

func TestSupportingStoreCreateGetListAndCountDocuments(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newMigratedApplicationStore(t)

	created, err := st.CreateDocument(ctx, model.Document{
		ID:          " doc_resume ",
		Name:        " Resume ",
		StoragePath: " documents/resume.pdf ",
		Notes:       " Primary version ",
	})
	if err != nil {
		t.Fatalf("CreateDocument() error = %v", err)
	}
	if created.ID != "doc_resume" {
		t.Fatalf("ID = %q, want doc_resume", created.ID)
	}
	if created.Name != "Resume" {
		t.Fatalf("Name = %q, want Resume", created.Name)
	}
	if created.Type != model.DocumentOther {
		t.Fatalf("Type = %q, want %q", created.Type, model.DocumentOther)
	}
	if created.StoragePath != "documents/resume.pdf" {
		t.Fatalf("StoragePath = %q, want documents/resume.pdf", created.StoragePath)
	}
	if created.Notes != "Primary version" {
		t.Fatalf("Notes = %q, want Primary version", created.Notes)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("timestamps were not populated: created=%v updated=%v", created.CreatedAt, created.UpdatedAt)
	}

	got, err := st.GetDocument(ctx, " doc_resume ")
	if err != nil {
		t.Fatalf("GetDocument() error = %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("GetDocument() ID = %q, want %q", got.ID, created.ID)
	}

	documents, err := st.ListDocuments(ctx)
	if err != nil {
		t.Fatalf("ListDocuments() error = %v", err)
	}
	if len(documents) != 1 || documents[0].ID != created.ID {
		t.Fatalf("ListDocuments() = %#v, want created document", documents)
	}

	count, err := st.CountDocuments(ctx)
	if err != nil {
		t.Fatalf("CountDocuments() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("CountDocuments() = %d, want 1", count)
	}
}

func TestSupportingStoreDocumentErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newMigratedApplicationStore(t)

	_, err := st.GetDocument(ctx, "missing")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetDocument(missing) error = %v, want ErrNotFound", err)
	}
	if _, err := st.GetDocument(ctx, " "); err == nil {
		t.Fatalf("GetDocument(blank id) error = nil, want error")
	}
	if _, err := st.CreateDocument(ctx, model.Document{StoragePath: "documents/resume.pdf"}); err == nil {
		t.Fatalf("CreateDocument(missing name) error = nil, want error")
	}
	if _, err := st.CreateDocument(ctx, model.Document{
		Name:        "Resume",
		Type:        "transcript",
		StoragePath: "documents/resume.pdf",
	}); err == nil {
		t.Fatalf("CreateDocument(invalid type) error = nil, want error")
	}
	if _, err := st.CreateDocument(ctx, model.Document{Name: "Resume"}); err == nil {
		t.Fatalf("CreateDocument(missing storage path) error = nil, want error")
	}
}

func TestSupportingStoreAttachDocumentToApplication(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newMigratedApplicationStore(t)
	app, err := st.CreateApplication(ctx, model.Application{
		ID:      "app_supporting_docs",
		Company: "Northstar Systems",
		Role:    "Senior Platform Engineer",
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}

	attached, err := st.AttachDocumentToApplication(ctx, " "+app.ID+" ", model.Document{
		ID:          " doc_cover ",
		Name:        " Cover letter ",
		StoragePath: " documents/cover.pdf ",
		Notes:       " Tailored copy ",
	}, "", " Submitted with application ")
	if err != nil {
		t.Fatalf("AttachDocumentToApplication() error = %v", err)
	}
	if attached.ApplicationID != app.ID {
		t.Fatalf("ApplicationID = %q, want %q", attached.ApplicationID, app.ID)
	}
	if attached.Document.ID != "doc_cover" {
		t.Fatalf("Document.ID = %q, want doc_cover", attached.Document.ID)
	}
	if attached.Document.Type != model.DocumentOther {
		t.Fatalf("Document.Type = %q, want %q", attached.Document.Type, model.DocumentOther)
	}
	if attached.AttachmentType != model.AttachmentOther {
		t.Fatalf("AttachmentType = %q, want %q", attached.AttachmentType, model.AttachmentOther)
	}
	if attached.Notes != "Submitted with application" {
		t.Fatalf("Notes = %q, want Submitted with application", attached.Notes)
	}
	if attached.CreatedAt.IsZero() || attached.Document.CreatedAt.IsZero() || attached.Document.UpdatedAt.IsZero() {
		t.Fatalf("timestamps were not populated: attached=%v documentCreated=%v documentUpdated=%v", attached.CreatedAt, attached.Document.CreatedAt, attached.Document.UpdatedAt)
	}

	documents, err := st.ListApplicationDocuments(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListApplicationDocuments() error = %v", err)
	}
	if len(documents) != 1 || documents[0].Document.ID != attached.Document.ID {
		t.Fatalf("ListApplicationDocuments() = %#v, want attached document", documents)
	}
}

func TestSupportingStoreAttachDocumentErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newMigratedApplicationStore(t)
	validDocument := model.Document{
		ID:          "doc_resume",
		Name:        "Resume",
		StoragePath: "documents/resume.pdf",
	}

	if _, err := st.AttachDocumentToApplication(ctx, " ", validDocument, model.AttachmentResume, ""); err == nil {
		t.Fatalf("AttachDocumentToApplication(blank app id) error = nil, want error")
	}
	if _, err := st.AttachDocumentToApplication(ctx, "missing", validDocument, model.AttachmentResume, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("AttachDocumentToApplication(missing app) error = %v, want ErrNotFound", err)
	}

	count, err := st.CountDocuments(ctx)
	if err != nil {
		t.Fatalf("CountDocuments() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("CountDocuments() after failed attach = %d, want rollback to leave 0", count)
	}

	app, err := st.CreateApplication(ctx, model.Application{
		ID:      "app_invalid_attachment",
		Company: "Atlas Cloud",
		Role:    "Staff DevOps Engineer",
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if _, err := st.AttachDocumentToApplication(ctx, app.ID, validDocument, "transcript", ""); err == nil {
		t.Fatalf("AttachDocumentToApplication(invalid attachment type) error = nil, want error")
	}
	if _, err := st.ListApplicationDocuments(ctx, " "); err == nil {
		t.Fatalf("ListApplicationDocuments(blank app id) error = nil, want error")
	}
}

func TestSupportingStoreCreateAndListContacts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st := newMigratedApplicationStore(t)

	created, err := st.CreateContact(ctx, model.Contact{
		ID:           " contact_recruiter ",
		Name:         " Avery Recruiter ",
		Organization: " Northstar Systems ",
		Role:         " Recruiter ",
		Email:        " avery@example.com ",
		Phone:        " 555-0100 ",
		Location:     " Remote ",
		Notes:        " Prefers email ",
	})
	if err != nil {
		t.Fatalf("CreateContact() error = %v", err)
	}
	if created.ID != "contact_recruiter" {
		t.Fatalf("ID = %q, want contact_recruiter", created.ID)
	}
	if created.Name != "Avery Recruiter" {
		t.Fatalf("Name = %q, want Avery Recruiter", created.Name)
	}
	if created.Organization != "Northstar Systems" {
		t.Fatalf("Organization = %q, want Northstar Systems", created.Organization)
	}
	if created.Role != "Recruiter" || created.Email != "avery@example.com" || created.Phone != "555-0100" {
		t.Fatalf("contact fields were not normalized: %#v", created)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("timestamps were not populated: created=%v updated=%v", created.CreatedAt, created.UpdatedAt)
	}

	contacts, err := st.ListContacts(ctx)
	if err != nil {
		t.Fatalf("ListContacts() error = %v", err)
	}
	if len(contacts) != 1 || contacts[0].ID != created.ID {
		t.Fatalf("ListContacts() = %#v, want created contact", contacts)
	}
}

func TestSupportingStoreCreateContactRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	st := newMigratedApplicationStore(t)

	if _, err := st.CreateContact(context.Background(), model.Contact{Name: " "}); err == nil {
		t.Fatalf("CreateContact(blank name) error = nil, want error")
	}
}
