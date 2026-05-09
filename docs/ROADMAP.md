# Roadmap

## Current Scope in `v0.1.4`

- Application CRUD backed by SQLite.
- Constrained application statuses, priorities, and timeline event types.
- Contacts linked to applications and timeline entries.
- Application next actions shown on the Applications page.
- Document inventory with PDF upload, validation, storage, preview, and download.
- Application posting PDF attachments.
- Dashboard pipeline pulse with Sankey graph and signal strip.
- Applications page Sankey graph above the application list.
- Settings page with theme selection and JSON export.
- Docker Compose install path with local `./data` storage.
- Public container image tags: `latest`, `vX.Y.Z`, and `sha-<shortsha>`.

## Next Product Work

- Import from legacy YAML.
- CSV export if needed in addition to JSON export.
- Redaction tooling for generating public demo fixtures.
- First-run setup page if installation requires user-entered settings.
- Structured status transition history for historical Sankey analytics.
- Authentication option or documented reverse-proxy authentication pattern for
  network-exposed deployments.

## Optional Automation

- Manual correspondence entry remains the baseline.
- Email metadata import should be optional and read-only.
- No automatic outbound communication.
- AI-assisted drafting, if added, must be opt-in and disableable.
