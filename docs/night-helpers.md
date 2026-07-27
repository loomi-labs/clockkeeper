# Night Helpers - Trouble Brewing Audit

## What night helpers are

A night helper is a small, per-character interactive widget that renders inside a
character's row on the night sheet (the night-order checklist). Where the plain
night sheet only shows the character's ability text and a done/kill swipe, a
helper turns the Storyteller's mental arithmetic into a computed, tap-friendly
control:

- The **Empath** helper counts evil neighbours for you.
- The **Chef** helper counts pairs of adjacent evils.
- The **Undertaker** helper names the player executed on the previous day and can
  show that character's token card. It reports the executed seat's real
  character, so a Drunk shows as the Drunk.
- The **Fortune Teller** helper lets you pick two players and tells you whether a
  Demon (or Red Herring) is among them.
- The **Washerwoman / Librarian / Investigator** helper lets you pick the
  "shown" player and a decoy, then attaches the reminder tokens and shows the
  matching info card. The shown character defaults to the seat's token but can
  be overridden from the script's characters of that category.
- The **Butler**, **Poisoner**, **Monk**, **Bureaucrat** and **Thief** helpers are
  single-player token pickers (`TokenPickHelper`): pick a seat to attach the
  "Master" / "Poisoned" / "Safe" / "3 Votes" / "Negative Vote" reminder token,
  kept in sync with manual grimoire attachment.
- The **Ravenkeeper** helper renders only the night the Ravenkeeper dies: pick a
  player to learn their real character (a Drunk is learned as the Drunk) and show
  the character-token card.
- The **Scarlet Woman** helper is an alert that fires when the Demon dies with
  5+ non-Traveller players alive (counting the Demon before their death),
  opening the revertible promotion prompt.

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
   helper to the first night, other nights, or both. Optionally set
   `triggersOnOwnDeath: true` when the character acts *because* its own seat died
   (only the Ravenkeeper today) — the night sheet uses that flag to keep such a
   dead row live-styled and badge it "Died tonight — acts now". Any other dead
   seat stays dimmed, since dead players do not act.

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
- **Displayed vs. true character.** The bag-sub facade is only what the
  substituted player believes, so it applies to players'-perspective displays
  (`displayedCharacterOf`, used by the first-night info helpers) but never to
  info a character _learns_ from the grimoire. The **Undertaker** and
  **Ravenkeeper** therefore read `HelperPlayer.trueCharacter` (built by
  `resolveTrueCharacter` in `night-helpers/helpers.ts`): the seat's own role,
  ignoring any bag substitution, or the acts-as character when the seat has been
  promoted (a star-passed Baron really is the Imp now).

## Registration ranges (Recluse / Spy)

The Recluse (good, may register as evil) and the Spy (evil, may register as
good) make counting helpers ambiguous. Registration is decided per check at
Storyteller discretion, so each ambiguous player varies independently. The
exact semantics differ per helper:

