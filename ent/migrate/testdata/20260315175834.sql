-- Seed data for add_scripts_and_games migration.
INSERT INTO scripts (created_at, updated_at, name, edition, character_ids, user_id)
VALUES (NOW(), NOW(), 'My TB Script', 'tb', '["washerwoman","librarian","investigator","chef"]', 1);

INSERT INTO games (created_at, updated_at, player_count, selected_roles, state, script_id)
VALUES (NOW(), NOW(), 7, '["washerwoman","librarian","investigator"]', 'setup', 1);
