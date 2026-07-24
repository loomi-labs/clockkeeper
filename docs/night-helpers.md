# Night Helpers - Trouble Brewing Audit

## What night helpers are

A night helper is a small, per-character interactive widget that renders inside a
character's row on the night sheet (the night-order checklist). Where the plain
night sheet only shows the character's ability text and a done/kill swipe, a
helper turns the Storyteller's mental arithmetic into a computed, tap-friendly
control:

- The **Empath** helper counts evil neighbours for you.
- The **Chef** helper counts pairs of adjacent evils.
- The **Undertaker** helper names the player executed on the previous day.
- The **Fortune Teller** helper lets you pick two players and tells you whether a
  Demon (or Red Herring) is among them.
- The **Washerwoman / Librarian / Investigator** helper lets you pick the
  "shown" player and a decoy, then attaches the reminder tokens and shows the
  matching info card.

Helpers are advisory. They read game state (seating, deaths, alignments,
poisoned/drunk status, bag substitutions) and never mutate real roles on their
own - the Storyteller stays in control. A helper degrades gracefully: if the
page does not wire the optional context it needs, it renders nothing.

## How the registry works

Adding a helper is a two-step change, so the dispatcher never needs to know
about individual helper components:

1. **Build a component** at `web/src/lib/components/night-helpers/<Name>.svelte`.
   It receives `{ entryId, ctx }`, where `entryId` is the character id of the
   row and `ctx` is the shared `NightHelperContext` (seating order, players,
   statuses, picks, and the various callbacks).
2. **Register it** in `web/src/lib/night-helpers/registry.ts` by adding one entry
   to `NIGHT_HELPERS`: `characterId -> { nights, component }`. `nights` gates the
   helper to the first night, other nights, or both.

The dispatcher (`NightEntryHelper.svelte`) looks up the entry by character id and
`night`, and renders the registered component. The page assembles a single
`NightHelperContext` once per night (`+page.svelte`, `nightHelperContext`) and
passes it down through `NightOrder.svelte`.

Two nuances worth knowing:

- **Type-only import cycle avoidance.** Helper components import only the _type_
  `NightHelperContext` from the registry. Type-only imports are erased at
  compile time, so `registry -> component` (value) and `component -> registry`
  (type) do not form a runtime cycle.
- **Bag substitutions.** A shown character's row can belong to a different
  underlying seat (for example the Drunk shown as the Empath). `ctx` exposes
  `playerIdForEntry(entryId)` to resolve the seat, and `displayedCharacterOf`
  to resolve what players see, so a helper classifies a seat by its shown token
  rather than its real role.

## Registration ranges (Recluse / Spy)

The Recluse (good, may register as evil) and the Spy (evil, may register as
good) make neighbour- and pair-counting helpers ambiguous. Registration is
decided per check at Storyteller discretion, so each ambiguous player varies
independently and the honest answer is a contiguous range rather than a single
number. The Empath, Chef, and Fortune Teller helpers therefore compute a
`{ min, max }` (rendered as "N" when `min === max`, else "N-M") and, when
`min !== max`, show a short hint naming the cause ("Recluse may register evil" /
"Spy may register good"). The Fortune Teller additionally reports
"NO (Recluse could register YES)" when a picked seat's real character is the
Recluse. Dead players never count and are skipped when walking to the next alive
neighbour.

## The 27 Trouble Brewing characters

Legend for "Night": F = has a first-night action, O = has an other-night action,
`-` = no night action (passive or day-only).

