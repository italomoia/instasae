CREATE TABLE conversations (
    id                       uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id               uuid        NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    contact_id               uuid        NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    chatwoot_conversation_id integer     NOT NULL,
    last_customer_message_at timestamptz NOT NULL,
    is_active                boolean     NOT NULL DEFAULT true,
    created_at               timestamptz NOT NULL DEFAULT NOW(),
    updated_at               timestamptz NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, contact_id, chatwoot_conversation_id)
);

CREATE INDEX idx_conversations_account_contact ON conversations(account_id, contact_id)
    WHERE is_active = true;

CREATE INDEX idx_conversations_chatwoot ON conversations(chatwoot_conversation_id)
    WHERE is_active = true;
