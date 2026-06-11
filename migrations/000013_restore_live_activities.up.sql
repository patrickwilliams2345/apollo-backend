-- Restores the live_activities table dropped in 000009, slimmed for the
-- account-backed design: Reddit OAuth tokens now live on the accounts row
-- (looked up via reddit_account_id) instead of being duplicated here.
-- apns_token is text because ActivityKit push tokens (~160 hex chars) exceed
-- the old varchar(100).
CREATE TABLE IF NOT EXISTS live_activities (
    id SERIAL PRIMARY KEY,
    apns_token text UNIQUE NOT NULL,
    reddit_account_id character varying(32) NOT NULL DEFAULT '',
    thread_id character varying(32) NOT NULL DEFAULT '',
    subreddit character varying(32) NOT NULL DEFAULT '',
    next_check_at timestamp without time zone,
    expires_at timestamp without time zone,
    development boolean DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS live_activities_next_check_at_idx ON live_activities(next_check_at);
