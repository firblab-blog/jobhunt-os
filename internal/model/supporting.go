package model

import (
	"errors"
	"strings"
	"time"
)

type DocumentType string

const (
	DocumentResume      DocumentType = "resume"
	DocumentCoverLetter DocumentType = "cover_letter"
	DocumentWorkSample  DocumentType = "work_sample"
	DocumentSnippet     DocumentType = "snippet"
	DocumentPortfolio   DocumentType = "portfolio"
	DocumentOther       DocumentType = "other"
)

func (d DocumentType) Valid() bool {
	switch d {
	case DocumentResume, DocumentCoverLetter, DocumentWorkSample, DocumentSnippet, DocumentPortfolio, DocumentOther:
		return true
	default:
		return false
	}
}

type Document struct {
	ID          string
	Name        string
	Type        DocumentType
	StoragePath string
	Notes       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (d Document) ValidateForCreate() error {
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("document name is required")
	}
	if d.Type != "" && !d.Type.Valid() {
		return errors.New("document type is invalid")
	}
	if strings.TrimSpace(d.StoragePath) == "" {
		return errors.New("document storage path is required")
	}
	return nil
}

type Contact struct {
	ID           string
	Name         string
	Organization string
	Role         string
	Email        string
	Phone        string
	Location     string
	Notes        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (c Contact) ValidateForCreate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("contact name is required")
	}
	return nil
}
