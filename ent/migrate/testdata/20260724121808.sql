-- Seed data for add_info_cards migration.
-- Earlier seed users are deleted by the add_discord_and_anonymous migration,
-- so create a fresh user to own the info card.
INSERT INTO users (created_at, updated_at, uuid, is_anonymous, last_active_at)
VALUES (NOW(), NOW(), 'info-card-seed-user', false, NOW());

INSERT INTO info_cards (created_at, updated_at, title, body, character_ids, sort_order, user_id)
VALUES (NOW(), NOW(), 'Seed Info Card', 'Seed body text', '["washerwoman"]', 0,
  (SELECT id FROM users WHERE uuid = 'info-card-seed-user'));
