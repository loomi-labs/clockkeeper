# Clock Keeper — Project Overview

## Vision

A digital companion for in-person Blood on the Clocktower games. Not a digital clone — a tool that makes the physical experience smoother for the Storyteller.

## Goals

- Reduce Storyteller cognitive load during night phases (who to wake, in what order, what to say)
- Speed up game setup (script selection, role assignment by player count)
- Provide structured note-taking so the Storyteller can focus on running the game
- Server-client architecture: server handles AI features and data persistence
- Easy to self-host — single binary or Docker container
- Client remains a PWA that works offline for core features (setup, night, notes)

## Target Users

**MVP: Storytellers only.** The person running the game at a physical table, typically using a tablet or laptop.

Player-facing features (character reference, deduction aids) are a future consideration.

## Feature Areas

1. **Game Setup** — Select or import a script, configure player count, assign roles
2. **Night Management** — Night order checklist, character ability prompts, wake/sleep sequence
3. **Note-Taking & Tracking** — Per-phase notes, seating chart, nominations, votes, deaths
4. **Player Reference** *(future)* — Character abilities, jinx interactions, script info
5. **AI-Assisted Features** *(future)* — Setup recommendations, balancing advice, storyteller tips

## Out of Scope

- Online/remote play (this is for in-person games)
- Multi-device sync *(future — server supports it, but MVP is single-device)*
- Replacing the official app or clocktower.online
- Player-facing UI (MVP is Storyteller-only)
- Game rule enforcement or automation

## Data Sources

Game data comes from the official [ThePandemoniumInstitute/botc-release](https://github.com/ThePandemoniumInstitute/botc-release) repo, which provides assets explicitly for community toolmakers. Self-hosted copies.

- **`roles.json`** — All characters with abilities, team, reminders, night prompts, setup rules
- **`nightsheet.json`** — First night + other nights wake order
- **`jinxes.json`** — Character-pair interaction rules
- **Character icons** — WebP images (standard, exiled, dead variants) per edition
- **Script format** — Standard JSON schema, compatible with the official Script Tool

## Licensing

Governed by TPI's [Community Created Content Policy](https://bloodontheclocktower.com/pages/community-created-content-policy).

- Must be **free and non-commercial** — no selling, crowdfunding, or revenue from TPI IP
- Must be **clearly non-official** — include Community Created Content badge and non-affiliation disclaimer
- Must **not compete** with current or announced TPI products
- **No app store distribution** — self-hosted web app is permitted
- Contact for licensing questions: butler@thepandemoniuminstitute.com

## MVP Definition

A server-client app that lets a Storyteller:

1. Pick a script and generate a role assignment for N players
2. Follow a guided night phase with correct wake order and ability prompts
3. Take structured notes per game phase (day/night) with death and vote tracking

- **Server**: AI features, data persistence, runs as a single binary or Docker container
- **Client**: PWA on the Storyteller's device
- **Offline**: Core features (setup, night management, notes) work without server connection
- **Deployment**: Single `docker-compose up` or binary to self-host
- No account required for MVP
