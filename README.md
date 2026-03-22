<p align="center">
  <img src="web/static/logo.webp" alt="Clock Keeper" width="128" />
</p>

<h1 align="center">Clock Keeper</h1>

<p align="center">
  A digital companion for in-person <a href="https://bloodontheclocktower.com">Blood on the Clocktower</a> games.<br/>
  Not a digital clone — a tool that makes the physical experience smoother for the Storyteller.
</p>

---

## Features

- **Game Setup** — Select or create a script, set player count, randomize roles, follow the official setup checklist
- **Night Management** — Step-by-step wake order, ability prompts, reminder token tracking, night action recording
- **Note-Taking & Tracking** — Seating chart, alive/dead status, nominations, votes, executions, game state timeline

## Quick Start

```bash
# Download game data (characters, night order, jinxes, icons)
./scripts/download-botc-data.fish

# Start the app
docker-compose up
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go, ConnectRPC, Ent ORM |
| Frontend | Svelte 5, SvelteKit, Tailwind 4 |
| Database | PostgreSQL |
| Testing | Playwright (E2E), Vitest, testify |

See [docs/architecture.md](docs/architecture.md) for full details.

## Documentation

- [Project Overview](docs/project-overview.md) — Vision, scope, MVP definition
- [Architecture](docs/architecture.md) — Tech stack, system design, testing

## Disclaimer

This is an unofficial, non-commercial fan project. It is not affiliated with or endorsed by The Pandemonium Institute. Game data and character icons are sourced from the official [botc-release](https://github.com/ThePandemoniumInstitute/botc-release) repository under the [Community Created Content Policy](https://bloodontheclocktower.com/pages/community-created-content-policy).
