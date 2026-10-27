# Security policy

## Reporting a vulnerability

Report it privately through GitHub security advisories for `wayju/FleetWatch`: open the repository's **Security** tab and choose **Report a vulnerability**. Please do not open a public issue or pull request for a vulnerability.

Include the version (`GET /version`), how you run it, what you did and what happened. A proof of concept helps. You can expect an acknowledgement within a week. The maintainers will work with you on a fix and will credit you in the advisory unless you prefer otherwise. Reports are handled by a small team on a best-effort basis, so there is no guaranteed fix time.

## Supported versions

| Version | Supported |
|---|---|
| 0.1.x | yes |
| earlier | no |

Fixes land on the latest release only while the project is pre-1.0.

## Threat model in brief

FleetWatch reads every agent transcript on a machine and serves prompts, replies and file paths over HTTP. It assumes a single trusted user on the machine and a network it does not trust.

- It listens on loopback by default, and the data routes need a bearer token. With no token available they answer 503 rather than serve open.
- Requests must carry an allowed `Host` header, which defeats DNS rebinding from a web page.
- Free text is redacted and clipped, and `show_text: false` removes it altogether.
- Adapters are read-only. FleetWatch never sends keys, text or prompts to a session.
- It serves plain HTTP. Anything beyond loopback needs a TLS reverse proxy, and it must never be put on a public interface.
- It does not defend against another process running as the same user, who can already read the transcripts and the token.

The full model, including what is read, what leaves the process and the deployment rules, is in [docs/security.md](docs/security.md).

## Scope

In scope: authentication and authorisation bypass, disclosure of transcript content or credentials beyond what the configuration allows, reads or writes outside the documented files, denial of service from a single unauthenticated request, and a way to make an adapter write to a session.

Out of scope: an attacker who already has code execution as the same user, exposure caused by binding to a public interface against the documentation, and the content of the agents' own transcripts.
