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
	DocumentJobPosting  DocumentType = "job_posting"
	DocumentOther       DocumentType = "other"
)

func (d DocumentType) Valid() bool {
	switch d {
	case DocumentResume, DocumentCoverLetter, DocumentWorkSample, DocumentSnippet, DocumentPortfolio, DocumentJobPosting, DocumentOther:
		return true
	default:
		return false
	}
}

type AttachmentType string

const (
	AttachmentResume      AttachmentType = "resume"
	AttachmentCoverLetter AttachmentType = "cover_letter"
	AttachmentWorkSample  AttachmentType = "work_sample"
	AttachmentPortfolio   AttachmentType = "portfolio"
	AttachmentJobPosting  AttachmentType = "job_posting"
	AttachmentOther       AttachmentType = "other"
)

func (a AttachmentType) Valid() bool {
	switch a {
	case AttachmentResume, AttachmentCoverLetter, AttachmentWorkSample, AttachmentPortfolio, AttachmentJobPosting, AttachmentOther:
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

type ApplicationDocument struct {
	ApplicationID  string
	Document       Document
	AttachmentType AttachmentType
	SubmittedAt    *time.Time
	Notes          string
	CreatedAt      time.Time
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
