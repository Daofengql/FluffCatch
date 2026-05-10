CREATE TABLE submission_links (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  event_id bigint unsigned NOT NULL,
  token_hash char(64) NOT NULL,
  label varchar(191) NOT NULL,
  photographer_name varchar(191) NULL,
  expires_at timestamp NULL,
  max_uses int unsigned NOT NULL DEFAULT 0,
  use_count int unsigned NOT NULL DEFAULT 0,
  revoked_at timestamp NULL,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY submission_links_token_hash_unique (token_hash),
  KEY submission_links_event_id_index (event_id),
  KEY submission_links_expires_at_index (expires_at),
  CONSTRAINT submission_links_event_id_foreign FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

ALTER TABLE photos
  ADD KEY photos_photographer_index (photographer_name),
  ADD KEY photos_taken_at_index (taken_at);

ALTER TABLE submissions
  ADD COLUMN taken_at datetime NULL,
  ADD KEY submissions_taken_at_index (taken_at);
