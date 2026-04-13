# kraai-cli — Kraai CLI Tool

Go CLI for the Kraai platform. Talks to `api.kraai.dev` (or `api.lvh.me` with `--local`).

API contract changes originate in `kraai-backend` — update `internal/client/client.go` to match.

---

## Stack

| Layer | Tech |
|---|---|
| Language | Go 1.25 |
| CLI routing | Hand-rolled (stdlib `flag`) |
| HTTP | `net/http` (no framework) |
| Auth | Device Authorization Grant (RFC 8628) |
| Release | GoReleaser v2 |
| CI/CD | Forgejo Actions |

**Sister repos:** `kraai-backend` (API source of truth), `kraai-frontend` (parallel UI surface), `kraai-infra` (Compose → `api.lvh.me`), `kraai-marketing` (unrelated to CLI).

---

## Commands

```bash
make build              # → ./kraai
make run ARGS="..."     # build + run with --local
make clean
go test ./...
go vet ./...            # required before commit
```

**End-user install:** `curl -fsSL https://get.kraai.dev | sh`

---

## Directory Structure

```
kraai-cli/
├── cmd/                    # One file per command (~20 files)
│   ├── main.go             # Entry — hand-rolled switch router on os.Args
│   ├── login.go / logout.go / whoami.go
│   ├── workspaces.go / projects.go / plans.go
│   ├── deploy.go / publish.go / deployments.go
│   ├── tools.go / status.go / logs.go / usage.go
│   ├── tokens.go / auth-connections.go
│   ├── workflows.go / policies.go / approvals.go
│   └── validate.go
├── internal/
│   ├── client/client.go    # HTTP API client (~40 endpoints)
│   └── config/credentials.go  # ~/.kraai/credentials r/w
├── worker/                 # Cloudflare Worker (deploy-adjacent, not the CLI)
└── .forgejo/workflows/
    ├── release.yml         # v* tags → goreleaser
    └── deploy-worker.yml
```

---

## CLI Command Reference

| Command | Subcommands | Purpose |
|---|---|---|
| `login` | — | Device auth flow → `~/.kraai/credentials` |
| `logout` | — | Revoke token server-side + delete creds |
| `whoami` | — | Print user email + workspace |
| `workspaces` | list, new, use, rename, delete | `use` switches active |
| `projects` | list, new, rename, delete | Within active workspace |
| `plans` | list, set, resume | Billing |
| `validate` | — | Local OpenAPI 3.x validation |
| `deploy` | activate | Upload spec + publish; `activate` switches version |
| `publish` | — | Publish latest ready source (`--slug` required) |
| `deployments` | — | List versions + status |
| `tools` | — | List MCP tools from deployed server |
| `tokens` | create, revoke | API token mgmt |
| `status` | — | Deployment health |
| `usage` | — | Requests, quota, entitlements |
| `logs` | — | MCP request logs (`--follow` for tail) |
| `auth-connections` | list, create, delete | Upstream API creds |
| `workflows` | list, create, delete, trigger, runs, status, cancel | |
| `policies` | list, create, delete | OPA (Rego) |
| `approvals` | list, pending, approve, deny | |

---

## API Client

**Location:** `internal/client/client.go`

| Setting | Value |
|---|---|
| Default base | `https://api.kraai.dev` |
| Local dev | `http://api.lvh.me` (`--local` or `KRAAI_ENV=local`) |
| Override | `KRAAI_API_BASE_URL` |
| Timeout | 30s |
| Auth | `Authorization: Bearer <token>` |

---

## Credentials & Environment

**Credentials file:** `~/.kraai/credentials` (JSON, perms 0600)

```json
{"token":"...","token_id":"...","workspace_id":"...","workspace_name":"...","email":"...","created_at":"..."}
```

**Env overrides:**

| Var | Purpose |
|---|---|
| `KRAAI_TOKEN` | Bypass credentials file |
| `KRAAI_WORKSPACE_ID` | Override active workspace |
| `KRAAI_API_BASE_URL` | Override API endpoint |
| `KRAAI_RUNTIME_BASE_URL` | Override MCP runtime |
| `KRAAI_ENV=local` | Shorthand for local stack |

---

## Auth Flow (Device Grant, RFC 8628)

1. POST `/v1/auth/device/code` → device code + user code + verification URI
2. Print URI/user code; attempt platform-aware browser open
3. Poll `/v1/auth/device/token` every 5s until authorized/expired
4. On success → write `~/.kraai/credentials`

`logout` revokes server-side then deletes creds.

---

## Release (GoReleaser v2)

Triggered on `v*` tag push. Platforms: Linux, macOS, Windows (amd64 + arm64; no Windows arm64). Artifacts: tar.gz/zip, deb, rpm, apk, Arch pkgs. SHA256 checksums. Release token: `GH_RELEASE_TOKEN` (Forgejo secret).

---

## Rules

- No heavy CLI frameworks — stdlib `flag` + existing hand-rolled router
- No new external deps without discussion (only non-stdlib: `golang.org/x/term`)
- Never echo/log token values — memory-only, disk at 0600
- Each command self-contained in its own file in `cmd/`
