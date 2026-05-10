# Configuration

JobHunt OS has a small runtime configuration surface. The default setup is
local-first.

## Environment Variables

### `JOBHUNT_ADDR`

Address the HTTP server listens on.

Default when running the app directly:

```text
127.0.0.1:8080
```

The value must include both host and port, for example:

```sh
JOBHUNT_ADDR=127.0.0.1:9090
```

By default, non-loopback addresses such as `0.0.0.0:8080` are rejected unless
`JOBHUNT_ALLOW_NETWORK=true` is also set.

### `JOBHUNT_ALLOW_NETWORK`

Allows JobHunt OS to bind to a non-loopback address.

Default:

```text
false
```

Accepted values are Go-style booleans such as `true`, `false`, `1`, and `0`.

Use this only when you understand how the app is exposed. The Docker image sets
this to `true` because the process needs to listen on `0.0.0.0` inside the
container. The default Compose file still binds the host port to
`127.0.0.1`, so the app remains local to the host unless you change the port
mapping or place a reverse proxy in front of it.

### `JOBHUNT_DATA_DIR`

Directory where JobHunt OS stores its local data.

When running directly without this variable, the default depends on the
operating system:

- macOS: `~/Library/Application Support/jobhunt-os`
- Linux with `XDG_DATA_HOME`: `$XDG_DATA_HOME/jobhunt-os`
- Linux without `XDG_DATA_HOME`: `~/.local/share/jobhunt-os`
- Windows: `%APPDATA%\jobhunt-os`

When running with the provided Docker image, the default is:

```text
/data
```

The provided Compose file mounts that container path from a host directory
relative to `deploy/`:

```yaml
volumes:
  - ./data:/data
```

That means the data lives in `./data` next to your `docker-compose.yml` on the
host.

The Compose file also reads these optional values from `deploy/.env`:

```text
JOBHUNT_UID
JOBHUNT_GID
```

When set, the app container runs with that host UID/GID and the data-prep helper
sets `./data` ownership to match. On Linux, set:

```text
JOBHUNT_UID=<host-uid>
JOBHUNT_GID=<host-gid>
```

The prepared layout is:

```text
data/
  jobhunt-os.db
  documents/
  tmp/
```

### `JOBHUNT_AUTH_MODE`

Selects the built-in authentication mode.

Default:

```text
unset
```

Accepted values:

- `disabled`
- `login`
- `basic`

If unset, JobHunt OS uses `basic` when auth credentials are configured with
`JOBHUNT_AUTH_USERNAME` and either `JOBHUNT_AUTH_PASSWORD_FILE` or
`JOBHUNT_AUTH_PASSWORD_HASH`, and `disabled` otherwise. For new deployments,
prefer `JOBHUNT_AUTH_MODE=login`. Basic auth is a fallback, legacy, or simple
compatibility mode; it is not the preferred mode for public deployments.

Non-loopback binds refuse `JOBHUNT_AUTH_MODE=disabled` unless
`JOBHUNT_ALLOW_INSECURE_NO_AUTH=true` is set for a deployment protected
elsewhere. In practice, non-loopback no-auth needs both
`JOBHUNT_ALLOW_NETWORK=true` and `JOBHUNT_ALLOW_INSECURE_NO_AUTH=true`.
Loopback no-auth remains allowed for desktop and local-only use.

### `JOBHUNT_AUTH_USERNAME`

Username for built-in login or HTTP Basic authentication.

Default:

```text
unset
```

Authentication is disabled unless `JOBHUNT_AUTH_MODE` is `login` or `basic`.
With login auth, `/healthz`, `/login`, and static assets are public; app
routes, `/export.json`, and document download URLs require a valid session.
With basic auth, all routes except `/healthz` require credentials.

Built-in authentication does not encrypt the username, password, requests, or
responses. Use it only over `localhost`, an HTTPS reverse proxy, a VPN, or
another encrypted/trusted channel. Do not expose it over plain HTTP on an
untrusted network.

### `JOBHUNT_AUTH_PASSWORD_FILE`

Path to a file containing the password for built-in login or HTTP Basic
authentication.

Default:

```text
unset
```

The Docker Compose install sets this inside the container:

```text
JOBHUNT_AUTH_PASSWORD_FILE=/run/secrets/jobhunt_admin_password
```

The Compose secret source defaults to this host-side file from `deploy/`:

```text
.secrets/admin-password
```

Put only the password or passphrase in that file. A trailing newline is ignored
so text editors and `printf`-style file creation remain usable. At startup,
JobHunt OS reads the password file, validates the password policy, hashes the
password in memory with Argon2id, and uses the hash for authentication.

