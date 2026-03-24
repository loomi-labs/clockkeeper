# Clock Keeper

Digital companion app for in-person Blood on the Clocktower games. Storyteller-focused MVP.

## Documentation

- @docs/project-overview.md — Vision, scope, MVP definition
- @docs/architecture.md — Tech stack, system design, testing strategy
- @docs/development-guidelines.md — Coding guidelines
- @docs/commands.md — Full task command reference

## Tech Stack

- **Backend**: Go 1.26.1, ConnectRPC + Protocol Buffers, Ent ORM, PostgreSQL 18
- **Frontend**: Svelte 5 + SvelteKit, Tailwind 4, pnpm
- **Build**: Docker multi-stage, frontend embedded in Go binary via `//go:embed`
- **Code gen**: buf (proto → Go + TypeScript)
- **Task runner**: Taskfile

## Project Structure

```
cmd/              # Go entrypoint
internal/         # Backend services
  web/            # HTTP/ConnectRPC server
ent/              # Ent schemas + generated code
proto/            # Protocol Buffer definitions
gen/              # Generated code (protobuf + connectrpc)
web/              # Svelte frontend
data/             # BotC game data and character icons
scripts/          # Utility scripts
docs/             # Project documentation
```

## Commands

See @docs/commands.md for the full reference. Most common:

| Task | Command |
|------|---------|
| Run dev server | `task dev` |
| Run all tests | `task test` |
| Type-check frontend | `task check` |
| Generate all code | `task gen` |
| Build binary | `task build` |
| Format frontend | `task format` |
| Apply DB migrations | `task db:migrate` |

## Scripts

- **`scripts/download-botc-data.fish`** — Downloads game data (roles, night order, jinxes, script schema) and character icons from the official [botc-release](https://github.com/ThePandemoniumInstitute/botc-release) repo into `data/`. Requires `curl` and `gh`. Idempotent — safe to re-run.

## Testing

- **Backend unit**: Go `testing` + testify
- **Backend integration**: testcontainers (PostgreSQL) + enttest
- **Frontend unit**: Vitest
- **E2E**: Playwright against full docker-compose stack

## Guidelines

- This is a companion for physical play, not a digital clone of the game
- MVP is Storyteller-only — no player-facing UI yet
- Core features (setup, night, notes) must work offline via PWA
- Role assignment to physical players happens offline — the app tracks which roles are in play, not who has them
