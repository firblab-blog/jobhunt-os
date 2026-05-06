package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/firblab-blog/jobhunt-os/internal/model"
	"github.com/firblab-blog/jobhunt-os/internal/store"
)

const sqliteTimestampLayout = "2006-01-02T15:04:05.000Z"

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) ListApplications(ctx context.Context) ([]model.Application, error) {
	rows, err := s.db.QueryContext(ctx, listApplicationsSQL)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	defer rows.Close()

	var applications []model.Application
	for rows.Next() {
		app, err := scanApplication(rows)
		if err != nil {
			return nil, fmt.Errorf("scan application: %w", err)
		}
		applications = append(applications, app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applications: %w", err)
	}

	return applications, nil
}

func (s *Store) CreateApplication(ctx context.Context, app model.Application) (model.Application, error) {
	app = normalizeApplicationForCreate(app)
	if app.ID == "" {
		id, err := newID("app")
		if err != nil {
			return model.Application{}, err
		}
		app.ID = id
	}
	if err := validateApplicationForCreate(app); err != nil {
		return model.Application{}, err
	}

	_, err := s.db.ExecContext(ctx, insertApplicationSQL,
		app.ID,
		app.Company,
		app.Role,
		string(app.Status),
		string(app.Priority),
		app.Source,
		app.PostingURL,
		app.Location,
		app.Compensation.MinCents,
		app.Compensation.MaxCents,
		app.Compensation.Currency,
		app.Compensation.Notes,
		app.Notes,
		formatOptionalTime(app.NextAction.Due),
		app.NextAction.Summary,
	)
	if err != nil {
		return model.Application{}, fmt.Errorf("create application: %w", err)
	}

	return s.GetApplication(ctx, app.ID)
}

func (s *Store) GetApplication(ctx context.Context, id string) (model.Application, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Application{}, fmt.Errorf("application id is required")
	}

	app, err := queryApplication(ctx, s.db, selectApplicationByIDSQL, id)
	if err != nil {
		return model.Application{}, err
	}
	return app, nil
}

func (s *Store) ListApplicationEvents(ctx context.Context, applicationID string) ([]model.ApplicationEvent, error) {
	applicationID = strings.TrimSpace(applicationID)
	if applicationID == "" {
		return nil, fmt.Errorf("application id is required")
	}

	rows, err := s.db.QueryContext(ctx, listApplicationEventsSQL, applicationID)
	if err != nil {
		return nil, fmt.Errorf("list application events: %w", err)
	}
	defer rows.Close()

	var events []model.ApplicationEvent
	for rows.Next() {
		event, err := scanApplicationEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan application event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate application events: %w", err)
	}

	return events, nil
}

