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

The provided Compose file mounts that container path from a host directory:

```yaml
volumes:
  - ./data:/data
```

That means the data lives in `./data` next to your `docker-compose.yml` on the
host.

The Compose file also reads these optional values from `.env`:

```text
JOBHUNT_UID
JOBHUNT_GID
```

When set, the app container runs with that host UID/GID and the data-prep helper
sets `./data` ownership to match. On Linux, add them with:

```sh
printf "JOBHUNT_UID=%s\nJOBHUNT_GID=%s\n" "$(id -u)" "$(id -g)" >> .env
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

If unset, JobHunt OS uses `basic` when both `JOBHUNT_AUTH_USERNAME` and
`JOBHUNT_AUTH_PASSWORD_HASH` are set, and `disabled` otherwise. For new
deployments, prefer `JOBHUNT_AUTH_MODE=login`. Basic auth is a fallback,
legacy, or simple compatibility mode; it is not the preferred mode for public
deployments.

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

### `JOBHUNT_AUTH_PASSWORD_HASH`

Password hash for built-in login or HTTP Basic authentication.

Default:

```text
unset
```

Argon2id is the preferred hash format:

```text
argon2id$v=19$m=19456,t=2,p=1$<salt-base64url>$<digest-base64url>
```

Generate an Argon2id hash locally without putting the plaintext password in
shell history. This command uses Python's `argon2-cffi` package:

```sh
JOBHUNT_AUTH_PASSWORD_HASH="$(python3 - <<'PY'
import base64
import getpass
import secrets
import sys

from argon2.low_level import Type, hash_secret_raw

password = getpass.getpass("JobHunt OS password: ")
if len(password) < 15 or len(password) > 1024 or not password.isprintable():
    sys.exit("Password must be 15-1024 printable characters.")
salt = secrets.token_bytes(16)
digest = hash_secret_raw(
    password.encode("utf-8"),
    salt,
    time_cost=2,
    memory_cost=19456,
    parallelism=1,
    hash_len=32,
    type=Type.ID,
    version=19,
)
encode = lambda value: base64.urlsafe_b64encode(value).rstrip(b"=").decode("ascii")
print(f"argon2id$v=19$m=19456,t=2,p=1${encode(salt)}${encode(digest)}")
PY
)"
```

Existing PBKDF2-SHA256 hashes remain supported for compatibility if your
install already landed with one:

```text
pbkdf2-sha256$<iterations>$<salt-base64url>$<digest-base64url>
```

Then store only the username and hash in the runtime environment, for example
in a local `.env` file used by Docker Compose:

```text
JOBHUNT_AUTH_MODE=login
JOBHUNT_AUTH_USERNAME=<username>
JOBHUNT_AUTH_PASSWORD_HASH='argon2id$v=19$m=19456,t=2,p=1$<salt-base64url>$<digest-base64url>'
```

The single quotes matter for Docker Compose `.env` files because password
hashes contain `$`. Alternatively, escape each dollar sign as `$$` in unquoted
examples.

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
repositories. Store runtime secrets in local `.env` files, CI variables, Vault,
or an equivalent secret store for the deployment environment.

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
Compose file defaults to `JOBHUNT_AUTH_MODE=login` and requires
`JOBHUNT_AUTH_USERNAME` and `JOBHUNT_AUTH_PASSWORD_HASH` from the local `.env`.
Deployed non-loopback instances, including firblab-v2/GitLab CI deployments,
must use login auth. Set `JOBHUNT_SECURE_COOKIES=true` when a trusted reverse
proxy serves the app over HTTPS; keep `JOBHUNT_SECURE_COOKIES=false` for direct
plain-HTTP LAN access.

## Settings Page

The in-app Settings page is available at:

```text
/settings
```

As of `v0.1.4`, it contains:

- theme selection: system, light, and dark
- JSON export download

The legacy `/backup` route redirects to `/settings`. Theme selection is stored
in the `jobhunt_theme` cookie.

## Configuration Scope

JobHunt OS does not currently include account management, SMTP settings, OAuth,
object storage, background workers, or a separate management service.
