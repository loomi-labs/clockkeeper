# Commands

All commands use [Task](https://taskfile.dev/) as the runner.

## Development

| Command | Description |
|---------|-------------|
| `task dev` | Start full dev environment (Postgres + backend with Air + frontend dev server) |
| `task run` | Run the Go server (no frontend, no DB) |
| `task build` | Build the Clock Keeper binary |

## Testing

| Command | Description |
|---------|-------------|
| `task test` | Run all tests (unit + E2E) |
| `task test:unit` | Run Go + Vitest unit tests |
| `task test:e2e` | Run Playwright E2E tests |
| `task check` | Run svelte-check for TypeScript/Svelte type errors |

## Code Generation

| Command | Description |
|---------|-------------|
| `task gen` | Run all code generation (Ent + Protobuf) |
| `task gen:ent` | Generate Ent ORM code (`go generate ./ent`) |
| `task gen:proto` | Generate Protobuf + ConnectRPC code (`buf generate`) |

## Database

| Command | Description |
|---------|-------------|
| `task db:up` | Start PostgreSQL and apply migrations |
| `task db:migrate` | Apply all pending migrations |
| `task db:migrate:new -- <name>` | Generate a new migration from Ent schema changes |
| `task db:migrate:status` | Show migration status |
| `task db:migrate:create:blank -- <name>` | Create a blank migration file |

## Frontend

| Command | Description |
|---------|-------------|
| `task frontend:dev` | Start frontend dev server only |
| `task frontend:build` | Build the Svelte frontend for production |
| `task format` | Format frontend code with Prettier |
| `task format:check` | Check frontend code formatting (no writes) |
