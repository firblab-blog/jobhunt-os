ALTER TABLE applications
  ADD COLUMN posting_url TEXT NOT NULL DEFAULT '';

CREATE TABLE documents_new (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  document_type TEXT NOT NULL
    CHECK (document_type IN (
      'resume',
      'cover_letter',
      'work_sample',
      'snippet',
      'portfolio',
      'job_posting',
      'other'
    )),
  storage_path TEXT NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

INSERT INTO documents_new (
  id,
  name,
  document_type,
  storage_path,
  notes,
  created_at,
  updated_at
)
SELECT
  id,
  name,
  document_type,
  storage_path,
  notes,
  created_at,
  updated_at
FROM documents;

CREATE TEMP TABLE application_documents_copy AS
SELECT
  application_id,
  document_id,
  attachment_type,
  submitted_at,
  notes,
  created_at
FROM application_documents;

DROP TABLE application_documents;
DROP TABLE documents;
ALTER TABLE documents_new RENAME TO documents;

CREATE TABLE application_documents_new (
  application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  attachment_type TEXT NOT NULL DEFAULT 'other'
    CHECK (attachment_type IN (
      'resume',
      'cover_letter',
      'work_sample',
      'portfolio',
      'job_posting',
      'other'
    )),
  submitted_at TEXT,
  notes TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  PRIMARY KEY (application_id, document_id, attachment_type)
);

INSERT INTO application_documents_new (
  application_id,
  document_id,
  attachment_type,
  submitted_at,
  notes,
  created_at
)
SELECT
  application_id,
  document_id,
  attachment_type,
  submitted_at,
  notes,
  created_at
FROM application_documents_copy;

ALTER TABLE application_documents_new RENAME TO application_documents;
DROP TABLE application_documents_copy;

CREATE INDEX idx_application_documents_application_id
  ON application_documents(application_id);
