CREATE TABLE settings (
  `key` varchar(191) NOT NULL,
  `value` json NOT NULL,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE admin_users (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  username varchar(191) NOT NULL,
  password_hash varchar(255) NOT NULL,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY admin_users_username_unique (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE sessions (
  id char(64) NOT NULL,
  admin_user_id bigint unsigned NOT NULL,
  expires_at timestamp NOT NULL,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY sessions_admin_user_id_index (admin_user_id),
  KEY sessions_expires_at_index (expires_at),
  CONSTRAINT sessions_admin_user_id_foreign FOREIGN KEY (admin_user_id) REFERENCES admin_users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE events (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  title varchar(191) NOT NULL,
  description text NOT NULL,
  location varchar(191) NOT NULL,
  starts_at datetime NULL,
  ends_at datetime NULL,
  cover_storage_policy_id varchar(191) NULL,
  cover_object_key varchar(512) NULL,
  cover_thumbnail_key varchar(512) NULL,
  is_public boolean NOT NULL DEFAULT true,
  submission_enabled boolean NOT NULL DEFAULT true,
  submission_password_hash varchar(255) NULL,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY events_public_starts_at_index (is_public, starts_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE photos (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  event_id bigint unsigned NOT NULL,
  storage_policy_id varchar(191) NOT NULL,
  object_key varchar(512) NOT NULL,
  thumbnail_key varchar(512) NULL,
  original_filename varchar(255) NOT NULL,
  content_type varchar(100) NOT NULL,
  size_bytes bigint unsigned NOT NULL DEFAULT 0,
  photographer_name varchar(191) NULL,
  visibility enum('public', 'protected', 'private') NOT NULL DEFAULT 'public',
  access_password_hash varchar(255) NULL,
  exif json NULL,
  taken_at datetime NULL,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY photos_event_visibility_index (event_id, visibility),
  KEY photos_storage_policy_id_index (storage_policy_id),
  KEY photos_taken_at_index (taken_at),
  CONSTRAINT photos_event_id_foreign FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE submissions (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  event_id bigint unsigned NOT NULL,
  storage_policy_id varchar(191) NOT NULL,
  object_key varchar(512) NOT NULL,
  thumbnail_key varchar(512) NULL,
  original_filename varchar(255) NOT NULL,
  content_type varchar(100) NOT NULL,
  size_bytes bigint unsigned NOT NULL DEFAULT 0,
  photographer_name varchar(191) NULL,
  tags json NOT NULL,
  status enum('pending', 'approved') NOT NULL DEFAULT 'pending',
  exif json NULL,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  approved_at timestamp NULL,
  PRIMARY KEY (id),
  KEY submissions_event_status_index (event_id, status),
  KEY submissions_storage_policy_id_index (storage_policy_id),
  CONSTRAINT submissions_event_id_foreign FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE tags (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  name varchar(191) NOT NULL,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY tags_name_unique (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE photo_tags (
  photo_id bigint unsigned NOT NULL,
  tag_id bigint unsigned NOT NULL,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (photo_id, tag_id),
  KEY photo_tags_tag_id_index (tag_id),
  CONSTRAINT photo_tags_photo_id_foreign FOREIGN KEY (photo_id) REFERENCES photos (id) ON DELETE CASCADE,
  CONSTRAINT photo_tags_tag_id_foreign FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE access_grants (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  photo_id bigint unsigned NULL,
  event_id bigint unsigned NULL,
  token_hash char(64) NOT NULL,
  expires_at timestamp NULL,
  created_at timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY access_grants_token_hash_unique (token_hash),
  KEY access_grants_photo_id_index (photo_id),
  KEY access_grants_event_id_index (event_id),
  CONSTRAINT access_grants_photo_id_foreign FOREIGN KEY (photo_id) REFERENCES photos (id) ON DELETE CASCADE,
  CONSTRAINT access_grants_event_id_foreign FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
