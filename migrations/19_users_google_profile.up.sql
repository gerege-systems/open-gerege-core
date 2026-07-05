-- Gerege Template Version 27.0
-- Google профайл: холбогдсон Google account-аас ирсэн бүх мэдээллийг хадгална
-- (email, баталгаажсан эсэх, нэр, зураг, холбосон огноо). Dashboard дээр харуулна.

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS google_email          TEXT,
  ADD COLUMN IF NOT EXISTS google_email_verified BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS google_name           TEXT,
  ADD COLUMN IF NOT EXISTS google_picture        TEXT,
  ADD COLUMN IF NOT EXISTS google_linked_at      TIMESTAMPTZ;
