ALTER TABLE songs DROP CONSTRAINT IF EXISTS fk_unique_song;
ALTER TABLE songs ADD CONSTRAINT fk_unique_song UNIQUE(group_id, song_name);