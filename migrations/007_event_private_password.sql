ALTER TABLE events
  ADD COLUMN private_password_hash varchar(255) NULL AFTER submission_password_plain,
  ADD COLUMN private_password_plain varchar(191) NULL AFTER private_password_hash;

UPDATE photos
SET visibility = 'private'
WHERE visibility = 'protected';

ALTER TABLE photos
  MODIFY visibility enum('public', 'private') NOT NULL DEFAULT 'public',
  DROP COLUMN access_password_hash;
