ALTER TABLE photos
  ADD COLUMN like_count bigint unsigned NOT NULL DEFAULT 0 AFTER size_bytes;

CREATE TABLE photo_likes (
  photo_id bigint unsigned NOT NULL,
  fingerprint_hash char(64) NOT NULL,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (photo_id, fingerprint_hash),
  KEY photo_likes_created_at_index (created_at),
  CONSTRAINT photo_likes_photo_id_foreign FOREIGN KEY (photo_id) REFERENCES photos (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
