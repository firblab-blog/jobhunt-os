package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/firblab-blog/jobhunt-os/internal/model"
	"github.com/firblab-blog/jobhunt-os/internal/store"
)

func (s *Store) ListDocuments(ctx context.Context) ([]model.Document, error) {
	rows, err := s.db.QueryContext(ctx, listDocumentsSQL)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	var documents []model.Document
	for rows.Next() {
		document, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate documents: %w", err)
	}

	return documents, nil
}

func (s *Store) CountDocuments(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, countDocumentsSQL).Scan(&count); err != nil {
		return 0, fmt.Errorf("count documents: %w", err)
	}
	return count, nil
}

func (s *Store) GetDocument(ctx context.Context, id string) (model.Document, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Document{}, fmt.Errorf("document id is required")
	}
	return queryDocument(ctx, s.db, selectDocumentByIDSQL, id)
}

func (s *Store) ListApplicationDocuments(ctx context.Context, applicationID string) ([]model.ApplicationDocument, error) {
	applicationID = strings.TrimSpace(applicationID)
	if applicationID == "" {
		return nil, fmt.Errorf("application id is required")
	}

	rows, err := s.db.QueryContext(ctx, listApplicationDocumentsSQL, applicationID)
	if err != nil {
		return nil, fmt.Errorf("list application documents: %w", err)
	}
	defer rows.Close()

	var documents []model.ApplicationDocument
	for rows.Next() {
		document, err := scanApplicationDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("scan application document: %w", err)
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate application documents: %w", err)
	}

	return documents, nil
}

func (s *Store) CreateDocument(ctx context.Context, document model.Document) (model.Document, error) {
	document = normalizeDocumentForCreate(document)
	if document.ID == "" {
		id, err := newID("doc")
		if err != nil {
			return model.Document{}, err
		}
		document.ID = id
	}
	if err := document.ValidateForCreate(); err != nil {
		return model.Document{}, err
	}
	if document.Type == "" {
		document.Type = model.DocumentOther
	}

	_, err := s.db.ExecContext(ctx, insertDocumentSQL,
		document.ID,
		document.Name,
		string(document.Type),
		document.StoragePath,
		document.Notes,
	)
	if err != nil {
		return model.Document{}, fmt.Errorf("create document: %w", err)
	}

	return queryDocument(ctx, s.db, selectDocumentByIDSQL, document.ID)
}

