# Roadmap

## Phase 0: Planning Baseline

- Capture what is salvageable from the old platform.
- Keep old application YAMLs and document templates as private historical fixtures.
- Build synthetic data that looks real enough for UI work.
- Set dependency policy before adding SQLite or frontend tooling.

## Phase 1: Manual Tracker

- Application CRUD.
- Status and timeline model.
- Contacts and correspondence notes.
- Follow-up reminders inside the UI.
- Document inventory and application attachments.
- CSV and JSON export.

## Phase 2: Durable Local Storage

- SQLite-backed store.
- Embedded migrations.
- Backup and restore.
- Import from legacy YAML.
- Redaction tooling for generating public demo fixtures.

## Phase 3: Installability

- Single-binary releases for macOS, Linux, and Windows.
- Docker image for users who prefer containers.
- Clear data directory conventions.
- First-run setup page.

## Phase 4: Optional Automation

- Manual correspondence entry first.
- Optional email metadata import only after the local workflow is solid.
- No automatic outbound communication.
- Any AI-assisted drafting must be opt-in, local-data-aware, and easy to disable.
