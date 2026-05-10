# Security

For a concise public posture summary, see
[SECURITY_REVIEW.md](SECURITY_REVIEW.md).

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
- Back up the data directory before upgrades and encrypt long-lived or
  off-machine backups when practical.
- Treat JSON exports as sensitive job-hunt data, not as harmless diagnostics.

## Deployment Tiers

Use the smallest exposure that fits the install.

- Localhost-only: personal use on one machine. This is the default and
  recommended starting point. Bind to `127.0.0.1`, keep the default Compose
  port mapping, and protect the local account and disk like any other personal
  data store.
- Private LAN: use from trusted devices on a home or private network. Set
  `JOBHUNT_ALLOW_NETWORK=true` only when the LAN is trusted, use VPN or another
  trusted channel when practical, and assume anyone on that network may try the
  HTTP port. Built-in auth does not protect credentials from passive
  observers on plain HTTP networks.
- Internet behind authenticated reverse proxy: remote access through a public
  hostname. Keep JobHunt OS bound to `127.0.0.1`, expose only the reverse
  proxy, require `JOBHUNT_AUTH_MODE=login` or authentication at a trusted proxy
  layer, use HTTPS, set `JOBHUNT_SECURE_COOKIES=true`, and keep the host,
  Docker, and proxy patched.

Do not expose an unauthenticated JobHunt OS port directly to the internet.

## Threat Model

JobHunt OS is a local-first personal application. The main assets are the
SQLite database, uploaded documents, JSON exports, backups, credentials, and
private notes about applications, contacts, interviews, and recruiters.

The practical threats are:

- Local attacker: someone with access to the host account, filesystem, Docker
  socket, browser profile, or terminal history can read or alter JobHunt OS
  data. JobHunt OS does not defend against a compromised host.
- LAN exposure: if the app binds beyond loopback, anyone who can reach that
  address can attempt to use it. Without authentication, network reachability is
  effectively app access.
- Internet exposure: public access increases credential guessing, reverse proxy
  misconfiguration, host patching, and log hygiene risks. Put internet-facing
  installs behind HTTPS and authentication.
- Malicious uploaded PDFs: uploads are size-limited and checked for a PDF
  header, but JobHunt OS does not scan for malware or make PDFs safe to open in
  other readers. Treat files from job boards, recruiters, and unknown senders as
  untrusted.
- Compromised image tags: mutable tags such as `latest` can move, and any
  registry account or CI compromise could publish a bad image. Pin versioned or
  commit-specific tags for production-ish installs, review release notes, and
  use SBOM/image scan results as release evidence rather than a guarantee.
- Leaked backups: data directory archives and JSON exports can contain resumes,
  contact details, recruiter messages, notes, and timelines. Encrypt
  long-lived or off-machine backups, restrict access, and test restores without
  sharing real data.

## Current Security Boundaries

JobHunt OS does not provide these protections yet:

- Multi-user isolation. The app is designed for one trusted owner, not multiple
  mutually untrusted users sharing one instance.
- Built-in malware scanning for uploaded documents.
- Encrypted SQLite database storage at rest. Use host disk encryption or
  encrypted backups when that matters.
- Full restore from JSON export. Use data directory backups for disaster
  recovery and complete restores.

## Current Controls

- The server rejects non-loopback binds unless `JOBHUNT_ALLOW_NETWORK=true`.
- Non-loopback no-auth is refused unless
  `JOBHUNT_ALLOW_INSECURE_NO_AUTH=true` is also set. Loopback no-auth remains
  allowed for desktop and local-only use.
- Docker Compose binds the host port to `127.0.0.1:8080` by default.
- Built-in authentication is optional and disabled by default. New shared,
  remote, or reverse-proxy deployments should use `JOBHUNT_AUTH_MODE=login`
  with `JOBHUNT_AUTH_USERNAME` and `JOBHUNT_AUTH_PASSWORD_HASH`. Basic auth is
  retained as fallback, legacy, or simple compatibility mode, but it is not the
  preferred mode for public deployments. This is an access-control layer, not
  encryption; use it only over localhost, HTTPS, VPN, or another
  encrypted/trusted channel.
- Login auth uses server-side SQLite sessions. Browser cookies carry only a
  session token; the database stores a token hash plus session timestamps.
  `JOBHUNT_SESSION_IDLE_TIMEOUT` controls the idle timeout and
  `JOBHUNT_SESSION_ABSOLUTE_TIMEOUT` controls the maximum session lifetime.
  Logout revokes the current server-side session and clears the session
  cookies.
- SQLite foreign key enforcement is enabled through the SQLite DSN.
- State-changing forms use HMAC-signed CSRF tokens.
- CSRF, theme, and login session cookies are `HttpOnly` where applicable and
  `SameSite=Lax`; `JOBHUNT_SECURE_COOKIES=true` also marks them `Secure` for
  HTTPS reverse-proxy deployments. With secure cookies enabled, the login
  session cookie uses the `__Host-` cookie prefix.
- PDF uploads are capped at 20 MB and checked for a `%PDF-` header.
- JobHunt OS validates this basic PDF shape, but it does not malware-scan PDFs.
- Document downloads are constrained to paths under the configured data
  directory.
- JSON export and document download responses use `Cache-Control: no-store`.
- Security headers are set on responses, including `Permissions-Policy` and a
  CSP with `object-src 'none'`.
- The container runs as a non-root user.
- The Dockerfile build image is pinned to an immutable `golang:1.26.3` digest,
  and the Compose data-prep helper is pinned to an immutable `busybox:1.37.0`
  digest.
