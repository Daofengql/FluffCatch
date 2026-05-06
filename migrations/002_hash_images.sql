ALTER TABLE photos
  ADD COLUMN content_hash char(64) NOT NULL DEFAULT '' AFTER thumbnail_key;

UPDATE photos
SET content_hash = SHA2(CONCAT(storage_policy_id, ':', object_key), 256)
WHERE content_hash = '';

ALTER TABLE photos
  DROP COLUMN original_filename,
  ADD UNIQUE KEY photos_event_hash_unique (event_id, content_hash);

ALTER TABLE submissions
  ADD COLUMN content_hash char(64) NOT NULL DEFAULT '' AFTER thumbnail_key;

UPDATE submissions
SET content_hash = SHA2(CONCAT(storage_policy_id, ':', object_key), 256)
WHERE content_hash = '';

ALTER TABLE submissions
  DROP COLUMN original_filename,
  ADD UNIQUE KEY submissions_event_hash_unique (event_id, content_hash);
