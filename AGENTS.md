# kraai-cli — AI assistant instructions

Go CLI for the Kraai platform. Talks to `api.kraai.dev` by default or `api.lvh.me` with `--local`.

API contract changes originate in the Kraai backend/monorepo. Keep `internal/client/client.go` aligned with the current API surface.

## Stack

- Go 1.25
- Hand-rolled CLI routing with stdlib `flag`
- `net/http`, no heavy HTTP/CLI framework
- Device Authorization Grant (RFC 8628)
- GoReleaser v2

## Commands

```bash
make build
make run ARGS="..."
make clean
go test ./...
go vet ./...
```

Run `go test ./...` and `go vet ./...` before committing code changes.

## Structure

- `cmd/`: one file per command; `cmd/main.go` routes on `os.Args`
- `internal/client/client.go`: HTTP API client
- `internal/config/credentials.go`: credentials file handling
- `worker/`: Cloudflare Worker deploy-adjacent code, not the CLI itself

## Rules

- Keep commands self-contained in `cmd/<command>.go`.
- Do not add a CLI framework. Use existing stdlib `flag` patterns.
- Do not add dependencies without explicit approval.
- Never echo, log, or print token values.
- Credentials live at `~/.kraai/credentials` with mode `0600`; do not read or print real credentials during assistant work.
- Respect env overrides: `KRAAI_TOKEN`, `KRAAI_WORKSPACE_ID`, `KRAAI_API_BASE_URL`, `KRAAI_RUNTIME_BASE_URL`, `KRAAI_ENV=local`.

## Auth Flow

1. POST `/v1/auth/device/code`.
2. Show verification URI and user code.
3. Poll `/v1/auth/device/token`.
4. Write credentials on success.

`logout` revokes server-side and removes local credentials.