func (s *Store) UpdateApplicationStatusAndNextAction(ctx context.Context, id string, status model.ApplicationStatus, nextAction model.NextAction) (model.Application, error) {
	id = strings.TrimSpace(id)
	nextAction = normalizeNextAction(nextAction)
	if id == "" {
		return model.Application{}, fmt.Errorf("application id is required")
	}
	if !status.Valid() {
		return model.Application{}, fmt.Errorf("status %q is invalid", status)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Application{}, fmt.Errorf("begin application update: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := tx.ExecContext(ctx, updateApplicationStatusAndNextActionSQL,
		string(status),
		formatOptionalTime(nextAction.Due),
		nextAction.Summary,
		id,
	)
	if err != nil {
		return model.Application{}, fmt.Errorf("update application: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return model.Application{}, fmt.Errorf("check application update: %w", err)
	}
	if updated == 0 {
		return model.Application{}, store.ErrNotFound
	}

	eventID, err := newID("evt")
	if err != nil {
		return model.Application{}, err
	}
	if _, err := tx.ExecContext(ctx, insertGeneratedStatusEventSQL,
		eventID,
		id,
		string(model.EventNote),
		generatedStatusSummary(status, nextAction),
	); err != nil {
		return model.Application{}, fmt.Errorf("append status event: %w", err)
	}

	app, err := queryApplication(ctx, tx, selectApplicationByIDSQL, id)
	if err != nil {
		return model.Application{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Application{}, fmt.Errorf("commit application update: %w", err)
	}

	return app, nil
}

func (s *Store) UpdateApplicationPostingURL(ctx context.Context, id string, postingURL string) (model.Application, error) {
	id = strings.TrimSpace(id)
	postingURL = strings.TrimSpace(postingURL)
	if id == "" {
		return model.Application{}, fmt.Errorf("application id is required")
	}
	if postingURL != "" && !model.ValidHTTPURL(postingURL) {
		return model.Application{}, fmt.Errorf("posting URL is invalid")
	}

	result, err := s.db.ExecContext(ctx, updateApplicationPostingURLSQL, postingURL, id)
	if err != nil {
		return model.Application{}, fmt.Errorf("update application posting url: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return model.Application{}, fmt.Errorf("check application posting url update: %w", err)
	}
	if updated == 0 {
		return model.Application{}, store.ErrNotFound
	}

	return s.GetApplication(ctx, id)
}

func (s *Store) AddApplicationEvent(ctx context.Context, event model.ApplicationEvent) (model.ApplicationEvent, error) {
	event = normalizeApplicationEventForCreate(event)
	if event.ID == "" {
		id, err := newID("evt")
		if err != nil {
			return model.ApplicationEvent{}, err
		}
		event.ID = id
	}
	if err := validateApplicationEventForCreate(event); err != nil {
		return model.ApplicationEvent{}, err
	}

	_, err := s.db.ExecContext(ctx, insertApplicationEventSQL,
		event.ID,
		event.ApplicationID,
		nullableString(event.ContactID),
		string(event.EventType),
		formatTime(event.OccurredAt),
		event.Summary,
		event.Notes,
	)
	if err != nil {
		if isSQLiteForeignKeyFailure(err) {
			return model.ApplicationEvent{}, store.ErrNotFound
		}
		return model.ApplicationEvent{}, fmt.Errorf("add application event: %w", err)
	}

	return queryApplicationEvent(ctx, s.db, selectApplicationEventByIDSQL, event.ID)
}

type applicationQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type applicationScanner interface {
	Scan(dest ...any) error
}

func queryApplication(ctx context.Context, q applicationQueryer, query string, args ...any) (model.Application, error) {
	app, err := scanApplication(q.QueryRowContext(ctx, query, args...))
	if err == nil {
		return app, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return model.Application{}, store.ErrNotFound
	}
	return model.Application{}, fmt.Errorf("get application: %w", err)
}

func scanApplication(row applicationScanner) (model.Application, error) {
	var app model.Application
	var minCents sql.NullInt64
	var maxCents sql.NullInt64
	var nextActionDue sql.NullString
	var createdAt string
	var updatedAt string

	err := row.Scan(
		&app.ID,
		&app.Company,
		&app.Role,
		&app.Status,
		&app.Priority,
		&app.Source,
		&app.PostingURL,
		&app.Location,
		&minCents,
		&maxCents,
		&app.Compensation.Currency,
		&app.Compensation.Notes,
		&app.Notes,
		&nextActionDue,
		&app.NextAction.Summary,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return model.Application{}, err
	}

	if minCents.Valid {
		app.Compensation.MinCents = &minCents.Int64
	}
	if maxCents.Valid {
		app.Compensation.MaxCents = &maxCents.Int64
	}
	if nextActionDue.Valid {
		due, err := parseSQLiteTime(nextActionDue.String)
		if err != nil {
			return model.Application{}, fmt.Errorf("parse next action due: %w", err)
		}
		app.NextAction.Due = &due
	}

	var errParse error
	app.CreatedAt, errParse = parseSQLiteTime(createdAt)
	if errParse != nil {
		return model.Application{}, fmt.Errorf("parse created_at: %w", errParse)
	}
	app.UpdatedAt, errParse = parseSQLiteTime(updatedAt)
	if errParse != nil {
		return model.Application{}, fmt.Errorf("parse updated_at: %w", errParse)
	}

	return app, nil
}

func queryApplicationEvent(ctx context.Context, q applicationQueryer, query string, args ...any) (model.ApplicationEvent, error) {
	event, err := scanApplicationEvent(q.QueryRowContext(ctx, query, args...))
	if err == nil {
		return event, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return model.ApplicationEvent{}, store.ErrNotFound
	}
	return model.ApplicationEvent{}, fmt.Errorf("get application event: %w", err)
}

func scanApplicationEvent(row applicationScanner) (model.ApplicationEvent, error) {
	var event model.ApplicationEvent
	var contactID sql.NullString
	var occurredAt string
	var createdAt string

	err := row.Scan(
		&event.ID,
		&event.ApplicationID,
		&contactID,
		&event.EventType,
		&occurredAt,
		&event.Summary,
		&event.Notes,
		&createdAt,
	)
	if err != nil {
		return model.ApplicationEvent{}, err
	}
	if contactID.Valid {
		event.ContactID = contactID.String
	}

	var errParse error
	event.OccurredAt, errParse = parseSQLiteTime(occurredAt)
	if errParse != nil {
		return model.ApplicationEvent{}, fmt.Errorf("parse occurred_at: %w", errParse)
	}
	event.CreatedAt, errParse = parseSQLiteTime(createdAt)
	if errParse != nil {
		return model.ApplicationEvent{}, fmt.Errorf("parse created_at: %w", errParse)
	}

	return event, nil
}

func normalizeApplicationForCreate(app model.Application) model.Application {
	app.ID = strings.TrimSpace(app.ID)
	app.Company = strings.TrimSpace(app.Company)
	app.Role = strings.TrimSpace(app.Role)
	app.Source = strings.TrimSpace(app.Source)
	app.PostingURL = strings.TrimSpace(app.PostingURL)
	app.Location = strings.TrimSpace(app.Location)
	app.Compensation.Currency = strings.TrimSpace(app.Compensation.Currency)
	app.Compensation.Notes = strings.TrimSpace(app.Compensation.Notes)
	app.Notes = strings.TrimSpace(app.Notes)
	app.NextAction = normalizeNextAction(app.NextAction)
	if app.Status == "" {
		app.Status = model.StatusProspect
	}
	if app.Priority == "" {
		app.Priority = model.PriorityNormal
	}
	return app
}

func normalizeNextAction(nextAction model.NextAction) model.NextAction {
	nextAction.Summary = strings.TrimSpace(nextAction.Summary)
	return nextAction
}

func normalizeApplicationEventForCreate(event model.ApplicationEvent) model.ApplicationEvent {
	event.ID = strings.TrimSpace(event.ID)
	event.ApplicationID = strings.TrimSpace(event.ApplicationID)
	event.ContactID = strings.TrimSpace(event.ContactID)
	event.Summary = strings.TrimSpace(event.Summary)
	event.Notes = strings.TrimSpace(event.Notes)
	return event
}

func validateApplicationForCreate(app model.Application) error {
	if err := app.ValidateForCreate(); err != nil {
		return err
	}
	if app.ID == "" {
		return fmt.Errorf("application id is required")
	}
	if app.Compensation.Currency != "" && (len(app.Compensation.Currency) != 3 || app.Compensation.Currency != strings.ToUpper(app.Compensation.Currency)) {
		return fmt.Errorf("compensation currency must be a three-letter uppercase code")
	}
	return nil
}

func validateApplicationEventForCreate(event model.ApplicationEvent) error {
	if event.ID == "" {
		return fmt.Errorf("event id is required")
	}
	if event.ApplicationID == "" {
		return fmt.Errorf("application id is required")
	}
	if !event.EventType.Valid() {
		return fmt.Errorf("event type %q is invalid", event.EventType)
	}
	if event.OccurredAt.IsZero() {
		return fmt.Errorf("occurred at is required")
	}
	if event.Summary == "" {
		return fmt.Errorf("summary is required")
	}
	return nil
}

func generatedStatusSummary(status model.ApplicationStatus, nextAction model.NextAction) string {
	summary := "Status changed to " + string(status) + "."
	if nextAction.Summary != "" {
		summary += " Next action: " + nextAction.Summary + "."
	}
	return summary
}

func newID(prefix string) (string, error) {
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", fmt.Errorf("generate %s id: %w", prefix, err)
	}
	return prefix + "_" + hex.EncodeToString(randomBytes[:]), nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func formatOptionalTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}

func formatTime(value time.Time) string {
	return value.UTC().Truncate(time.Millisecond).Format(sqliteTimestampLayout)
}

func parseSQLiteTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed, nil
	}
	return time.Parse(sqliteTimestampLayout, value)
}

func isSQLiteForeignKeyFailure(err error) bool {
	return strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

const applicationColumnsSQL = `
id,
company,
role,
status,
priority,
source,
posting_url,
location,
comp_min_cents,
comp_max_cents,
comp_currency,
comp_notes,
notes,
next_action_due,
next_action_summary,
created_at,
updated_at
`

const listApplicationsSQL = `
SELECT ` + applicationColumnsSQL + `
FROM applications
ORDER BY updated_at DESC, company COLLATE NOCASE ASC, role COLLATE NOCASE ASC, id ASC;
`

const selectApplicationByIDSQL = `
SELECT ` + applicationColumnsSQL + `
FROM applications
WHERE id = ?;
`

const insertApplicationSQL = `
INSERT INTO applications (
  id,
  company,
  role,
  status,
  priority,
  source,
  posting_url,
  location,
  comp_min_cents,
  comp_max_cents,
  comp_currency,
  comp_notes,
  notes,
  next_action_due,
  next_action_summary
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
`

const updateApplicationPostingURLSQL = `
UPDATE applications
SET posting_url = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;
`

const updateApplicationStatusAndNextActionSQL = `
UPDATE applications
SET status = ?,
    next_action_due = ?,
    next_action_summary = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;
`

const insertGeneratedStatusEventSQL = `
INSERT INTO application_events (
  id,
  application_id,
  event_type,
  occurred_at,
  summary
)
VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), ?);
`

const applicationEventColumnsSQL = `
id,
application_id,
contact_id,
event_type,
occurred_at,
summary,
notes,
created_at
`

const selectApplicationEventByIDSQL = `
SELECT ` + applicationEventColumnsSQL + `
FROM application_events
WHERE id = ?;
`

const listApplicationEventsSQL = `
SELECT ` + applicationEventColumnsSQL + `
FROM application_events
WHERE application_id = ?
ORDER BY occurred_at DESC, created_at DESC, id DESC;
`

const insertApplicationEventSQL = `
INSERT INTO application_events (
  id,
  application_id,
  contact_id,
  event_type,
  occurred_at,
  summary,
  notes
)
VALUES (?, ?, ?, ?, ?, ?, ?);
`