- **Empath** (numeric range): counts evil among the two nearest _alive_
  neighbours - dead players never count and the walk continues to the next
  alive player. Ambiguity yields a `{ min, max }` range (rendered "N" when
  `min === max`, else "N-M") with a hint naming the cause ("Recluse may
  register evil" / "Spy may register good").
- **Chef** (numeric range): counts adjacent evil _pairs_ among alive players
  around the circle (first night, so usually everyone is alive). Same range
  and hint treatment as the Empath.
- **Fortune Teller** (boolean, not a range): the two chosen players may be
  alive or dead. The answer is YES when a pick is the Demon or the Red
  Herring; otherwise NO. A Recluse pick never forces a result - when the
  answer would be NO, the helper adds "NO (Recluse could register YES)" so
  the Storyteller can choose either legally; an actual Demon or Red Herring
  among the picks stays YES regardless.

## The 27 Trouble Brewing characters

Legend for "Night": F = has a first-night action, O = has an other-night action,
`-` = no night action (passive or day-only).

| Character     | Team      | Night | Helper today         | Proposal / rationale                                                                                                                                                                                 |
| ------------- | --------- | ----- | -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Washerwoman   | Townsfolk | F     | FirstNightInfoHelper | Implemented. Pick shown + decoy, attach "Townsfolk"/"Wrong" tokens, show card. Shown character defaults to the seat's token but can be overridden from the Townsfolk in the script.                  |
| Librarian     | Townsfolk | F     | FirstNightInfoHelper | Implemented. As Washerwoman, plus a "No Outsiders" card mode; the shown-character override is limited to the script's Outsiders.                                                                     |
| Investigator  | Townsfolk | F     | FirstNightInfoHelper | Implemented. As Washerwoman, with "Minion"/"Wrong" tokens; the shown-character override is limited to the script's Minions.                                                                          |
| Chef          | Townsfolk | F     | ChefHelper           | Implemented. Counts adjacent evil pairs; range under Recluse/Spy.                                                                                                                                    |
| Empath        | Townsfolk | FO    | EmpathHelper         | Implemented. Counts evil neighbours; range under Recluse/Spy.                                                                                                                                        |
| Fortuneteller | Townsfolk | FO    | FortuneTellerHelper  | Implemented. Two-player pick; Red Herring aware (Set-Red-Herring button attaches the token) plus a compact "Demon: …" display; Recluse-may-yes.                                                      |
| Undertaker    | Townsfolk | O     | UndertakerHelper     | Implemented. Names yesterday's execution, plus a "Show card" button that shows the bare character-token card. Uses the executed seat's TRUE character (grimoire truth): a Drunk shows as the Drunk, never the Townsfolk facade; a promoted seat shows its acts-as character. "Show different…" still overrides both. |
| Monk          | Townsfolk | O     | TokenPickHelper      | Implemented. Single-player picker (`TokenPickHelper`, excludes the Monk's own seat) that attaches the "Safe" (protected) reminder token; acts on other nights only.                                  |
| Ravenkeeper   | Townsfolk | O     | RavenkeeperHelper    | Implemented. Conditional — renders only the night the Ravenkeeper died (via `diedTonight`). Pick a player to learn their TRUE character (grimoire truth): a Drunk is learned as the Drunk, never the Townsfolk facade; a promoted seat is learned as its acts-as character. Shows the bare character-token card; "Show different…" still overrides. |
| Virgin        | Townsfolk | -     | none                 | No helper. Day-time nomination trigger; nothing to compute at night. Has a "No Ability" reminder the ST places manually.                                                                             |
| Slayer        | Townsfolk | -     | none                 | No helper. Day-time public ability; nothing at night.                                                                                                                                                |
| Soldier       | Townsfolk | -     | none                 | No helper. Passive Demon immunity; no ST input.                                                                                                                                                      |
| Mayor         | Townsfolk | -     | none                 | No helper. Passive win/bounce condition; no night action.                                                                                                                                            |
| Butler        | Outsider  | FO    | TokenPickHelper      | Implemented. Single-player picker (excludes the Butler's own seat) that attaches the "Master" reminder token; bidirectional with manual grimoire attachment.                                         |
| Drunk         | Outsider  | -     | none (bag sub)       | No standalone helper. Handled by the bag-substitution flow: the Drunk shows a Townsfolk token, and the "Is the Drunk" grimoire token can be dragged onto a Townsfolk seat to reassign the real role. The facade is a player-perspective display only — the Drunk's real character is the Drunk, so the Undertaker and Ravenkeeper learn "Drunk" (see `HelperPlayer.trueCharacter`). |
| Recluse       | Outsider  | -     | none                 | No helper. Passive misregistration; surfaced as ranges in the Empath/Chef/FT helpers rather than its own widget.                                                                                     |
| Saint         | Outsider  | -     | none                 | No helper. Passive execution-loss condition; no night action.                                                                                                                                        |
| Poisoner      | Minion    | FO    | TokenPickHelper      | Implemented. Single-player picker (may target anyone, including the Poisoner) that attaches the "Poisoned" reminder token, which the state-aware night sheet already reads to flag impaired info.    |
| Spy           | Minion    | FO    | none                 | Proposed: a deliberate "show grimoire" no-op note (the Spy sees the grimoire; there is nothing to compute). Also surfaced as ranges in Empath/Chef/FT via its may-register-good treatment.           |
| Scarletwoman  | Minion    | O     | ScarletWomanHelper   | Implemented. Amber alert that fires when every Demon is dead with 5+ non-Traveller players alive (counted before the death), with a Promote button that opens the revertible star-pass / promotion prompt (`onstarpass`).                     |
| Baron         | Minion    | -     | none                 | No helper. Passive setup modifier (+2 Outsiders); affects the bag at setup, not at night.                                                                                                            |
| Imp           | Demon     | O     | none (demon kill)    | Uses the existing demon-kill picker. Star pass is a revertible promotion: when the Imp dies, `onstarpass` opens the prompt to promote a Minion to the Demon ("Imp (ex Baron)"), overlaid via `rolePromotions` and reversible.                                         |
| Beggar        | Traveller | -     | none                 | No helper. Day-time voting ability; no night action.                                                                                                                                                 |
| Bureaucrat    | Traveller | FO    | TokenPickHelper      | Implemented. Single-player picker (excludes the Bureaucrat's own seat) that attaches the "3 Votes" reminder token; acts on both nights.                                                               |
| Gunslinger    | Traveller | -     | none                 | No helper. Day-time execution ability; no night action.                                                                                                                                              |
| Scapegoat     | Traveller | -     | none                 | No helper. Passive execution-redirect; no ST input.                                                                                                                                                  |
| Thief         | Traveller | FO    | TokenPickHelper      | Implemented. Single-player picker (excludes the Thief's own seat) that attaches the "Negative Vote" reminder token; acts on both nights.                                                              |

## Summary

- **Implemented (14):** Empath, Chef, Undertaker, Fortune Teller, Washerwoman,
  Librarian, Investigator, Butler, Poisoner, Monk, Bureaucrat, Thief (these five
  via the shared `TokenPickHelper`), Ravenkeeper (conditional died-at-night
  reveal), and Scarlet Woman (Demon-death promotion alert).
- **Proposed (conditional / event-driven):** Imp (star pass — now a revertible
  promotion via `onstarpass` + `rolePromotions`), Spy ("show grimoire" note).
- **No helper (passive or day-only):** Virgin, Slayer, Soldier, Mayor, Recluse,
  Saint, Baron, Drunk, Beggar, Gunslinger, Scapegoat. The Recluse and Spy still
  influence the Empath/Chef/Fortune Teller helpers through registration ranges.
