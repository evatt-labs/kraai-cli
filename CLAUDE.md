# kraai-cli — Kraai CLI Tool

Go CLI for the Kraai platform. Talks to `api.kraai.dev` (or local `api.lvh.me`).
Global rules and safety boundaries live in `~/.claude/CLAUDE.md`.

---

## Stack

| Layer       | Tech                          |
|-------------|-------------------------------|
| Language    | Go 1.25                       |
| CLI routing | Hand-rolled (stdlib `flag`)   |
| HTTP client | `net/http` (no framework)     |
| Auth        | Device Authorization Grant (RFC 8628) |
| Release     | GoReleaser v2                 |
| CI/CD       | Forgejo Actions               |

---

## Commands

```bash
make build          # Compile → ./kraai binary
make run ARGS="..."  # Build + run with --local flag (hits api.lvh.me)
make clean          # Remove ./kraai binary

go test ./...       # Run tests (no Makefile target)
go vet ./...        # Vet (required before commit)
```

**Install (end users):**
```bash
curl -fsSL https://get.kraai.dev | sh
```

---

## Directory Structure

```
kraai-cli/
├── cmd/                    # All command implementations (~20 files, one per feature)
│   ├── main.go             # Entry point — hand-rolled switch router on os.Args
│   ├── login.go / logout.go / whoami.go
│   ├── workspaces.go / projects.go / plans.go
│   ├── deploy.go / publish.go / deployments.go
│   ├── tools.go / status.go / logs.go / usage.go
│   ├── tokens.go / auth-connections.go
│   ├── workflows.go / policies.go / approvals.go
│   └── validate.go
├── internal/
│   ├── client/client.go    # HTTP API client + all API method signatures (~40 endpoints)
│   └── config/credentials.go  # ~/.kraai/credentials read/write
├── worker/                 # Cloudflare Worker (deploy-related, not the Go CLI itself)
│   ├── index.js
│   └── wrangler.toml
└── .forgejo/workflows/
    ├── release.yml         # Triggers on v* tags → goreleaser
    └── deploy-worker.yml   # Deploys the CF worker
```

---

## CLI Command Reference

| Command            | Subcommands                                    | Purpose                                      |
|--------------------|------------------------------------------------|----------------------------------------------|
| `login`            | —                                              | Device auth flow → `~/.kraai/credentials`    |
| `logout`           | —                                              | Revoke token server-side + delete creds      |
| `whoami`           | —                                              | Print user email, workspace name/ID          |
| `workspaces`       | list, new, use, rename, delete                 | Manage workspaces; `use` switches active     |
| `projects`         | list, new, rename, delete                      | Manage projects within active workspace      |
| `plans`            | list, set, resume                              | Billing plan management                      |
| `validate`         | —                                              | Local OpenAPI 3.x spec validation            |
| `deploy`           | activate                                       | Upload spec + publish; `activate` switches version |
| `publish`          | —                                              | Publish latest ready source (`--slug` required) |
| `deployments`      | —                                              | List deployment versions + status            |
| `tools`            | —                                              | List MCP tools from deployed server          |
| `tokens`           | create, revoke                                 | API token management                         |
| `status`           | —                                              | Deployment health + info                     |
| `usage`            | —                                              | Request counts, quota, plan entitlements     |
| `logs`             | —                                              | MCP request logs; `--follow` for live tail   |
| `auth-connections` | list, create, delete                           | Upstream API credentials                     |
| `workflows`        | list, create, delete, trigger, runs, status, cancel | Workflow definitions + runs            |
| `policies`         | list, create, delete                           | OPA policies (Rego)                          |
| `approvals`        | list, pending, approve, deny                   | Approval request management                  |

---

## API Client

**Location:** `internal/client/client.go`

| Setting       | Value                                                |
|---------------|------------------------------------------------------|
| Default base  | `https://api.kraai.dev`                             |
| Local dev     | `http://api.lvh.me` (`--local` flag or `KRAAI_ENV=local`) |
| Override      | `KRAAI_API_BASE_URL` env var                         |
| Timeout       | 30s                                                  |
| Auth          | `Authorization: Bearer <token>`                      |

---

## Configuration & Credentials

**Credentials file:** `~/.kraai/credentials` (JSON, perms 0600)

```json
{
  "token": "...",
  "token_id": "...",
  "workspace_id": "...",
  "workspace_name": "...",
  "email": "...",
  "created_at": "..."
}
```

**Environment variable overrides:**

| Var                      | Purpose                                     |
|--------------------------|---------------------------------------------|
| `KRAAI_TOKEN`            | Use token directly (bypasses credentials file) |
| `KRAAI_WORKSPACE_ID`     | Override active workspace                   |
| `KRAAI_API_BASE_URL`     | Override API endpoint                       |
| `KRAAI_RUNTIME_BASE_URL` | Override MCP runtime base URL               |
| `KRAAI_ENV=local`        | Shorthand for local dev stack               |

---

## Auth Flow (Device Authorization Grant — RFC 8628)

1. POST `/v1/auth/device/code` → get device code + user code + verification URI
2. Print URI/user code; attempt browser open (platform-aware)
3. Poll `/v1/auth/device/token` every 5s until authorized or expired
4. On success → write `~/.kraai/credentials`

`logout` revokes the token server-side then deletes the credentials file.

---

## Release & Distribution

GoReleaser v2 builds cross-platform binaries on `v*` tag push:

- **Platforms:** Linux, macOS, Windows (amd64 + arm64; Windows arm64 excluded)
- **Artifacts:** `.tar.gz` / `.zip` archives, deb, rpm, apk, Arch Linux packages
- **Checksums:** SHA256
- **Release token:** `GH_RELEASE_TOKEN` (Forgejo secret)

---

## Rules

- No heavy CLI frameworks — stick to stdlib `flag` + the existing hand-rolled router
- No new external dependencies without explicit discussion (`golang.org/x/term` is the only non-stdlib dep)
- Never echo or log token values — they flow through memory only, written to disk at 0600
- `go vet ./...` must pass before every commit
- Each command is self-contained in its own file in `cmd/`
