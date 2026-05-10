package model

import "testing"

func TestDocumentTypeValidity(t *testing.T) {
	t.Parallel()

	for _, documentType := range []DocumentType{
		DocumentResume,
		DocumentCoverLetter,
		DocumentWorkSample,
		DocumentSnippet,
		DocumentPortfolio,
		DocumentJobPosting,
		DocumentOther,
	} {
		documentType := documentType
		t.Run(string(documentType), func(t *testing.T) {
			t.Parallel()

			if !documentType.Valid() {
				t.Fatalf("DocumentType(%q).Valid() = false, want true", documentType)
			}
		})
	}

	if DocumentType("transcript").Valid() {
		t.Fatalf("DocumentType(transcript).Valid() = true, want false")
	}
}

func TestAttachmentTypeValidity(t *testing.T) {
	t.Parallel()

	for _, attachmentType := range []AttachmentType{
		AttachmentResume,
		AttachmentCoverLetter,
		AttachmentWorkSample,
		AttachmentPortfolio,
		AttachmentJobPosting,
		AttachmentOther,
	} {
		attachmentType := attachmentType
		t.Run(string(attachmentType), func(t *testing.T) {
			t.Parallel()

			if !attachmentType.Valid() {
				t.Fatalf("AttachmentType(%q).Valid() = false, want true", attachmentType)
			}
		})
	}

	if AttachmentType("offer_letter").Valid() {
		t.Fatalf("AttachmentType(offer_letter).Valid() = true, want false")
	}
}

func TestDocumentValidateForCreate(t *testing.T) {
	t.Parallel()

	document := Document{
		Name:        "Resume",
		Type:        DocumentResume,
		StoragePath: "documents/resume.pdf",
	}
	if err := document.ValidateForCreate(); err != nil {
		t.Fatalf("ValidateForCreate() error = %v", err)
	}

	for name, document := range map[string]Document{
		"missing name": {
			Type:        DocumentResume,
			StoragePath: "documents/resume.pdf",
		},
		"invalid type": {
			Name:        "Resume",
			Type:        "transcript",
			StoragePath: "documents/resume.pdf",
		},
		"missing storage path": {
			Name: "Resume",
			Type: DocumentResume,
		},
	} {
		name, document := name, document
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := document.ValidateForCreate(); err == nil {
				t.Fatalf("ValidateForCreate() error = nil, want error")
			}
		})
	}
}

func TestContactValidateForCreate(t *testing.T) {
	t.Parallel()

	if err := (Contact{Name: "Avery Recruiter"}).ValidateForCreate(); err != nil {
		t.Fatalf("ValidateForCreate() error = %v", err)
	}
	if err := (Contact{Name: "   "}).ValidateForCreate(); err == nil {
		t.Fatalf("ValidateForCreate(blank name) error = nil, want error")
	}
}
