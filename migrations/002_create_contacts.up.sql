CREATE TABLE contacts (
    id                         uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id                 uuid         NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    ig_sender_id               varchar(64)  NOT NULL,
    chatwoot_contact_id        integer      NOT NULL,
    chatwoot_contact_source_id varchar(255) NOT NULL,
    ig_username                varchar(255),
    ig_name                    varchar(255),
    ig_profile_pic             text,
    created_at                 timestamptz  NOT NULL DEFAULT NOW(),
    updated_at                 timestamptz  NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, ig_sender_id)
);

CREATE UNIQUE INDEX idx_contacts_account_sender ON contacts(account_id, ig_sender_id);