- CI verifies Go module checksums, runs `go vet`, tests with the race detector,
  and runs `govulncheck` with a pinned `golang.org/x/vuln` tool version.
- CI Docker image builds use digest-pinned `docker:28.0.1` and
  `docker:28.0.1-dind` images with TLS-enabled Docker-in-Docker on port 2376.
  The runner must support sharing the generated `/certs/client` certificates
  between the Docker service and job container. The Docker API is
  privileged-equivalent, so plaintext DinD is acceptable only on isolated,
  trusted runners with no untrusted jobs sharing the Docker network.
- Release image pipelines generate a CycloneDX SBOM and a GitLab container
  scanning report with a pinned Trivy image. The image scan fails on fixed
  critical vulnerabilities.

## CI/CD Secret Handling

CI/CD variables that contain credentials must be masked, protected, and scoped
to the smallest practical set of jobs and branches. Do not enable debug shell
tracing on release, mirror, publish, or deploy jobs because expanded environment
values can appear in logs before tools can redact them.

Keep registry credentials, release tokens, deployment hostnames, and deployment
topology values in CI secret storage rather than in source, Compose files, docs,
screenshots, or logs. Use fine-grained tokens where possible, and prefer
credential helpers or standard input over embedding secrets in command-line
arguments or remote URLs.

If a pipeline triggers downstream deployment or publishing jobs, forward only
the variables required for that job. Treat downstream projects, runners, and
logs as trusted at the same level as the source pipeline.

## Authentication Boundary

When running the Go binary directly, JobHunt OS remains no-auth by default for
local-only use on `127.0.0.1`. Anyone who can reach the HTTP port can use the
app when authentication is not configured. Non-loopback no-auth is refused
unless `JOBHUNT_ALLOW_INSECURE_NO_AUTH=true` is set, in addition to
`JOBHUNT_ALLOW_NETWORK=true`. The provided Compose file requires login auth
because the app process binds non-loopback inside the container.

For shared hosts, remote access, or reverse-proxy deployments, enable built-in
login authentication with:

- `JOBHUNT_AUTH_MODE=login`
- `JOBHUNT_AUTH_USERNAME`
- `JOBHUNT_AUTH_PASSWORD_HASH`

The password hash uses this format:

```text
argon2id$v=19$m=19456,t=2,p=1$<salt-base64url>$<digest-base64url>
```

Argon2id is preferred for new hashes. Existing PBKDF2-SHA256 hashes remain
supported for compatibility if your install already uses one:

```text
pbkdf2-sha256$<iterations>$<salt-base64url>$<digest-base64url>
```

Use passwords or passphrases with at least 15 characters. Passphrases are
encouraged. JobHunt OS does not require uppercase/lowercase/number/symbol
composition rules and does not force periodic password rotation; rotate when a
password may have been exposed or when access should be removed. The built-in
Argon2id hash helper enforces this policy when it creates a hash; if you
generate a supported hash with another tool, choose a password that meets the
same policy because the app cannot recover password strength from the hash.

Do not store plaintext passwords or real password hashes in the repository,
Compose file, screenshots, docs, or other public material. Prefer a local
`.env` file, CI variables, Vault, systemd environment file, or another secret
manager appropriate for the host.

Use built-in auth only over localhost, HTTPS, VPN, or another encrypted/trusted
channel. Set `JOBHUNT_SECURE_COOKIES=true` when users access the app through an
HTTPS reverse proxy. Leave it `false` for direct plain-HTTP LAN access, because
browsers do not send `Secure` cookies over HTTP. Set
`JOBHUNT_AUTH_TRUST_PROXY_HEADERS=true` only behind a trusted reverse proxy that
sanitizes forwarded headers, because those headers are used for login
throttling client identity. If the app is reachable over plain HTTP by other
people, credentials and private job-hunt data can be observed in transit.
Deployed non-loopback instances, including firblab-v2 GitLab CI deployments,
must use login auth; HTTPS plus secure cookies is the preferred remote-access
posture when a trusted reverse proxy is in front.

## Brute-Force Protection

Login auth includes a small in-process throttle and temporary lockout for
repeated failed sign-in attempts. For remote access, still add reverse-proxy or
host-level controls:

- Rate-limit requests to the app or to authenticated paths at the proxy.
- Watch proxy access logs for repeated failed login responses and ban abusive
  clients with fail2ban or the host firewall.
- Keep the app bound to `127.0.0.1` so clients cannot bypass the proxy controls.
- Use long, unique passwords and rotate the hash if logs or backups suggest a
  credential may have leaked.

Start with conservative thresholds, such as a handful of failed attempts per
minute with a short temporary ban, then tune for your own access pattern.

## PDF Preview Sandbox

The document detail page previews PDFs in an iframe with
`sandbox="allow-same-origin allow-scripts"`. Browser PDF viewers are active
viewer implementations rather than plain HTML rendering, and common desktop
browsers need script-capable iframe contexts for the embedded viewer controls to
work reliably. The preview response is limited to same-origin framing, uses
`Cache-Control: no-store`, and serves only validated PDF-shaped uploads from the
configured data directory.

Do not remove `allow-scripts` unless browser testing confirms PDF previews still
render in the supported browsers. The direct-download link remains available
when a browser blocks embedded PDF viewing.

## Future Hardening Checklist

- Built-in backup encryption tooling, if the project later needs more than
  documented examples.
- Import validation and report-only dry runs.
- Container image signing and provenance attestations.
