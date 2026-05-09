# Security Review

This public summary captures the current security posture for JobHunt OS
without publishing raw audit notes or private operational context.

## Scope

The review covers the public source tree, self-hosting documentation, container
build inputs, runtime data handling, and repository hygiene for a local-first
personal job hunt tracker.

## Summary

JobHunt OS is suitable for public source distribution as a localhost-first,
self-hosted app when users keep real personal data out of the repository and
protect any network-exposed deployment with authentication.

For the practical threat model, deployment tiers, and current security
boundaries, see [SECURITY.md](SECURITY.md).

No public fixtures should contain real resumes, cover letters, applications,
recruiter messages, correspondence, uploaded documents, or raw database files.
Synthetic fixtures should use the `sample-*.yaml` naming convention.

## Public Controls

- The default server address is loopback-only.
- Docker Compose publishes the app on `127.0.0.1:8080` by default.
- Optional built-in HTTP Basic authentication can protect shared or remote
  deployments when used over localhost, HTTPS, VPN, or another trusted channel.
- HTTPS reverse-proxy deployments can enable secure cookies with
  `JOBHUNT_SECURE_COOKIES=true`.
- State-changing forms use signed CSRF tokens.
- Document downloads are constrained to the configured data directory.
- PDF uploads are size-limited and checked for a PDF header.
- JSON exports and document downloads use `Cache-Control: no-store`.
- The container runs as a non-root user.
- Backup documentation recommends encrypted storage for long-lived or
  off-machine copies without requiring a specific paid service.
- Public fixtures are limited to synthetic samples.
- `.gitignore` and `.dockerignore` exclude local data directories, SQLite
  databases, non-sample YAML fixtures, local environment files, and secret
  directories.

## Contributor Guidance

- Do not commit local `.env` files, `.secrets/`, `secrets/`, `data/`, SQLite
  databases, uploaded documents, or private job hunt material.
- Keep credentials in local environment files, host secret stores, or CI secret
  variables rather than in source, Compose files, docs, screenshots, or logs.
- Redact names, email addresses, phone numbers, hostnames, tokens, private
  paths, and company-specific notes before sharing issue reports or support
  material.
- Review dependencies before adding them, especially dependencies that parse
  uploaded files or other untrusted input.

## Remaining Hardening

- Multi-user isolation.
- Built-in malware scanning for uploaded documents.
- Encrypted SQLite database storage at rest.
- Full restore from JSON export.
- Built-in backup encryption tooling, if the project later needs more than
  documented examples.
- Import validation and report-only dry runs.
- Container image signing and provenance attestations.
