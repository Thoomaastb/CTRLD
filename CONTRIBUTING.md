# Contributing to CTRLD

Thanks for your interest in contributing! This guide explains how to get started.

---

## Code of Conduct

Be kind. Be constructive. We're building something together.

---

## Ways to contribute

- **Report bugs** — [open a bug report](https://github.com/Thoomaastb/CTRLD/issues/new?template=bug-report.yml)
- **Request features** — [open a feature request](https://github.com/Thoomaastb/CTRLD/issues/new?template=feature-request.yml)
- **Discuss ideas** — [join a discussion](https://github.com/Thoomaastb/CTRLD/discussions)
- **Translate** — help us bring CTRLD to more languages (core: DE + EN, rest is community-driven)
- **Write code** — see the process below

---

## CLA

All contributors must sign the **CTRLD Contributor License Agreement** before a PR can be merged.

The CLA bot will guide you through this automatically when you open your first PR. You only need to sign once.

[Read the full CLA](docs/cla/CLA.md)

---

## Development setup

### Requirements

- Go 1.22+
- Node.js 20+ (see `web/.nvmrc`)
- Git

### Clone and run

```bash
git clone https://github.com/Thoomaastb/CTRLD.git
cd CTRLD

# Backend
go run ./cmd/ctrld

# Frontend (separate terminal)
cd web
npm install
npm run dev
```

---

## Commit conventions

CTRLD uses [Conventional Commits](https://www.conventionalcommits.org/). This is enforced in CI — PRs with non-conforming commits will fail the lint check.

### Format

```
type(scope): short description

[optional body]

[optional footer: BREAKING CHANGE: ...]
```

### Types

| Type | When to use | Release impact |
|---|---|---|
| `feat` | New feature | Minor |
| `fix` | Bug fix | Patch |
| `perf` | Performance improvement | Patch |
| `docs` | Documentation only | None |
| `refactor` | Code change, no feature/fix | None |
| `test` | Adding or fixing tests | None |
| `chore` | Tooling, dependencies | None |
| `ci` | CI/CD changes | None |
| `revert` | Revert a previous commit | Patch |

A `!` after the type (e.g. `feat!:`) or a `BREAKING CHANGE:` footer triggers a **Major** release.

### Scopes

```
auth · pim · audit · monitoring · logs · services · dashboard
hub · spoke · api · db · ui · installer · docker · ci · deps
release · readme · license · security
```

### Examples

```
feat(auth): add passkey registration via WebAuthn
fix(pim): resolve countdown timer drift after session extension
docs(readme): update installation instructions
chore(deps): bump golangci-lint to v1.62.0
feat(hub)!: change spoke registration API — breaks existing spoke clients
```

---

## Branch strategy

| Branch | Purpose | Protected |
|---|---|---|
| `main` | Stable — every commit is a potential release | Yes, PR required |
| `develop` | Active development integration branch | Yes, PR required |
| `feature/*` | Feature branches | No |
| `fix/*` | Bug fix branches | No |
| `chore/*` | Tooling, CI, docs | No |

All PRs go to `develop` first. `develop` is merged to `main` for releases.

### Naming examples

```
feature/auth-passkey-support
fix/pim-timer-drift
chore/update-golangci-lint
docs/add-api-reference
```

---

## Pull request process

1. Fork the repo and create a branch from `develop`
2. Make your changes — keep commits atomic and well-described
3. Ensure tests pass: `go test ./...` and `npm run test`
4. Ensure linters pass: `golangci-lint run` and `npm run lint`
5. Open a PR against `develop`
6. Sign the CLA if prompted
7. Wait for review — we aim to respond within a few days

---

## Security vulnerabilities

Please **do not** open a public issue for security vulnerabilities.

See [SECURITY.md](SECURITY.md) for our responsible disclosure process.