func (s *Store) AttachDocumentToApplication(ctx context.Context, applicationID string, document model.Document, attachmentType model.AttachmentType, notes string) (model.ApplicationDocument, error) {
	applicationID = strings.TrimSpace(applicationID)
	document = normalizeDocumentForCreate(document)
	attachmentType = normalizeAttachmentType(attachmentType)
	notes = strings.TrimSpace(notes)
	if applicationID == "" {
		return model.ApplicationDocument{}, fmt.Errorf("application id is required")
	}
	if document.ID == "" {
		id, err := newID("doc")
		if err != nil {
			return model.ApplicationDocument{}, err
		}
		document.ID = id
	}
	if err := document.ValidateForCreate(); err != nil {
		return model.ApplicationDocument{}, err
	}
	if !attachmentType.Valid() {
		return model.ApplicationDocument{}, fmt.Errorf("attachment type %q is invalid", attachmentType)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("begin attach document: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, insertDocumentSQL,
		document.ID,
		document.Name,
		string(document.Type),
		document.StoragePath,
		document.Notes,
	); err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("create attached document: %w", err)
	}

	if _, err := tx.ExecContext(ctx, insertApplicationDocumentSQL,
		applicationID,
		document.ID,
		string(attachmentType),
		notes,
	); err != nil {
		if isSQLiteForeignKeyFailure(err) {
			return model.ApplicationDocument{}, store.ErrNotFound
		}
		return model.ApplicationDocument{}, fmt.Errorf("attach document to application: %w", err)
	}

	attached, err := queryApplicationDocument(ctx, tx, selectApplicationDocumentSQL, applicationID, document.ID, string(attachmentType))
	if err != nil {
		return model.ApplicationDocument{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("commit attached document: %w", err)
	}

	return attached, nil
}

func (s *Store) ListContacts(ctx context.Context) ([]model.Contact, error) {
	rows, err := s.db.QueryContext(ctx, listContactsSQL)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	var contacts []model.Contact
	for rows.Next() {
		contact, err := scanContact(rows)
		if err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		contacts = append(contacts, contact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contacts: %w", err)
	}

	return contacts, nil
}

func (s *Store) CreateContact(ctx context.Context, contact model.Contact) (model.Contact, error) {
	contact = normalizeContactForCreate(contact)
	if contact.ID == "" {
		id, err := newID("ctc")
		if err != nil {
			return model.Contact{}, err
		}
		contact.ID = id
	}
	if err := contact.ValidateForCreate(); err != nil {
		return model.Contact{}, err
	}

	_, err := s.db.ExecContext(ctx, insertContactSQL,
		contact.ID,
		contact.Name,
		contact.Organization,
		contact.Role,
		contact.Email,
		contact.Phone,
		contact.Location,
		contact.Notes,
	)
	if err != nil {
		return model.Contact{}, fmt.Errorf("create contact: %w", err)
	}

	return queryContact(ctx, s.db, selectContactByIDSQL, contact.ID)
}

func queryDocument(ctx context.Context, q applicationQueryer, query string, args ...any) (model.Document, error) {
	document, err := scanDocument(q.QueryRowContext(ctx, query, args...))
	if err == nil {
		return document, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return model.Document{}, store.ErrNotFound
	}
	return model.Document{}, fmt.Errorf("get document: %w", err)
}

func scanDocument(row applicationScanner) (model.Document, error) {
	var document model.Document
	var createdAt string
	var updatedAt string

	err := row.Scan(
		&document.ID,
		&document.Name,
		&document.Type,
		&document.StoragePath,
		&document.Notes,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return model.Document{}, err
	}

	var errParse error
	document.CreatedAt, errParse = parseSQLiteTime(createdAt)
	if errParse != nil {
		return model.Document{}, fmt.Errorf("parse created_at: %w", errParse)
	}
	document.UpdatedAt, errParse = parseSQLiteTime(updatedAt)
	if errParse != nil {
		return model.Document{}, fmt.Errorf("parse updated_at: %w", errParse)
	}

	return document, nil
}

func queryApplicationDocument(ctx context.Context, q applicationQueryer, query string, args ...any) (model.ApplicationDocument, error) {
	document, err := scanApplicationDocument(q.QueryRowContext(ctx, query, args...))
	if err == nil {
		return document, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return model.ApplicationDocument{}, store.ErrNotFound
	}
	return model.ApplicationDocument{}, fmt.Errorf("get application document: %w", err)
}

func scanApplicationDocument(row applicationScanner) (model.ApplicationDocument, error) {
	var attached model.ApplicationDocument
	var submittedAt sql.NullString
	var documentCreatedAt string
	var documentUpdatedAt string
	var attachedCreatedAt string

	err := row.Scan(
		&attached.ApplicationID,
		&attached.AttachmentType,
		&submittedAt,
		&attached.Notes,
		&attachedCreatedAt,
		&attached.Document.ID,
		&attached.Document.Name,
		&attached.Document.Type,
		&attached.Document.StoragePath,
		&attached.Document.Notes,
		&documentCreatedAt,
		&documentUpdatedAt,
	)
	if err != nil {
		return model.ApplicationDocument{}, err
	}
	if submittedAt.Valid {
		parsed, err := parseSQLiteTime(submittedAt.String)
		if err != nil {
			return model.ApplicationDocument{}, fmt.Errorf("parse submitted_at: %w", err)
		}
		attached.SubmittedAt = &parsed
	}

	var errParse error
	attached.CreatedAt, errParse = parseSQLiteTime(attachedCreatedAt)
	if errParse != nil {
		return model.ApplicationDocument{}, fmt.Errorf("parse application document created_at: %w", errParse)
	}
	attached.Document.CreatedAt, errParse = parseSQLiteTime(documentCreatedAt)
	if errParse != nil {
		return model.ApplicationDocument{}, fmt.Errorf("parse document created_at: %w", errParse)
	}
	attached.Document.UpdatedAt, errParse = parseSQLiteTime(documentUpdatedAt)
	if errParse != nil {
		return model.ApplicationDocument{}, fmt.Errorf("parse document updated_at: %w", errParse)
	}

	return attached, nil
}

func queryContact(ctx context.Context, q applicationQueryer, query string, args ...any) (model.Contact, error) {
	contact, err := scanContact(q.QueryRowContext(ctx, query, args...))
	if err != nil {
		return model.Contact{}, fmt.Errorf("get contact: %w", err)
	}
	return contact, nil
}

func scanContact(row applicationScanner) (model.Contact, error) {
	var contact model.Contact
	var createdAt string
	var updatedAt string

	err := row.Scan(
		&contact.ID,
		&contact.Name,
		&contact.Organization,
		&contact.Role,
		&contact.Email,
		&contact.Phone,
		&contact.Location,
		&contact.Notes,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return model.Contact{}, err
	}

	var errParse error
	contact.CreatedAt, errParse = parseSQLiteTime(createdAt)
	if errParse != nil {
		return model.Contact{}, fmt.Errorf("parse created_at: %w", errParse)
	}
	contact.UpdatedAt, errParse = parseSQLiteTime(updatedAt)
	if errParse != nil {
		return model.Contact{}, fmt.Errorf("parse updated_at: %w", errParse)
	}

	return contact, nil
}

func normalizeDocumentForCreate(document model.Document) model.Document {
	document.ID = strings.TrimSpace(document.ID)
	document.Name = strings.TrimSpace(document.Name)
	document.StoragePath = strings.TrimSpace(document.StoragePath)
	document.Notes = strings.TrimSpace(document.Notes)
	if document.Type == "" {
		document.Type = model.DocumentOther
	}
	return document
}

func normalizeContactForCreate(contact model.Contact) model.Contact {
	contact.ID = strings.TrimSpace(contact.ID)
	contact.Name = strings.TrimSpace(contact.Name)
	contact.Organization = strings.TrimSpace(contact.Organization)
	contact.Role = strings.TrimSpace(contact.Role)
	contact.Email = strings.TrimSpace(contact.Email)
	contact.Phone = strings.TrimSpace(contact.Phone)
	contact.Location = strings.TrimSpace(contact.Location)
	contact.Notes = strings.TrimSpace(contact.Notes)
	return contact
}

func normalizeAttachmentType(attachmentType model.AttachmentType) model.AttachmentType {
	if attachmentType == "" {
		return model.AttachmentOther
	}
	return attachmentType
}

const documentColumnsSQL = `
id,
name,
document_type,
storage_path,
notes,
created_at,
updated_at
`

const listDocumentsSQL = `
SELECT ` + documentColumnsSQL + `
FROM documents
ORDER BY updated_at DESC, name COLLATE NOCASE ASC, id ASC;
`

const countDocumentsSQL = `
SELECT count(*)
FROM documents;
`

const selectDocumentByIDSQL = `
SELECT ` + documentColumnsSQL + `
FROM documents
WHERE id = ?;
`

const insertDocumentSQL = `
INSERT INTO documents (
  id,
  name,
  document_type,
  storage_path,
  notes
)
VALUES (?, ?, ?, ?, ?);
`

const applicationDocumentColumnsSQL = `
ad.application_id,
ad.attachment_type,
ad.submitted_at,
ad.notes,
ad.created_at,
d.id,
d.name,
d.document_type,
d.storage_path,
d.notes,
d.created_at,
d.updated_at
`

const listApplicationDocumentsSQL = `
SELECT ` + applicationDocumentColumnsSQL + `
FROM application_documents ad
JOIN documents d ON d.id = ad.document_id
WHERE ad.application_id = ?
ORDER BY ad.created_at DESC, d.name COLLATE NOCASE ASC, d.id ASC;
`

const selectApplicationDocumentSQL = `
SELECT ` + applicationDocumentColumnsSQL + `
FROM application_documents ad
JOIN documents d ON d.id = ad.document_id
WHERE ad.application_id = ?
  AND ad.document_id = ?
  AND ad.attachment_type = ?;
`

const insertApplicationDocumentSQL = `
INSERT INTO application_documents (
  application_id,
  document_id,
  attachment_type,
  notes
)
VALUES (?, ?, ?, ?);
`

const contactColumnsSQL = `
id,
name,
organization,
role,
email,
phone,
location,
notes,
created_at,
updated_at
`

const listContactsSQL = `
SELECT ` + contactColumnsSQL + `
FROM contacts
ORDER BY updated_at DESC, name COLLATE NOCASE ASC, id ASC;
`

const selectContactByIDSQL = `
SELECT ` + contactColumnsSQL + `
FROM contacts
WHERE id = ?;
`

const insertContactSQL = `
INSERT INTO contacts (
  id,
  name,
  organization,
  role,
  email,
  phone,
  location,
  notes
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);
`
