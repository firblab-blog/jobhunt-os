# Security

## Dependency Policy

Dependencies are allowed when they reduce risk or remove meaningful
implementation complexity.

Before adding a dependency, record:

- what it does
- why the standard library is not enough
- its maintenance status
- its transitive dependency count
- whether it handles untrusted input
- what the removal path would be

## Data Handling

- Do not commit real resumes, cover letters, applications, recruiter messages, or personal correspondence.
- Use synthetic fixtures in the public repository.
- Treat legacy data as private input for import testing unless explicitly redacted.
- Bind the server to `127.0.0.1` by default.
- Store runtime data in the configured data directory.
- Back up the data directory before upgrades.

## Current Controls in `v0.1.4`

- The server rejects non-loopback binds unless `JOBHUNT_ALLOW_NETWORK=true`.
- Docker Compose binds the host port to `127.0.0.1:8080` by default.
- SQLite foreign key enforcement is enabled through the SQLite DSN.
- State-changing forms use HMAC-signed CSRF tokens.
- CSRF and theme cookies are `HttpOnly` where applicable and `SameSite=Lax`.
- PDF uploads are capped at 20 MB and checked for a `%PDF-` header.
- Document downloads are constrained to paths under the configured data
  directory.
- Security headers are set on responses.
- The container runs as a non-root user.

## Authentication Boundary

JobHunt OS has no built-in user accounts or authentication. Anyone who can
reach the HTTP port can use the app and download the JSON export. Keep the app
bound to localhost, or add authentication at the reverse proxy before exposing
it beyond the host.

## Future Hardening Checklist

- Built-in authentication or documented proxy-auth examples.
- Optional secure-cookie mode for HTTPS reverse-proxy deployments.
- `Cache-Control: no-store` on export and document download endpoints.
- `Permissions-Policy` response header.
- Backup encryption option.
- Import validation and report-only dry runs.
- SBOM generation for releases.
