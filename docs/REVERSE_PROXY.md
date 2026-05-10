# Reverse Proxy

The recommended pattern is to keep JobHunt OS bound to `127.0.0.1` on the host
and let a reverse proxy handle HTTPS and the public hostname.

JobHunt OS has optional built-in login authentication through
`JOBHUNT_AUTH_MODE=login`, `JOBHUNT_AUTH_USERNAME`, and
`JOBHUNT_AUTH_PASSWORD_HASH`. If the reverse proxy makes the app reachable by
other people or by the public internet, prefer built-in login authentication.
HTTP Basic auth remains available as a fallback, legacy, or simple mode, but it
is not the preferred mode for public deployments. A trusted proxy-level
authentication layer can also be used when that fits the deployment.

Built-in authentication is not transport security. It is suitable behind the
default localhost binding, behind an HTTPS reverse proxy, over a VPN, or over
another encrypted/trusted channel. Do not rely on it over plain HTTP on an
untrusted network.

For HTTPS reverse-proxy deployments, also set `JOBHUNT_SECURE_COOKIES=true`.
This marks CSRF, theme, and login session cookies as `Secure`, so browsers only
send them over HTTPS. Keep it disabled for plain HTTP localhost access.

Set `JOBHUNT_AUTH_TRUST_PROXY_HEADERS=true` only when the reverse proxy is
trusted and sanitizes forwarded headers before requests reach JobHunt OS. The
login throttle uses these headers to identify clients. Keep it disabled for
direct LAN access or untrusted proxy paths.

For remote or public access, the app environment should include:

```text
JOBHUNT_AUTH_MODE=login
JOBHUNT_AUTH_USERNAME=<username>
JOBHUNT_AUTH_PASSWORD_HASH='argon2id$v=19$m=19456,t=2,p=1$<salt-base64url>$<digest-base64url>'
JOBHUNT_SECURE_COOKIES=true
```

Store the real username and password hash in a local `.env`, CI variables,
Vault, or an equivalent secret store. Do not commit plaintext passwords or real
password hashes to a public repository.

The default Compose file already uses this host binding:

```yaml
ports:
  - "127.0.0.1:8080:8080"
```

Inside the container, `JOBHUNT_ADDR=0.0.0.0:8080` is expected. Docker needs the
process to listen on the container interface so it can forward traffic from the
host loopback port.

## Caddy Example

Install and configure Caddy on the host, then add a site like this:

```caddyfile
jobs.example.com {
	reverse_proxy 127.0.0.1:8080
}
```

Reload Caddy after changing the Caddyfile.

With this setup:

- Caddy listens publicly on `jobs.example.com`.
- Caddy manages HTTPS for the public site.
- JobHunt OS remains reachable only on `127.0.0.1:8080` from the host.
- Docker does not publish JobHunt OS directly on every network interface.
- `JOBHUNT_SECURE_COOKIES=true` is appropriate because users access the app
  over HTTPS.

If the hostname is not private, add authentication. Prefer JobHunt OS built-in
login auth in the app environment. If you intentionally use proxy-level fallback
auth instead, Caddy can handle it with `basicauth` and a hashed password:

```caddyfile
jobs.example.com {
	basicauth {
		<username> <hashed-password>
	}
	reverse_proxy 127.0.0.1:8080
}
```

Generate the password hash with Caddy tooling and store the Caddyfile according
to your host's operational policy.

## Brute-Force Protection

JobHunt OS includes a small in-process login throttle and temporary lockout. For
remote access, still put brute-force controls at the reverse proxy or host
firewall where they can see client IPs and block before requests reach the app.

For Nginx, a small baseline is:

```nginx
http {
	limit_req_zone $binary_remote_addr zone=jobhunt_auth:10m rate=5r/m;

	server {
		location / {
			limit_req zone=jobhunt_auth burst=10 nodelay;
			proxy_pass http://127.0.0.1:8080;
		}
	}
}
```

For Caddy, use access logs plus fail2ban, or a maintained Caddy rate-limit
plugin if your host already supports custom Caddy builds. A practical fail2ban
setup should watch the reverse proxy access log for repeated `401` responses
from the same client and ban that source at the host firewall for a short window
such as 10 to 30 minutes. Tune thresholds for your own access pattern so a
mistyped password does not lock you out.

## Avoid Public Docker Port Binding

Avoid changing the Compose port mapping to this unless you have a specific
reason and understand the exposure:

```yaml
ports:
  - "8080:8080"
```

That form can bind the app on all host interfaces. Prefer keeping the app on
`127.0.0.1` and exposing only the reverse proxy.

## Security Note

JobHunt OS is local-first and has no multi-user security model. If you expose it
to the internet, put it behind HTTPS, prefer `JOBHUNT_AUTH_MODE=login`, enable
secure cookies, add rate limiting or fail2ban-style blocking for repeated
failures, keep Docker and the host patched, and verify that the hostname is
intended to be reachable. Deployed non-loopback instances, including
firblab-v2/GitLab CI deployments, must use login auth. Keep secure cookies off
for direct plain-HTTP LAN access; turn them on when the app is accessed through
a trusted HTTPS reverse proxy.
