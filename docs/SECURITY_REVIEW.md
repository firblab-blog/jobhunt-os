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
protect any network-exposed deployment with login authentication. Public or
remote access should add HTTPS and secure cookies; direct private-LAN HTTP
deployments should leave secure cookies off so browser sessions work.

For the practical threat model, deployment tiers, and current security
boundaries, see [SECURITY.md](SECURITY.md).

No public fixtures should contain real resumes, cover letters, applications,
recruiter messages, correspondence, uploaded documents, or raw database files.
Synthetic fixtures should use the `sample-*.yaml` naming convention.

## Public Controls

- The default server address is loopback-only.
- Loopback no-auth is allowed for desktop/local use; non-loopback no-auth is
  refused unless the explicit insecure no-auth escape hatch is enabled.
- Docker Compose publishes the app on `127.0.0.1:8080` by default.
- Built-in login authentication is the preferred app auth mode for shared or
  remote deployments when used over localhost, HTTPS, VPN, or another trusted
  channel. Basic auth remains fallback/legacy/simple mode.
- Login auth uses server-side sessions with idle and absolute timeouts,
  logout, and secure cookies when `JOBHUNT_SECURE_COOKIES=true`.
- Argon2id is preferred for password hashes; existing PBKDF2-SHA256 hashes are
  legacy-compatible.
- HTTPS reverse-proxy deployments can enable secure cookies with
  `JOBHUNT_SECURE_COOKIES=true`.
- Login throttling is implemented in the app, and reverse-proxy rate limiting
  or fail2ban-style blocking is still recommended for remote deployments.
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
- Keep credentials, plaintext passwords, and real password hashes in local
  ignored secret files, Docker secrets, host secret stores, Vault, or CI secret
  variables rather than in source, Compose files, docs, screenshots, or logs.
- Deployed non-loopback instances must use `JOBHUNT_AUTH_MODE=login`. Use HTTPS
  and `JOBHUNT_SECURE_COOKIES=true` when access goes through a trusted reverse
  proxy; keep secure cookies off for direct plain-HTTP LAN access.
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
