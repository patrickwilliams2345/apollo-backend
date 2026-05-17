-- Reddit access tokens migrated from short opaque strings (~30 chars) to JWTs
-- (~1100+ chars) sometime after the original backend was archived. The
-- varchar(64) cap was sized for the old format and overflows on JWT writes
-- with `SQLSTATE 22001: value too long for type character varying(64)`.
-- Refresh tokens are still short opaque strings today but use the same type
-- for consistency.
ALTER TABLE accounts ALTER COLUMN access_token TYPE text;
ALTER TABLE accounts ALTER COLUMN refresh_token TYPE text;
