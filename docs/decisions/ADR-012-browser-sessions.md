# ADR-012: Browser sessions

Status: accepted

## Context

The board is served to clients that send `Authorization: Bearer <token>`. A browser cannot do that for the two requests the page depends on: `EventSource` has no way to set a header, and a token carried in the URL ends up in server logs, proxy logs, history and the `Referer` of anything the page links to. A page that cannot authenticate has nothing to show, so a standalone install had no way to look at its own board.

## Decision

The page and its assets are static and carry no data, so `GET /` and `GET /ui/*` need no credential. They are served with a policy that admits only same-origin script and style, forbids framing and limits connections to the origin, plus `X-Content-Type-Options: nosniff` and `Referrer-Policy: no-referrer`.

A browser trades the token once for a cookie:

- `POST /v1/session` takes `{"token": "..."}`, compares it to the configured token in constant time and, on a match, sets `fleet_session` to a random 256-bit id. The cookie is `HttpOnly`, `SameSite=Strict` and `Path=/v1`, gains `Secure` when the request came over TLS (directly, or through a proxy that sets `X-Forwarded-Proto: https`), and lasts 12 hours. It never holds the token.
- Sessions live in memory with an absolute expiry and a cap of 64. At the cap, expired entries go first and then the one nearest expiry. A restart signs everyone out.
- `POST` and `DELETE` on `/v1/session` are refused with 403 unless `Origin` is present and names the request's own `Host`. This is the CSRF control for the one state-changing route; `SameSite=Strict` backs it.
- `DELETE /v1/session` removes the session and clears the cookie.
- `/v1/fleet` and `/v1/events` accept either the bearer header or the cookie. Every other path, including the unversioned forms, takes the bearer only, which also keeps the cookie's reach to its path.
- `ui.enabled: false` does not register the page or the session route, so they answer 404, and the guard never consults a session store.

## Consequences

- The token is typed into the page once per session and then never leaves the request that carried it. Cookie-bearing requests are `GET`s only, so the cookie cannot be used to change anything.
- A browser on the same machine as another user's process is as exposed as before: the token protects the network path, not the account.
- Failed logins are limited to 5 per remote address per minute, then 429 until the minute is up. The address is the TCP peer, so behind a proxy all clients share one; the proxy may add its own limit. A token under 32 bytes logs a warning at start. Token comparison goes through SHA-256 digests, so length does not show in timing.
- A cookie is host-wide, not per port: browsers do not isolate cookies by port, so another service on the same host name (for example another localhost port) can see `fleet_session`. It is `HttpOnly` and `Path=/v1`, but that does not hide it from other servers on the host. Run FleetWatch on its own host name if that matters.
- Behind a TLS-terminating proxy, the proxy must send `X-Forwarded-Proto: https`; otherwise the cookie is issued without `Secure`.
- A stream opened with the cookie ends when its session does, at logout or expiry. The cookie is honoured only on `/v1/fleet` and `/v1/events`.
- Behind a proxy, the proxy's public host must be in `allowed_hosts` and must pass `Host` and `Origin` through unchanged, or login answers 403.
- The page is built from `web/` and committed under `internal/ui/dist`, so the Go build needs no Node and the binary stays standard-library only (ADR-006). A host replaces the colours and fonts by naming a stylesheet in `ui.theme_file`; the structure stylesheet is not configurable.
