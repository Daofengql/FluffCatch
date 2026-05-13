ALTER TABLE admin_users
  ADD COLUMN oidc_subject varchar(255) NULL,
  ADD COLUMN oidc_username varchar(191) NULL,
  ADD COLUMN oidc_email varchar(191) NULL,
  ADD UNIQUE KEY admin_users_oidc_subject_unique (oidc_subject);
