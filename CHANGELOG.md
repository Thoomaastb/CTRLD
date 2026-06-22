# Changelog

All notable changes to CTRLD are documented here.
Format: [Semantic Versioning](https://semver.org)

## [0.14.0](https://github.com/Thoomaastb/CTRLD/compare/v0.13.0...v0.14.0) (2026-06-22)

### Features

* **logs:** add journald log viewer with live-tail, filtering and export ([1434e31](https://github.com/Thoomaastb/CTRLD/commit/1434e314e767f06ec5f2e8674a3007d64101c55b))

## [0.13.0](https://github.com/Thoomaastb/CTRLD/compare/v0.12.0...v0.13.0) (2026-06-22)

### Features

* **monitoring:** add alert engine with anti-flapping, webhook support and toast notifications ([f51ce30](https://github.com/Thoomaastb/CTRLD/commit/f51ce30d0d9fa9e2715bfe6d4f943a67f4148628))

## [0.12.0](https://github.com/Thoomaastb/CTRLD/compare/v0.11.1...v0.12.0) (2026-06-22)

### Features

* **ui:** add live dashboard with WebSocket metrics, sparklines and monitoring page ([ae3647d](https://github.com/Thoomaastb/CTRLD/commit/ae3647dda1832116ae0625886a41df89bc228246))

### Documentation

* **readme:** update roadmap to feature blocks without pre-1.0.0 versions ([031cc0d](https://github.com/Thoomaastb/CTRLD/commit/031cc0d85edda83d485179e4382efaece9bb1ed6))

## [0.11.1](https://github.com/Thoomaastb/CTRLD/compare/v0.11.0...v0.11.1) (2026-06-22)

### Bug Fixes

* **monitoring:** fix NumCores field, add Windows stub and system inventory with Docker support ([1a9722c](https://github.com/Thoomaastb/CTRLD/commit/1a9722cdc1afa32f243578002c97f30667c47be6))

## [0.11.0](https://github.com/Thoomaastb/CTRLD/compare/v0.10.1...v0.11.0) (2026-06-19)

### Features

* **monitoring:** add /proc metrics collector, rolling buffer and WebSocket live stream ([8e4e786](https://github.com/Thoomaastb/CTRLD/commit/8e4e78680fde8e457f322669babaab7815a6118a))

## [0.10.1](https://github.com/Thoomaastb/CTRLD/compare/v0.10.0...v0.10.1) (2026-06-19)

### Bug Fixes

* **server:** move test config to internal helper to resolve module lookup issue ([4b76f16](https://github.com/Thoomaastb/CTRLD/commit/4b76f16432cbcb0db66bc49a7d1882aa0a474320))

## [0.10.0](https://github.com/Thoomaastb/CTRLD/compare/v0.9.0...v0.10.0) (2026-06-19)

### Features

* **ui:** add router wiring, login page, MFA flow and setup wizard ([d233ed9](https://github.com/Thoomaastb/CTRLD/commit/d233ed90acf2928e11515550ce416d6eff7c5782))

## [0.9.0](https://github.com/Thoomaastb/CTRLD/compare/v0.8.0...v0.9.0) (2026-06-19)

### Features

* **ui:** add router wiring, login page, MFA flow and setup wizard ([eecabbf](https://github.com/Thoomaastb/CTRLD/commit/eecabbfbd2e5c181c6cd1db4c43208a8c7ef8ab2))

## [0.8.0](https://github.com/Thoomaastb/CTRLD/compare/v0.7.0...v0.8.0) (2026-06-19)

### Features

* **setup:** add setup wizard, user management and last-admin protection ([ffa8b43](https://github.com/Thoomaastb/CTRLD/commit/ffa8b43b205a5e918d13d031fce7f99159b18978))

## [0.7.0](https://github.com/Thoomaastb/CTRLD/compare/v0.6.0...v0.7.0) (2026-06-19)

### Features

* **pim:** add PIM engine, audit log service and break-glass support ([15ecba9](https://github.com/Thoomaastb/CTRLD/commit/15ecba9377fe95649d8da5d366e44f813f6c450a))

## [0.6.0](https://github.com/Thoomaastb/CTRLD/compare/v0.5.0...v0.6.0) (2026-06-19)

### Features

* **auth:** add TOTP MFA with QR code setup, verification and backup codes ([dd68aed](https://github.com/Thoomaastb/CTRLD/commit/dd68aed173af2300ce6e8c7b12e96548708f68f4))

## [0.5.0](https://github.com/Thoomaastb/CTRLD/compare/v0.4.0...v0.5.0) (2026-06-19)

### Features

* **auth:** add login endpoint, session management and refresh token rotation ([a96006e](https://github.com/Thoomaastb/CTRLD/commit/a96006e166152d6ad1245628dfb005ab4fc8f70a))

## [0.4.0](https://github.com/Thoomaastb/CTRLD/compare/v0.3.1...v0.4.0) (2026-06-19)

### Features

* **auth:** add Argon2id password hashing, JWT tokens, rate limiting and auth middleware ([5ab1564](https://github.com/Thoomaastb/CTRLD/commit/5ab15644bdc42dd592b7d296e50252fcf3812f0d))

## [0.3.1](https://github.com/Thoomaastb/CTRLD/compare/v0.3.0...v0.3.1) (2026-06-19)

### Bug Fixes

* **docker:** update to Node 24, Go 1.26, enable CGO for sqlite3 ([e4c90f1](https://github.com/Thoomaastb/CTRLD/commit/e4c90f16493372988e0ec5ea9ea05576a63e45d2))

## [0.3.0](https://github.com/Thoomaastb/CTRLD/compare/v0.2.0...v0.3.0) (2026-06-19)

### Features

* **api:** add Go project structure with health endpoint and config loading ([b0eb1ab](https://github.com/Thoomaastb/CTRLD/commit/b0eb1abfd90169c6ffaf25dd62313d30a03d0ceb))

## [0.2.0](https://github.com/Thoomaastb/CTRLD/compare/v0.1.2...v0.2.0) (2026-06-19)

### Features

* **db:** add SQLite schema, goose migrations and sqlc query definitions ([9e54230](https://github.com/Thoomaastb/CTRLD/commit/9e54230a6593816853d5492c5056ffc1b0e86034))

## [0.1.2](https://github.com/Thoomaastb/CTRLD/compare/v0.1.1...v0.1.2) (2026-06-19)

### Bug Fixes

* **ui:** add --dir src flag to next lint to prevent path misparse ([dfbd0c0](https://github.com/Thoomaastb/CTRLD/commit/dfbd0c074b3daca0ecf8437c03833f7ccf8f304f))

## [0.1.1](https://github.com/Thoomaastb/CTRLD/compare/v0.1.0...v0.1.1) (2026-06-19)

### Bug Fixes

* **ci:** set working-directory for Node jobs and add test placeholder ([a29ea55](https://github.com/Thoomaastb/CTRLD/commit/a29ea55524dc38ec8f224705f717a57efd9016bb))

## [0.1.0](https://github.com/Thoomaastb/CTRLD/compare/v0.0.0...v0.1.0) (2026-06-19)

### Features

* **ui:** add Next.js frontend structure with design system and API client ([a989d6c](https://github.com/Thoomaastb/CTRLD/commit/a989d6cd490a5e0d5b15ceed92d1a0b19464d196))

## 1.0.0 (2026-06-18)

### Bug Fixes

* **ci:** correct golangci-lint config schema and go version for vuln fixes ([d266f64](https://github.com/Thoomaastb/CTRLD/commit/d266f6485e18282c98e4a558dfd48c5335720e49))

### Documentation

* **readme:** restructure screenshots as mockup tables, add pre-alpha disclaimer ([6620b7e](https://github.com/Thoomaastb/CTRLD/commit/6620b7ed1ca7ad96092a1b8374a48dd4c57be3f4))

<!-- CHANGELOG content is automatically generated by semantic-release. Do not edit manually. -->
