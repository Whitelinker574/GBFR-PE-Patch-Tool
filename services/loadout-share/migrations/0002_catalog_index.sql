ALTER TABLE loadouts ADD COLUMN character_slug TEXT NOT NULL DEFAULT '';
ALTER TABLE loadouts ADD COLUMN weapon_name TEXT NOT NULL DEFAULT '';
ALTER TABLE loadouts ADD COLUMN weapon_name_en TEXT NOT NULL DEFAULT '';
ALTER TABLE loadouts ADD COLUMN search_text TEXT NOT NULL DEFAULT '';
ALTER TABLE loadouts ADD COLUMN search_text_en TEXT NOT NULL DEFAULT '';
ALTER TABLE loadouts ADD COLUMN preview_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE loadouts ADD COLUMN preview_en_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE loadouts ADD COLUMN catalog_ready INTEGER NOT NULL DEFAULT 0;
ALTER TABLE loadouts ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';
ALTER TABLE loadouts ADD COLUMN title_sort TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS loadouts_catalog_time
  ON loadouts(catalog_ready, created_at DESC, code DESC);
CREATE INDEX IF NOT EXISTS loadouts_catalog_character_time
  ON loadouts(catalog_ready, character_slug, created_at DESC, code DESC);
CREATE INDEX IF NOT EXISTS loadouts_catalog_character_name
  ON loadouts(catalog_ready, character_slug, title_sort ASC, code ASC);
CREATE INDEX IF NOT EXISTS loadouts_catalog_character_likes
  ON loadouts(catalog_ready, character_slug, likes_count DESC, code ASC);
CREATE INDEX IF NOT EXISTS loadouts_catalog_name
  ON loadouts(catalog_ready, title_sort ASC, code ASC);
CREATE INDEX IF NOT EXISTS loadouts_catalog_likes
  ON loadouts(catalog_ready, likes_count DESC, code ASC);
