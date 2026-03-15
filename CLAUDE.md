# Clock Keeper

Digital companion app for in-person Blood on the Clocktower games. Storyteller-focused MVP.

## Documentation

- @docs/project-overview.md — Vision, scope, MVP definition
- @docs/architecture.md — Tech stack, system design, testing strategy
- `@docs/user-stories.md` — User stories by feature area

## Tech Stack

- **Backend**: Go 1.25, ConnectRPC + Protocol Buffers, Ent ORM, PostgreSQL 18
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

| Task | Command |
|------|---------|
| Run dev server | `task dev` |
| Run all tests | `task test` |
| Run unit tests | `task test:unit` |
| Run E2E tests | `task test:e2e` |
| Generate proto | `task gen` |
| Generate Ent | `task gen:ent` |
| Build binary | `task build` |
| Docker up | `docker-compose up` |

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
