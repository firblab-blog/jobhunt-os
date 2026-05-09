# Reverse Proxy

The recommended pattern is to keep JobHunt OS bound to `127.0.0.1` on the host
and let a reverse proxy handle HTTPS and the public hostname.

JobHunt OS has no built-in authentication. If the reverse proxy makes the app
reachable by other people or by the public internet, add authentication at the
proxy layer.

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

If the hostname is not private, add authentication. For example, Caddy can use
`basicauth` with a hashed password:

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
to the internet, put it behind a reverse proxy with authentication, keep Docker
and the host patched, and verify that the hostname is intended to be reachable.
