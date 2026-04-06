-- Drop unused columns from creators (curation model doesn't need portfolio fields)
ALTER TABLE creators DROP COLUMN IF EXISTS bio;
ALTER TABLE creators DROP COLUMN IF EXISTS roles;
ALTER TABLE creators DROP COLUMN IF EXISTS contacts;

-- Drop the roles GIN index
DROP INDEX IF EXISTS idx_creators_roles;
