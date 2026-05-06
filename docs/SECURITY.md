# Security

## Dependency Policy

Dependencies are allowed, but they need a reason. The default is no dependency until the project can name the risk it removes or the complexity it prevents.

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

## Future Hardening Checklist

- CSRF protection before accepting state-changing browser requests beyond localhost-only development.
- File upload validation before document management.
- Backup encryption option.
- Import validation and report-only dry runs.
- SBOM generation for releases once third-party dependencies exist.
