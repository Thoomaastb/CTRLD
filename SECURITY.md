# Security Policy

## Supported versions

| Version | Security updates |
|---|---|
| `main` (pre-alpha) | Yes |
| Older branches | No |

Once v1.0.0 is released, we will maintain security patches for the latest minor version of each major version.

---

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

If you discover a security vulnerability, please report it responsibly:

**Email:** security@ctrld.io

Include in your report:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if you have one)

We will acknowledge your report within **48 hours** and aim to provide a fix within **14 days** for critical issues.

We will credit you in the release notes unless you prefer to remain anonymous.

---

## Security design principles

CTRLD is built with security as a core architectural principle, not an afterthought:

- **Zero Trust** — every request is authenticated and authorized, no implicit trust
- **Least Privilege** — minimal permissions by default
- **PIM** — time-limited elevated access with mandatory MFA re-authentication
- **Append-only audit log** — all actions are logged and cannot be deleted
- **No root required** — CTRLD runs as a dedicated system user
- **Spoke isolation** — spoke servers open no inbound ports; all communication is outbound to the hub

---

## Known security considerations

- CTRLD should only be exposed over HTTPS — never plain HTTP
- The `ctrld` system user should have no login shell
- Database files contain sensitive data — ensure appropriate filesystem permissions
- Hub-Spoke mTLS certificates should be rotated periodically (tooling planned for v2.x)
