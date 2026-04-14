<p align="center">
  <img src="web/static/logo.webp" alt="Clock Keeper" width="128" />
</p>

<h1 align="center">Clock Keeper</h1>

<p align="center">
  A digital companion for in-person <a href="https://bloodontheclocktower.com">Blood on the Clocktower</a> games.<br/>
  Not a digital clone — a tool that makes the physical experience smoother for the Storyteller.
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-blue.svg" alt="License: AGPL-3.0" /></a>
  <a href="https://github.com/shifty11/clockkeeper/actions/workflows/main.yml"><img src="https://github.com/shifty11/clockkeeper/actions/workflows/main.yml/badge.svg" alt="CI" /></a>
</p>

---

## Features

- **Game Setup** — Select or create a script, set player count, randomize roles, follow the official setup checklist
- **Night Management** — Step-by-step wake order, ability prompts, reminder token tracking, night action recording
- **Note-Taking & Tracking** — Seating chart, alive/dead status, nominations, votes, executions, game state timeline

## Quick Start

**Prerequisites:** Docker and Docker Compose

```bash
# Download game data (characters, night order, jinxes, icons)
# Requires: curl, gh (GitHub CLI), fish shell
./scripts/download-botc-data.fish

# Copy and configure environment variables
cp .env.example .env

# Start the app
docker-compose up
```

The app will be available at `http://localhost:8080`.

## Development

```bash
# Install dependencies
pnpm install --dir web

# Start full dev environment (Postgres + backend + frontend)
task dev
```

See [docs/commands.md](docs/commands.md) for the full command reference.

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
- [Development Guidelines](docs/development-guidelines.md) — Coding standards
- [Commands](docs/commands.md) — Task runner reference

## License

This project is licensed under the [GNU Affero General Public License v3.0](LICENSE).

## Disclaimer

This is an unofficial, non-commercial fan project. It is not affiliated with or endorsed by The Pandemonium Institute. Game data and character icons are sourced from the official [botc-release](https://github.com/ThePandemoniumInstitute/botc-release) repository under the [Community Created Content Policy](https://bloodontheclocktower.com/pages/community-created-content-policy).
