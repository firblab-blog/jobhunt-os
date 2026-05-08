# Roadmap

## Phase 0: Planning Baseline

- Capture what is salvageable from the old platform.
- Keep old application YAMLs and document templates as private historical fixtures.
- Build synthetic data that looks real enough for UI work.
- Set dependency policy before adding SQLite or frontend tooling.

## Phase 1: Manual Tracker

- Application CRUD on the initial SQLite schema.
- Status and timeline model with constrained status/event vocabularies.
- Contacts and correspondence notes linked to applications and timeline entries.
- Follow-up reminders inside the UI using application-level next-action fields.
- Document inventory and application attachments through join records.
- CSV and JSON export.

## Phase 2: Durable Local Storage

- SQLite-backed store.
- Embedded migrations.
- Backup and restore.
- Import from legacy YAML.
- Redaction tooling for generating public demo fixtures.

## Phase 3: Installability

- Docker Compose as the canonical self-hosted install path.
- Public container image with `latest`, versioned, and `sha-<shortsha>` tags.
- Clear data directory conventions.
- First-run setup page.
- Optional binary releases later if they prove useful.

## Phase 4: Optional Automation

- Manual correspondence entry first.
- Optional email metadata import only after the local workflow is solid.
- No automatic outbound communication.
- Any AI-assisted drafting must be opt-in, local-data-aware, and easy to disable.
