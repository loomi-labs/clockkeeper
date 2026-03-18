-- Modify "games" table
ALTER TABLE "games" ALTER COLUMN "selected_travellers" DROP DEFAULT;
-- Modify "scripts" table
ALTER TABLE "scripts" DROP CONSTRAINT "scripts_users_scripts", ALTER COLUMN "user_id" DROP NOT NULL, ADD COLUMN "is_system" boolean NOT NULL DEFAULT false, ADD CONSTRAINT "scripts_users_scripts" FOREIGN KEY ("user_id") REFERENCES "users" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Seed system scripts (base editions)
INSERT INTO scripts (created_at, updated_at, name, edition, is_system, character_ids) VALUES
  (NOW(), NOW(), 'Trouble Brewing', 'tb', true, '["baron","butler","chef","drunk","empath","fortuneteller","imp","investigator","librarian","mayor","monk","poisoner","ravenkeeper","recluse","saint","scarletwoman","slayer","soldier","spy","undertaker","virgin","washerwoman"]'::jsonb),
  (NOW(), NOW(), 'Bad Moon Rising', 'bmr', true, '["assassin","chambermaid","courtier","devilsadvocate","exorcist","fool","gambler","godfather","goon","gossip","grandmother","innkeeper","lunatic","mastermind","minstrel","moonchild","pacifist","po","professor","pukka","sailor","shabaloth","tealady","tinker","zombuul"]'::jsonb),
  (NOW(), NOW(), 'Sects & Violets', 'snv', true, '["artist","barber","cerenovus","clockmaker","dreamer","eviltwin","fanggu","flowergirl","juggler","klutz","mathematician","mutant","nodashii","oracle","philosopher","pithag","sage","savant","seamstress","snakecharmer","sweetheart","towncrier","vigormortis","vortox","witch"]'::jsonb);
