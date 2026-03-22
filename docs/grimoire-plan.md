# Grimoire Feature — Implementation Plan

## Context

The app currently tracks roles anonymously — no player names, no spatial layout, no token placement. The Storyteller has no digital equivalent of the physical grimoire (the secret board showing who has which role, reminder tokens, alive/dead state). This feature adds a free-form canvas grimoire that becomes the source of truth for player-to-role assignments, persisted to the backend.

The grimoire tab already exists as a placeholder in the setup state. The in-progress state currently has no tabs (single scrollable page) — we'll add a tab bar there too.

---

## Phase 1: Database — Ent Schemas + Migration

### 1a. Create `ent/schema/grimoire_player.go`

New entity:
- `game_id` (int, FK to Game, required)
- `name` (string, not empty — physical player's name)
- `role_id` (string, default empty — character ID, empty = unassigned)
- `x` (float64, default 0)
- `y` (float64, default 0)
- `is_dead` (bool, default false)
- `ghost_vote_used` (bool, default false)
- `seat_order` (int, default 0 — conceptual circle position)
- Edges: `From("game", Game)`, `To("reminder_tokens", GrimoireToken)`
- Mixin: `TimestampMixin{}`

### 1b. Create `ent/schema/grimoire_token.go`

New entity for placed reminder tokens:
- `game_id` (int, FK to Game, required)
- `player_id` (int, FK to GrimoirePlayer, optional — nil = floating on canvas)
- `character_id` (string, not empty — which character's reminder)
- `text` (string, not empty — reminder text)
- `x` (float64, default 0)
- `y` (float64, default 0)
- Edges: `From("game", Game)`, `From("player", GrimoirePlayer)`
- Mixin: `TimestampMixin{}`

### 1c. Modify `ent/schema/game.go`

Add edges:
```go
edge.To("grimoire_players", GrimoirePlayer.Type),
edge.To("grimoire_tokens", GrimoireToken.Type),
```

### 1d. Generate + migrate
```
task gen:ent
task db:migrate:new -- add_grimoire
```

---

## Phase 2: Proto Definitions

### Modify `proto/clockkeeper/v1/clockkeeper.proto`

**New messages:**
- `GrimoirePlayer` — id, game_id, name, role_id, x, y, is_dead, ghost_vote_used, seat_order, character (resolved)
- `GrimoireToken` — id, game_id, player_id (optional), character_id, text, x, y
- `Grimoire` — repeated players, repeated tokens

**New RPCs on ClockKeeperService:**

| RPC | Purpose |
|-----|---------|
| `GetGrimoire` | Fetch full grimoire state for a game |
| `AddGrimoirePlayer` | Add a named player seat |
| `UpdateGrimoirePlayer` | Change name or role assignment |
| `RemoveGrimoirePlayer` | Delete player + cascade tokens |
| `MoveGrimoireItems` | Batch update x/y for players and/or tokens (single drag-end call) |
| `AddGrimoireToken` | Place a reminder token on the canvas |
| `RemoveGrimoireToken` | Remove a reminder token |
| `ToggleGrimoirePlayerDeath` | Toggle alive/dead, syncs with Death system |

**Also:** Add `Grimoire grimoire` field to existing `Game` message so grimoire loads with the game.

### Generate
```
task gen:proto
```

---

## Phase 3: Backend Handlers

### 3a. Modify `internal/web/convert.go`

Add conversion functions:
- `entGrimoirePlayerToProto` — resolves role_id → Character via registry
- `entGrimoireTokenToProto`
- `buildGrimoireProto`

Update `entGameToProto` to include grimoire data.

### 3b. Modify `internal/web/api_games.go`

- Update `getOwnedGame` to eagerly load `grimoire_players` (with `reminder_tokens`) and `grimoire_tokens`
- Update `DeleteGame` to cascade-delete grimoire entities

### 3c. Create `internal/web/api_grimoire.go`

All grimoire RPC handlers. Key design:
- All handlers use `getOwnedGame()` for ownership verification
- `MoveGrimoireItems` accepts both player and token positions in one call (efficient for drag operations)
- `ToggleGrimoirePlayerDeath` integrates with existing death system (see below)

### 3d. Death sync integration

When grimoire toggles a player dead (player has role_id, game is in_progress):
- **Kill**: Create Death record in active phase for the player's `role_id` with propagation
- **Revive**: Remove Death record for the role in active phase with propagation
- **Ghost vote**: Update Death records via existing `UseGhostVote` logic

When existing `RecordDeath`/`RemoveDeath` RPCs are called (from DeathTracker UI):
- Also update corresponding `GrimoirePlayer.is_dead` (find by `role_id` match)

Extract shared helper: `syncGrimoireDeathState(ctx, game, roleID, isDead)`

### 3e. Create `internal/web/api_grimoire_test.go`

Tests following existing patterns:
- CRUD for players and tokens
- Batch position updates
- Death toggle syncs with Death records
- Cascade delete (remove player → removes tokens)
- Ownership checks

---

## Phase 4: Frontend — Grimoire Components

### 4a. `web/src/lib/components/GrimoireCanvas.svelte` (main component)

**Canvas approach: CSS transforms on HTML div** (no external library needed):
- Outer wrapper: `overflow: hidden`, captures pointer events for pan
- Inner "world" div: `transform: translate(panX, panY) scale(zoom)`
- Player tokens + reminder tokens: absolutely positioned in world space
- Pan: pointer drag on empty canvas area
- Zoom: wheel event + pinch via `svelte-gestures` (already a dependency)
- Zoom clamp: 0.3–3.0

**State:**
```typescript
let players = $state<GrimoirePlayer[]>([]);
let tokens = $state<GrimoireToken[]>([]);
let panX = $state(0), panY = $state(0), zoom = $state(1);
let dragging = $state<{type: 'player'|'token', id: bigint} | null>(null);
```

**Features:**
- "Initialize Players" button: creates N players in a circle layout (using game.player_count)
- If roles already assigned in setup, pre-fills role_ids
- Add player button for adding seats mid-game
- Debounced position saves: send `MoveGrimoireItems` on pointer-up after drag

### 4b. `web/src/lib/components/GrimoirePlayerToken.svelte`

Circular token displaying:
- Character icon (from existing `/characters/{edition}/{id}.webp` pattern) or empty seat icon
- Player name below token
- Dead state: grayscale filter + red shroud overlay
- Ghost vote indicator (skull icon, reuse from DeathTracker)
- Tap → popover/modal for: edit name, assign role (opens CharacterPickerModal), toggle death
- Draggable via pointer events

### 4c. `web/src/lib/components/GrimoireReminderToken.svelte`

Smaller circular token:
- Character icon (small) + reminder text
- Draggable
- Tap to delete (or long-press → confirm)

### 4d. `web/src/lib/components/GrimoireTokenTray.svelte`

Slide-up/collapsible panel at bottom of canvas:
- Lists all available reminder tokens from `game.reminderTokens`
- Tap a token → places it on canvas (center or near last-tapped player)
- Grouped by character

---

## Phase 5: Frontend — Page Integration

### Modify `web/src/routes/games/[id]/+page.svelte`

**Setup state** (tabs already exist): Replace grimoire placeholder (lines 779-788) with `<GrimoireCanvas>`.

**In-progress state** (currently no tabs): Add a tab bar with "Phase" and "Grimoire" tabs.

```typescript
type InProgressTab = 'phase' | 'grimoire';
let inProgressTab = $state<InProgressTab>('phase');
```

- "Phase" tab: existing night order + death tracker + travellers (unchanged)
- "Grimoire" tab: `<GrimoireCanvas>`

**Data loading:** Load grimoire with the game (it's included in the Game proto). Refresh after mutations.

### Optional: Modify `web/src/lib/components/NightOrder.svelte`

Accept optional `grimoirePlayers` prop. When provided, display "PlayerName (RoleName)" instead of just role name in the night order list. Low priority enhancement.

---

## Files Summary

### New files
| File | Purpose |
|------|---------|
| `ent/schema/grimoire_player.go` | Player seat entity |
| `ent/schema/grimoire_token.go` | Placed reminder token entity |
| `internal/web/api_grimoire.go` | Grimoire RPC handlers |
| `internal/web/api_grimoire_test.go` | Handler tests |
| `web/src/lib/components/GrimoireCanvas.svelte` | Pan/zoom canvas |
| `web/src/lib/components/GrimoirePlayerToken.svelte` | Draggable player token |
| `web/src/lib/components/GrimoireReminderToken.svelte` | Draggable reminder token |
| `web/src/lib/components/GrimoireTokenTray.svelte` | Available tokens panel |

### Modified files
| File | Change |
|------|--------|
| `ent/schema/game.go` | Add grimoire edges |
| `proto/clockkeeper/v1/clockkeeper.proto` | Add grimoire messages + RPCs |
| `internal/web/convert.go` | Grimoire conversion functions |
| `internal/web/api_games.go` | Eager loading + cascade delete |
| `internal/web/api_deaths.go` | Sync grimoire player state on death/revive |
| `web/src/routes/games/[id]/+page.svelte` | Replace placeholder, add in-progress tabs |

---

## Verification

1. `task gen` — all codegen passes
2. `task db:migrate` — migration applies cleanly
3. `task test` — existing + new tests pass
4. `task check` — TypeScript/Svelte type checks pass
5. Manual test flow:
   - Create game → Setup tab → Grimoire tab → Initialize players → Assign names + roles
   - Start game → Grimoire tab → Drag tokens around → Reload page → Positions persisted
   - Place reminder tokens from tray → Drag near players
   - Toggle player death → Verify DeathTracker shows matching death record
   - Toggle death from DeathTracker → Verify grimoire player shows dead state
   - Pan and zoom canvas on mobile/tablet
