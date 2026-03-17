CREATE TABLE accounts (
    id                   uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    ig_page_id           varchar(64)  NOT NULL UNIQUE,
    ig_page_name         varchar(255),
    ig_access_token      text         NOT NULL,
    chatwoot_base_url    varchar(512) NOT NULL,
    chatwoot_account_id  integer      NOT NULL,
    chatwoot_inbox_id    integer      NOT NULL,
    chatwoot_api_token   text         NOT NULL,
    webhook_verify_token varchar(128) NOT NULL,
    token_expires_at     timestamptz,
    is_active            boolean      NOT NULL DEFAULT true,
    created_at           timestamptz  NOT NULL DEFAULT NOW(),
    updated_at           timestamptz  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_accounts_ig_page_id ON accounts(ig_page_id);

CREATE INDEX idx_accounts_token_expires ON accounts(token_expires_at)
    WHERE is_active = true AND token_expires_at IS NOT NULL;
