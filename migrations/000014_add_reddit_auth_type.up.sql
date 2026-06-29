ALTER TABLE accounts
    ADD COLUMN reddit_auth_type varchar(16) NOT NULL DEFAULT 'oauth';
