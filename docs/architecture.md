# Clock Keeper — Architecture

## Tech Stack

| Layer                | Technology                                            |
|----------------------|-------------------------------------------------------|
| Backend              | Go 1.26.1                                             |
| Frontend             | Svelte 5 + SvelteKit + Tailwind 4                     |
| API                  | ConnectRPC + Protocol Buffers                         |
| ORM                  | Ent + Atlas migrations                                |
| Database             | PostgreSQL 18                                         |
| Build                | Docker multi-stage (frontend embedded into Go binary) |
| Task runner          | Taskfile                                              |
| Frontend pkg manager | pnpm                                                  |
| Code gen             | buf (proto → Go + TypeScript)                         |
| Backend testing      | Go `testing` + testify + testcontainers + enttest     |
| Frontend testing     | Vitest                                                |
| E2E testing          | Playwright                                            |

## System Design

### Single binary deployment

The Go binary serves both the ConnectRPC API and the static Svelte frontend. The frontend is built with SvelteKit's static adapter and embedded into the binary at compile time via `//go:embed`.

### API services (ConnectRPC)

Service boundaries (preliminary — will evolve during implementation):

- **ScriptService** — import/select scripts, list characters
- **GameService** — create game, assign roles, manage game lifecycle
- **NightService** — night order, wake sequence, ability prompts
- **NoteService** — per-phase notes, nominations, votes, deaths

### Database (Ent schemas)

Preliminary entities:

- Script, Character, Jinx
- Game, Player, RoleAssignment
- Phase, Note, Nomination, Vote

### Frontend

SvelteKit SPA with static adapter. ConnectRPC TypeScript client generated from the same proto files as the server. PWA with service worker for offline core features.

### Deployment

```
docker-compose.yml
├── clockkeeper   (Go binary: API + embedded frontend)
└── postgres      (PostgreSQL 18)
```

Single `docker-compose up` to self-host.

## Key Decisions

| Decision                      | Rationale                                                                  |
|-------------------------------|----------------------------------------------------------------------------|
| Mirror plutus architecture    | Proven pattern in existing projects, no new tooling to learn               |
| Embed frontend in Go binary   | Single artifact deployment, simple ops                                     |
| ConnectRPC over REST          | Type-safe API with generated Go + TypeScript clients from one proto source |
| PostgreSQL (not SQLite)       | Server-based app needs proper DB; enables future multi-device sync         |
| Offline PWA for core features | Storyteller can run night phases even with spotty connectivity             |

## Testing

| Layer               | Tool                                  | Covers                                                      |
|---------------------|---------------------------------------|-------------------------------------------------------------|
| Backend unit        | Go `testing` + testify                | Service logic, helpers, utilities                           |
| Backend integration | testcontainers (PostgreSQL) + enttest | DB queries, migrations, API handlers against real DB        |
| Frontend unit       | Vitest                                | Component logic, stores, utilities                          |
| E2E (full stack)    | Playwright                            | Real user flows against running app (server + DB + browser) |

### E2E approach

Playwright tests run against a full docker-compose stack (Go server + PostgreSQL). Tests exercise real user flows end to end: create game → run night phase → take notes.

- `docker-compose.test.yml` spins up the test environment
- `task test:e2e` runs the Playwright suite
- `task test:unit` runs Go + Vitest unit tests
- `task test` runs everything
