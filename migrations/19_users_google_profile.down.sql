-- Gerege Template Version 27.0
-- 19_users_google_profile-ийг буцаана.

ALTER TABLE users
  DROP COLUMN IF EXISTS google_email,
  DROP COLUMN IF EXISTS google_email_verified,
  DROP COLUMN IF EXISTS google_name,
  DROP COLUMN IF EXISTS google_picture,
  DROP COLUMN IF EXISTS google_linked_at;
