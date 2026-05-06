ALTER TABLE events
  ADD COLUMN submission_password_plain varchar(191) NULL AFTER submission_password_hash;
