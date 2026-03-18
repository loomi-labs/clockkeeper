-- Ensure non-system scripts always have an owner.
ALTER TABLE scripts ADD CONSTRAINT chk_script_has_owner
    CHECK (is_system = true OR user_id IS NOT NULL);
