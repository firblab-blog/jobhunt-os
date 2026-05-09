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
sets `./data` ownership to match. On Linux, create them with:

```sh
printf "JOBHUNT_UID=%s\nJOBHUNT_GID=%s\n" "$(id -u)" "$(id -g)" > .env
```

The prepared layout is:

```text
data/
  jobhunt-os.db
  documents/
  tmp/
```

### `JOBHUNT_AUTH_USERNAME`

Optional username for built-in HTTP Basic authentication.

Default:

```text
unset
```

Authentication is disabled unless both `JOBHUNT_AUTH_USERNAME` and
`JOBHUNT_AUTH_PASSWORD_HASH` are set. When enabled, all routes except
`/healthz` require credentials, including `/export.json` and document download
URLs.

HTTP Basic authentication does not encrypt the username, password, requests, or
responses. Use the built-in auth only over `localhost`, an HTTPS reverse proxy,
a VPN, or another encrypted/trusted channel. Do not expose it over plain HTTP on
an untrusted network.

### `JOBHUNT_AUTH_PASSWORD_HASH`

Optional PBKDF2-SHA256 password hash for built-in HTTP Basic authentication.

Default:

```text
unset
```

The value must use this format:

```text
pbkdf2-sha256$<iterations>$<salt-base64url>$<digest-base64url>
```

Generate a hash locally without putting the plaintext password in shell history:

```sh
JOBHUNT_AUTH_PASSWORD_HASH="$(python3 - <<'PY'
import base64
import getpass
import hashlib
import secrets

password = getpass.getpass("JobHunt OS password: ")
salt = secrets.token_bytes(16)
iterations = 210000
digest = hashlib.pbkdf2_hmac("sha256", password.encode("utf-8"), salt, iterations, dklen=32)
encode = lambda value: base64.urlsafe_b64encode(value).rstrip(b"=").decode("ascii")
print(f"pbkdf2-sha256${iterations}${encode(salt)}${encode(digest)}")
PY
)"
```

Then store only the username and hash in the runtime environment, for example
in a local `.env` file used by Docker Compose:

```text
JOBHUNT_AUTH_USERNAME=<username>
JOBHUNT_AUTH_PASSWORD_HASH='pbkdf2-sha256$210000$<salt-base64url>$<digest-base64url>'
```

The single quotes matter for Docker Compose `.env` files because password
hashes contain `$`. Alternatively, escape each dollar sign as `$$` in unquoted
examples.

### `JOBHUNT_SECURE_COOKIES`

Adds the `Secure` attribute to CSRF and theme cookies.

Default:

```text
false
```

Accepted values are Go-style booleans such as `true`, `false`, `1`, and `0`.

Set this to `true` when JobHunt OS is served through an HTTPS reverse proxy.
Leave it unset or `false` for plain HTTP localhost access, because browsers do
not send `Secure` cookies over HTTP.

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
