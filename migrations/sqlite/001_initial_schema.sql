CREATE TABLE settings (
  "key" text NOT NULL,
  "value" text NOT NULL,
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("key")
);

CREATE TABLE events (
  id integer NOT NULL PRIMARY KEY AUTOINCREMENT,
  title text NOT NULL,
  description text NOT NULL,
  location text NOT NULL,
  province_code text NULL,
  province_name text NULL,
  city_code text NULL,
  city_name text NULL,
  starts_at datetime NULL,
  ends_at datetime NULL,
  cover_storage_policy_id text NULL,
  cover_object_key text NULL,
  cover_thumbnail_key text NULL,
  is_public boolean NOT NULL DEFAULT 1,
  submission_enabled boolean NOT NULL DEFAULT 1,
  private_password_hash text NULL,
  private_password_plain text NULL,
  sort_at datetime NOT NULL,
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX events_sort_index ON events (sort_at, id);
CREATE INDEX events_public_sort_index ON events (is_public, sort_at, id);
CREATE INDEX events_region_index ON events (province_code, city_code);
CREATE INDEX events_cover_policy_object_public_index ON events (cover_storage_policy_id, cover_object_key, is_public);

CREATE TABLE photos (
  id integer NOT NULL PRIMARY KEY AUTOINCREMENT,
  event_id integer NOT NULL,
  storage_policy_id text NOT NULL,
  object_key text NOT NULL,
  thumbnail_key text NULL,
  content_hash text NOT NULL,
  content_type text NOT NULL,
  size_bytes integer NOT NULL DEFAULT 0,
  like_count integer NOT NULL DEFAULT 0,
  photographer_name text NULL,
  visibility text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private')),
  exif text NULL,
  taken_at datetime NULL,
  sort_at datetime NOT NULL,
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT photos_event_id_foreign FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE CASCADE,
  UNIQUE (event_id, content_hash)
);

CREATE INDEX photos_event_sort_index ON photos (event_id, sort_at, id);
CREATE INDEX photos_event_visibility_sort_index ON photos (event_id, visibility, sort_at, id);
CREATE INDEX photos_storage_policy_id_index ON photos (storage_policy_id);
CREATE INDEX photos_policy_object_public_index ON photos (storage_policy_id, object_key, visibility, event_id);
CREATE INDEX photos_policy_thumbnail_public_index ON photos (storage_policy_id, thumbnail_key, visibility, event_id);
CREATE INDEX photos_photographer_index ON photos (photographer_name);
CREATE INDEX photos_taken_at_index ON photos (taken_at);

CREATE TABLE submissions (
  id integer NOT NULL PRIMARY KEY AUTOINCREMENT,
  event_id integer NOT NULL,
  storage_policy_id text NOT NULL,
  object_key text NOT NULL,
  thumbnail_key text NULL,
  content_hash text NOT NULL,
  content_type text NOT NULL,
  size_bytes integer NOT NULL DEFAULT 0,
  photographer_name text NULL,
  tags text NOT NULL,
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved')),
  exif text NULL,
  taken_at datetime NULL,
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  approved_at datetime NULL,
  CONSTRAINT submissions_event_id_foreign FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE CASCADE,
  UNIQUE (event_id, content_hash)
);

CREATE INDEX submissions_event_status_created_index ON submissions (event_id, status, created_at, id);
CREATE INDEX submissions_status_created_index ON submissions (status, created_at, id);
CREATE INDEX submissions_storage_policy_id_index ON submissions (storage_policy_id);
CREATE INDEX submissions_taken_at_index ON submissions (taken_at);

CREATE TABLE tags (
  id integer NOT NULL PRIMARY KEY AUTOINCREMENT,
  name text NOT NULL,
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (name)
);

CREATE TABLE photo_tags (
  photo_id integer NOT NULL,
  tag_id integer NOT NULL,
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (photo_id, tag_id),
  CONSTRAINT photo_tags_photo_id_foreign FOREIGN KEY (photo_id) REFERENCES photos (id) ON DELETE CASCADE,
  CONSTRAINT photo_tags_tag_id_foreign FOREIGN KEY (tag_id) REFERENCES tags (id) ON DELETE CASCADE
);

CREATE INDEX photo_tags_tag_id_index ON photo_tags (tag_id);

CREATE TABLE photo_likes (
  photo_id integer NOT NULL,
  fingerprint_hash text NOT NULL,
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (photo_id, fingerprint_hash),
  CONSTRAINT photo_likes_photo_id_foreign FOREIGN KEY (photo_id) REFERENCES photos (id) ON DELETE CASCADE
);

CREATE INDEX photo_likes_created_at_index ON photo_likes (created_at);

CREATE TABLE submission_links (
  id integer NOT NULL PRIMARY KEY AUTOINCREMENT,
  event_id integer NOT NULL,
  token_hash text NOT NULL,
  label text NOT NULL,
  photographer_name text NULL,
  expires_at datetime NULL,
  max_uses integer NOT NULL DEFAULT 0,
  use_count integer NOT NULL DEFAULT 0,
  revoked_at datetime NULL,
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT submission_links_event_id_foreign FOREIGN KEY (event_id) REFERENCES events (id) ON DELETE CASCADE,
  UNIQUE (token_hash)
);

CREATE INDEX submission_links_event_id_index ON submission_links (event_id);
CREATE INDEX submission_links_expires_at_index ON submission_links (expires_at);