`JOBHUNT_AUTH_PASSWORD_FILE` and `JOBHUNT_AUTH_PASSWORD_HASH` are mutually
exclusive. Prefer the password file for Docker Compose installs.

### `JOBHUNT_AUTH_PASSWORD_HASH`

Password hash for built-in login or HTTP Basic authentication.

Default:

```text
unset
```

This is mainly for existing installs, CI/deployment secret stores that already
manage hashed values, or non-Compose deployments. Argon2id is the preferred hash
format:

```text
argon2id$v=19$m=19456,t=2,p=1$<salt-base64url>$<digest-base64url>
```

Existing PBKDF2-SHA256 hashes remain supported for compatibility if your
install already landed with one:

```text
pbkdf2-sha256$<iterations>$<salt-base64url>$<digest-base64url>
```

Password policy is intentionally length-focused:

- 15 or more characters
- passphrases are encouraged
- no required uppercase, lowercase, number, or symbol composition rules
- no forced periodic rotation; change the password when it may be exposed or
  when an operator should no longer have access

The built-in Argon2id hash helper enforces this policy when it creates a hash.
If you generate a supported hash with another tool, choose a password that
meets the same policy; the app cannot recover password length or composition
from an already-generated hash.

Do not commit plaintext passwords or real password hashes to public
repositories. Store runtime secrets in local ignored files, Docker secrets, CI
variables, Vault, or an equivalent secret store for the deployment environment.

### `JOBHUNT_SECURE_COOKIES`

Adds the `Secure` attribute to CSRF, theme, and login session cookies.

Default:

```text
false
```

Accepted values are Go-style booleans such as `true`, `false`, `1`, and `0`.

Set this to `true` when JobHunt OS is served through an HTTPS reverse proxy.
Leave it unset or `false` for plain HTTP localhost access, because browsers do
not send `Secure` cookies over HTTP.

### `JOBHUNT_AUTH_TRUST_PROXY_HEADERS`

Trusts reverse-proxy headers for client identity used by login throttling.

Default:

```text
false
```

Set this to `true` only when JobHunt OS is behind a trusted reverse proxy that
sets and sanitizes `X-Forwarded-For` or `X-Real-IP`. Keep it `false` for direct
LAN, direct internet, or untrusted proxy paths.

### `JOBHUNT_SESSION_IDLE_TIMEOUT`

Maximum idle time for login sessions.

Default:

```text
12h
```

Values use Go duration syntax such as `30m`, `12h`, or day shorthand such as
`7d`. Login auth uses server-side sessions stored in the JobHunt OS SQLite
database. A valid request touches the session and refreshes the idle window.

### `JOBHUNT_SESSION_ABSOLUTE_TIMEOUT`

Maximum total lifetime for login sessions.

Default:

```text
30d
```

Values use the same duration format as `JOBHUNT_SESSION_IDLE_TIMEOUT`.
When this absolute timeout is reached, the user must sign in again even if the
session was active. The logout form revokes the current server-side session and
clears the browser session cookies.

## Local Defaults vs Container Defaults

When running the Go binary directly, JobHunt OS defaults to private local
settings:

- listens on `127.0.0.1:8080`
- rejects non-loopback binds unless `JOBHUNT_ALLOW_NETWORK=true`
- chooses a per-user data directory for the operating system

When running the container image, the image defaults are container-oriented:

- listens on `0.0.0.0:8080` inside the container
- sets `JOBHUNT_ALLOW_NETWORK=true`
- stores data under `/data`

The Compose file combines those container defaults with a host-side loopback
port binding. This is why `0.0.0.0` inside the container does not mean the app is
automatically exposed on the host network.

Because the process binds non-loopback inside the container, the provided
Compose file defaults to `JOBHUNT_AUTH_MODE=login`, requires
`JOBHUNT_AUTH_USERNAME` from `deploy/.env`, and reads the password from the
configured Docker secret file. Deployed non-loopback instances must use login
auth. Set `JOBHUNT_SECURE_COOKIES=true` when a trusted reverse proxy serves the
app over HTTPS; keep `JOBHUNT_SECURE_COOKIES=false` for direct plain-HTTP LAN
access.

## Settings Page

The in-app Settings page is available at:

```text
/settings
```

As of `v0.1.9`, it contains:

- theme selection: system, light, and dark
- JSON export download

The legacy `/backup` route redirects to `/settings`. Theme selection is stored
in the `jobhunt_theme` cookie.

## Configuration Scope

JobHunt OS does not currently include account management, SMTP settings, OAuth,
object storage, background workers, or a separate management service.