| Character     | Team      | Night | Helper today         | Proposal / rationale                                                                                                                                                                                 |
| ------------- | --------- | ----- | -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Washerwoman   | Townsfolk | F     | FirstNightInfoHelper | Implemented. Pick shown + decoy, attach "Townsfolk"/"Wrong" tokens, show card.                                                                                                                       |
| Librarian     | Townsfolk | F     | FirstNightInfoHelper | Implemented. As Washerwoman, plus a "No Outsiders" card mode.                                                                                                                                        |
| Investigator  | Townsfolk | F     | FirstNightInfoHelper | Implemented. As Washerwoman, with "Minion"/"Wrong" tokens.                                                                                                                                           |
| Chef          | Townsfolk | F     | ChefHelper           | Implemented. Counts adjacent evil pairs; range under Recluse/Spy.                                                                                                                                    |
| Empath        | Townsfolk | FO    | EmpathHelper         | Implemented. Counts evil neighbours; range under Recluse/Spy.                                                                                                                                        |
| Fortuneteller | Townsfolk | FO    | FortuneTellerHelper  | Implemented. Two-player pick; Red Herring aware; Recluse-may-yes.                                                                                                                                    |
| Undertaker    | Townsfolk | O     | UndertakerHelper     | Implemented. Names yesterday's execution.                                                                                                                                                            |
| Monk          | Townsfolk | O     | none                 | Proposed: single-player picker that attaches the "Safe" (protected) reminder token to the chosen player. Same pattern as the Fortune Teller pick plus `onattachreminder`.                            |
| Ravenkeeper   | Townsfolk | O     | none                 | Proposed: conditional helper, only when the Ravenkeeper died this night. Picker to choose a player, then reveal that player's real role and offer the "This player is" card.                         |
| Virgin        | Townsfolk | -     | none                 | No helper. Day-time nomination trigger; nothing to compute at night. Has a "No Ability" reminder the ST places manually.                                                                             |
| Slayer        | Townsfolk | -     | none                 | No helper. Day-time public ability; nothing at night.                                                                                                                                                |
| Soldier       | Townsfolk | -     | none                 | No helper. Passive Demon immunity; no ST input.                                                                                                                                                      |
| Mayor         | Townsfolk | -     | none                 | No helper. Passive win/bounce condition; no night action.                                                                                                                                            |
| Butler        | Outsider  | FO    | none                 | Proposed: single-player picker that attaches the "Master" reminder token. Same pattern as Monk.                                                                                                      |
| Drunk         | Outsider  | -     | none (bag sub)       | No standalone helper. Handled by the bag-substitution flow: the Drunk shows a Townsfolk token, and the "Is the Drunk" grimoire token can be dragged onto a Townsfolk seat to reassign the real role. |
| Recluse       | Outsider  | -     | none                 | No helper. Passive misregistration; surfaced as ranges in the Empath/Chef/FT helpers rather than its own widget.                                                                                     |
| Saint         | Outsider  | -     | none                 | No helper. Passive execution-loss condition; no night action.                                                                                                                                        |
| Poisoner      | Minion    | FO    | none                 | Proposed: single-player picker that attaches the "Poisoned" reminder token, which the state-aware night sheet already reads to flag impaired info.                                                   |
| Spy           | Minion    | FO    | none                 | Proposed: a deliberate "show grimoire" no-op note (the Spy sees the grimoire; there is nothing to compute). Also surfaced as ranges in Empath/Chef/FT via its may-register-good treatment.           |
| Scarletwoman  | Minion    | O     | none                 | Proposed: an alert that fires when the Demon dies with 5+ players alive, prompting the ST to promote the Scarlet Woman to Demon (attach "Is the Demon").                                             |
| Baron         | Minion    | -     | none                 | No helper. Passive setup modifier (+2 Outsiders); affects the bag at setup, not at night.                                                                                                            |
| Imp           | Demon     | O     | none (demon kill)    | Uses the existing demon-kill picker. Proposed extension: a star-pass flow when the Imp kills itself - prompt Minion promotion and a "You are" card shortcut.                                         |
| Beggar        | Traveller | -     | none                 | No helper. Day-time voting ability; no night action.                                                                                                                                                 |
| Bureaucrat    | Traveller | FO    | none                 | No helper planned. Attaches a "3 Votes" reminder to a player each night, but as a Traveller it is outside the core MVP scope; a Monk-style picker could be added later.                              |
| Gunslinger    | Traveller | -     | none                 | No helper. Day-time execution ability; no night action.                                                                                                                                              |
| Scapegoat     | Traveller | -     | none                 | No helper. Passive execution-redirect; no ST input.                                                                                                                                                  |
| Thief         | Traveller | FO    | none                 | No helper planned. Attaches a "Negative Vote" reminder each night; Traveller, out of core scope. Monk-style picker possible later.                                                                   |

## Summary

- **Implemented (7):** Empath, Chef, Undertaker, Fortune Teller, Washerwoman,
  Librarian, Investigator.
- **Proposed (single-player token pickers):** Monk, Butler, Poisoner (and,
  later, the Traveller pickers Bureaucrat and Thief).
- **Proposed (conditional / event-driven):** Ravenkeeper (died-at-night reveal),
  Scarlet Woman (Demon-death promotion alert), Imp (star-pass flow), Spy
  ("show grimoire" note).
- **No helper (passive or day-only):** Virgin, Slayer, Soldier, Mayor, Recluse,
  Saint, Baron, Drunk, Beggar, Gunslinger, Scapegoat. The Recluse and Spy still
  influence the Empath/Chef/Fortune Teller helpers through registration ranges.
