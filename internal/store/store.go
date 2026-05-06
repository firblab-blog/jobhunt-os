// Package store defines narrow persistence contracts for application features.
package store

import (
	"context"
	"errors"

	"github.com/firblab-blog/jobhunt-os/internal/model"
)

var (
	ErrNotFound = errors.New("store: not found")
	ErrConflict = errors.New("store: conflict")
)

type ApplicationReader interface {
	ListApplications(ctx context.Context) ([]model.Application, error)
	GetApplication(ctx context.Context, id string) (model.Application, error)
	ListApplicationEvents(ctx context.Context, applicationID string) ([]model.ApplicationEvent, error)
	ListDocuments(ctx context.Context) ([]model.Document, error)
	CountDocuments(ctx context.Context) (int, error)
	ListContacts(ctx context.Context) ([]model.Contact, error)
}

type ApplicationWriter interface {
	CreateApplication(ctx context.Context, app model.Application) (model.Application, error)
	UpdateApplicationStatusAndNextAction(ctx context.Context, id string, status model.ApplicationStatus, nextAction model.NextAction) (model.Application, error)
	AddApplicationEvent(ctx context.Context, event model.ApplicationEvent) (model.ApplicationEvent, error)
	CreateDocument(ctx context.Context, document model.Document) (model.Document, error)
	CreateContact(ctx context.Context, contact model.Contact) (model.Contact, error)
}

type ApplicationStore interface {
	ApplicationReader
	ApplicationWriter
}
