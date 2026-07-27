-- Clock Keeper dev seed data.
--
-- Idempotent — safe to re-run. LOCAL DEVELOPMENT ONLY: it creates the fixed
-- DEV_SINGLE_USER account plus a couple of ready-made games so `task dev` starts
-- with something to click on.
--
-- Load with: task db:seed

BEGIN;

-- 1. The fixed local-dev user.
--    The uuid must match devSingleUserUUID in internal/web/api_auth.go.
--    is_anonymous is false on purpose: that excludes it (and its games) from the
--    stale-anonymous-user cleanup in internal/web/cleanup.go.
INSERT INTO users (created_at, updated_at, uuid, is_anonymous, last_active_at)
VALUES (NOW(), NOW(), 'dev-local-user', false, NOW())
ON CONFLICT (uuid) DO NOTHING;

-- 2. A 7-player Trouble Brewing game still in setup.
INSERT INTO games (
  created_at, updated_at, name, user_id, script_id,
  player_count, traveller_count,
  selected_roles, selected_travellers, extra_characters, selected_bluffs,
  traveller_alignments, bag_substitutions, role_promotions,
  grimoire_positions, grimoire_player_names,
  grimoire_game_notes, grimoire_round_notes, grimoire_reminder_attachments,
  state
)
SELECT
  NOW(), NOW(), 'Dev Seed - TB Setup', u.id, s.id,
  7, 0,
  '["washerwoman","empath","fortuneteller","undertaker","recluse","poisoner","imp"]'::jsonb,
  '[]'::jsonb, '[]'::jsonb, '[]'::jsonb,
  '{}'::jsonb, '[]'::jsonb, '[]'::jsonb,
  '{}'::jsonb,
  '{
     "washerwoman": "Alice",
     "empath": "Bob",
     "fortuneteller": "Carla",
     "undertaker": "Dorian",
     "recluse": "Elena",
     "poisoner": "Femi",
     "imp": "Gus"
   }'::jsonb,
  '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
  'setup'
FROM users u
CROSS JOIN (SELECT id FROM scripts WHERE edition = 'tb' AND is_system LIMIT 1) s
WHERE u.uuid = 'dev-local-user'
  AND NOT EXISTS (
    SELECT 1 FROM games g
    WHERE g.user_id = u.id AND g.name = 'Dev Seed - TB Setup'
  );

-- 3. A 10-player Trouble Brewing game mid-play. The role set deliberately contains
--    ravenkeeper / undertaker / monk / scarletwoman / imp so the night helpers that
--    depend on them can be exercised without editing the game first.
INSERT INTO games (
  created_at, updated_at, name, user_id, script_id,
  player_count, traveller_count,
  selected_roles, selected_travellers, extra_characters, selected_bluffs,
  traveller_alignments, bag_substitutions, role_promotions,
  grimoire_positions, grimoire_player_names,
  grimoire_game_notes, grimoire_round_notes, grimoire_reminder_attachments,
  state
)
SELECT
  NOW(), NOW(), 'Dev Seed - TB Night 1', u.id, s.id,
  10, 0,
  '["washerwoman","librarian","investigator","chef","soldier","monk","ravenkeeper","undertaker","scarletwoman","imp"]'::jsonb,
  '[]'::jsonb, '[]'::jsonb, '[]'::jsonb,
  '{}'::jsonb, '[]'::jsonb, '[]'::jsonb,
  '{}'::jsonb,
  '{
     "washerwoman": "Alice",
     "librarian": "Bob",
     "investigator": "Carla",
     "chef": "Dorian",
     "soldier": "Elena",
     "monk": "Femi",
     "ravenkeeper": "Gus",
     "undertaker": "Hana",
     "scarletwoman": "Ivan",
     "imp": "Jo"
   }'::jsonb,
  '{}'::jsonb, '{}'::jsonb, '{}'::jsonb,
  'in_progress'
FROM users u
CROSS JOIN (SELECT id FROM scripts WHERE edition = 'tb' AND is_system LIMIT 1) s
WHERE u.uuid = 'dev-local-user'
  AND NOT EXISTS (
    SELECT 1 FROM games g
    WHERE g.user_id = u.id AND g.name = 'Dev Seed - TB Night 1'
  );

-- 4. Round-1 phases for the in-progress game (mirrors what StartGame creates:
--    an active night plus an inactive day).
INSERT INTO phases (
  created_at, updated_at, game_id, round_number, type, is_active,
  completed_actions, character_alignments
)
SELECT NOW(), NOW(), g.id, 1, 'night', true, '[]'::jsonb, '{}'::jsonb
FROM games g
JOIN users u ON u.id = g.user_id
WHERE u.uuid = 'dev-local-user'
  AND g.name = 'Dev Seed - TB Night 1'
  AND NOT EXISTS (
    SELECT 1 FROM phases p
    WHERE p.game_id = g.id AND p.round_number = 1 AND p.type = 'night'
  );

INSERT INTO phases (
  created_at, updated_at, game_id, round_number, type, is_active,
  completed_actions, character_alignments
)
SELECT NOW(), NOW(), g.id, 1, 'day', false, '[]'::jsonb, '{}'::jsonb
FROM games g
JOIN users u ON u.id = g.user_id
WHERE u.uuid = 'dev-local-user'
  AND g.name = 'Dev Seed - TB Night 1'
  AND NOT EXISTS (
    SELECT 1 FROM phases p
    WHERE p.game_id = g.id AND p.round_number = 1 AND p.type = 'day'
  );

-- 5. One death on Night 1 so the grimoire has a dead seat. Soldier is chosen so the
--    ravenkeeper/undertaker helpers stay testable against a live role.
INSERT INTO deaths (created_at, updated_at, phase_id, role_id, ghost_vote, cause)
SELECT NOW(), NOW(), p.id, 'soldier', true, 'demon'
FROM phases p
JOIN games g ON g.id = p.game_id
JOIN users u ON u.id = g.user_id
WHERE u.uuid = 'dev-local-user'
  AND g.name = 'Dev Seed - TB Night 1'
  AND p.round_number = 1
  AND p.type = 'night'
ON CONFLICT (role_id, phase_id) DO NOTHING;

COMMIT;
