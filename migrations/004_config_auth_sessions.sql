ALTER TABLE sessions
  DROP FOREIGN KEY sessions_admin_user_id_foreign;

ALTER TABLE sessions
  ADD COLUMN username varchar(191) NULL AFTER id;

UPDATE sessions
  INNER JOIN admin_users ON admin_users.id = sessions.admin_user_id
  SET sessions.username = admin_users.username
  WHERE sessions.username IS NULL;

ALTER TABLE sessions
  MODIFY username varchar(191) NOT NULL,
  DROP KEY sessions_admin_user_id_index,
  DROP COLUMN admin_user_id,
  ADD KEY sessions_username_index (username);

DROP TABLE admin_users;
