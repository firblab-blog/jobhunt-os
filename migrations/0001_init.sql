-- Planned initial schema. Not applied by the scaffold yet.
--
-- SQLite does not enforce foreign key constraints unless each connection enables:
--
--   PRAGMA foreign_keys = ON;
--
-- The future SQLite store must enable this immediately after opening every
-- database connection.

CREATE TABLE applications (
  id TEXT PRIMARY KEY,
  company TEXT NOT NULL,
  role TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'prospect'
    CHECK (status IN (
      'prospect',
      'applied',
      'interviewing',
      'offer',
      'accepted',
      'declined',
      'rejected',
      'withdrawn',
      'archived'
    )),
  priority TEXT NOT NULL DEFAULT 'normal'
    CHECK (priority IN ('low', 'normal', 'high')),
  source TEXT NOT NULL DEFAULT '',
  location TEXT NOT NULL DEFAULT '',
  comp_min_cents INTEGER
    CHECK (comp_min_cents IS NULL OR comp_min_cents >= 0),
  comp_max_cents INTEGER
    CHECK (comp_max_cents IS NULL OR comp_max_cents >= 0),
  comp_currency TEXT NOT NULL DEFAULT ''
    CHECK (
      comp_currency = ''
      OR (length(comp_currency) = 3 AND comp_currency = upper(comp_currency))
    ),
  comp_notes TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '',
  next_action_due TEXT,
  next_action_summary TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  CHECK (
    comp_min_cents IS NULL
    OR comp_max_cents IS NULL
    OR comp_min_cents <= comp_max_cents
  )
);

CREATE TABLE contacts (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  organization TEXT NOT NULL DEFAULT '',
  role TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL DEFAULT '',
  phone TEXT NOT NULL DEFAULT '',
  location TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE application_contacts (
  application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  contact_id TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
  relationship TEXT NOT NULL DEFAULT 'other'
    CHECK (relationship IN (
      'recruiter',
      'hiring_manager',
      'interviewer',
      'referrer',
      'teammate',
      'other'
    )),
  is_primary INTEGER NOT NULL DEFAULT 0 CHECK (is_primary IN (0, 1)),
  notes TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  PRIMARY KEY (application_id, contact_id)
);

CREATE TABLE application_events (
  id TEXT PRIMARY KEY,
  application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  contact_id TEXT REFERENCES contacts(id) ON DELETE SET NULL,
  event_type TEXT NOT NULL
    CHECK (event_type IN (
      'applied',
      'recruiter_screen',
      'phone_screen',
      'interview',
      'onsite',
      'take_home',
      'follow_up',
      'deadline',
      'offer',
      'decision',
      'note',
      'other'
    )),
  occurred_at TEXT NOT NULL,
  summary TEXT NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE documents (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  document_type TEXT NOT NULL
    CHECK (document_type IN (
      'resume',
      'cover_letter',
      'work_sample',
      'snippet',
      'portfolio',
      'other'
    )),
  storage_path TEXT NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE application_documents (
  application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
  attachment_type TEXT NOT NULL DEFAULT 'other'
    CHECK (attachment_type IN (
      'resume',
      'cover_letter',
      'work_sample',
      'portfolio',
      'other'
    )),
  submitted_at TEXT,
  notes TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
  PRIMARY KEY (application_id, document_id, attachment_type)
);

CREATE INDEX idx_applications_status
  ON applications(status);

CREATE INDEX idx_applications_updated_at
  ON applications(updated_at);

CREATE INDEX idx_applications_next_action_due
  ON applications(next_action_due);

CREATE INDEX idx_application_events_application_id
  ON application_events(application_id);

CREATE INDEX idx_application_events_occurred_at
  ON application_events(occurred_at);
