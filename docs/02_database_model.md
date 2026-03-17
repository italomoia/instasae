# Database Model — instasae

## About this document

This is the single source of truth for the instasae database schema. Every table, field, type, constraint, and relationship is documented here. The database name is `instasae`, running on the existing PostgreSQL instance.

## How the data flows

```
  Instagram webhook arrives
         │
         ▼
  ┌─── accounts ───┐     Lookup by ig_page_id
  │ ig_page_id     │     to find which Chatwoot
  │ ig_access_token│     inbox to route to
  │ chatwoot_*     │
  └───────┬────────┘
          │
          ▼
  ┌─── contacts ───┐     Lookup/create by ig_sender_id
  │ ig_sender_id   │     to map Instagram user to
  │ chatwoot_*     │     Chatwoot contact
  │ account_id (FK)│
  └───────┬────────┘
          │
          ▼
  ┌── conversations ─┐   Lookup/create active conversation
  │ chatwoot_conv_id  │   for this contact in this account
  │ contact_id (FK)   │
  │ account_id (FK)   │
  │ last_customer_msg │
  └───────────────────┘
```

The data model is intentionally minimal. instasae does NOT store message content — it only stores the mappings needed to route messages between Instagram and Chatwoot.

## Tables

### accounts

Stores the configuration for each Instagram account connected to a Chatwoot inbox. This is the central table — every webhook lookup starts here.

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK, DEFAULT gen_random_uuid() |
| ig_page_id | varchar(64) | NOT NULL, UNIQUE. The Instagram Page ID (recipient.id in webhooks) |
| ig_page_name | varchar(255) | Display name for admin reference |
| ig_access_token | text | NOT NULL. Encrypted with AES-256-GCM. Long-lived Instagram User Access Token |
| chatwoot_base_url | varchar(512) | NOT NULL. e.g. https://chat.example.com |
| chatwoot_account_id | integer | NOT NULL. Chatwoot account numeric ID |
| chatwoot_inbox_id | integer | NOT NULL. Chatwoot API channel inbox numeric ID |
| chatwoot_api_token | text | NOT NULL. Encrypted. Chatwoot user/bot API access token |
| webhook_verify_token | varchar(128) | NOT NULL. Token for Meta webhook verification handshake |
| token_expires_at | timestamptz | NULL. When the IG access token expires. NULL = unknown |
| is_active | boolean | NOT NULL, DEFAULT true. Inactive accounts are skipped |
| created_at | timestamptz | NOT NULL, DEFAULT NOW() |
| updated_at | timestamptz | NOT NULL, DEFAULT NOW() |

Notes:
- `ig_page_id` is UNIQUE because one Instagram account maps to exactly one Chatwoot inbox in this system.
- `ig_access_token` and `chatwoot_api_token` are encrypted at rest. The application decrypts on read using the `ENCRYPTION_KEY` env var.
- `token_expires_at` is checked by a periodic job to warn about expiring tokens.
- Soft-disable via `is_active = false` instead of deleting — preserves audit trail.

### contacts

Maps Instagram users (senders) to Chatwoot contacts. Created automatically on first message.

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK, DEFAULT gen_random_uuid() |
| account_id | uuid | NOT NULL, FK → accounts(id) ON DELETE CASCADE |
| ig_sender_id | varchar(64) | NOT NULL. Instagram-scoped ID of the person messaging |
| chatwoot_contact_id | integer | NOT NULL. Chatwoot contact numeric ID |
| chatwoot_contact_source_id | varchar(255) | NOT NULL. Source ID for Chatwoot contact_inbox |
| ig_username | varchar(255) | NULL. Fetched from IG profile API |
| ig_name | varchar(255) | NULL. Fetched from IG profile API |
| ig_profile_pic | text | NULL. URL to profile picture |
| created_at | timestamptz | NOT NULL, DEFAULT NOW() |
| updated_at | timestamptz | NOT NULL, DEFAULT NOW() |

Notes:
- UNIQUE constraint on (account_id, ig_sender_id) — one contact per sender per account.
- `ig_username`, `ig_name`, `ig_profile_pic` are fetched from the Instagram API on contact creation. May be NULL if the API call fails (non-blocking).
- `chatwoot_contact_source_id` is used when creating conversations in Chatwoot.

### conversations

Maps active conversations between a contact and an account. Used to track which Chatwoot conversation to send messages to, and to monitor the messaging window.

| Column | Type | Notes |
|---|---|---|
| id | uuid | PK, DEFAULT gen_random_uuid() |
| account_id | uuid | NOT NULL, FK → accounts(id) ON DELETE CASCADE |
| contact_id | uuid | NOT NULL, FK → contacts(id) ON DELETE CASCADE |
| chatwoot_conversation_id | integer | NOT NULL. Chatwoot conversation numeric ID |
| last_customer_message_at | timestamptz | NOT NULL. Last message FROM the Instagram user. Used to calculate 24h/7d window |
| is_active | boolean | NOT NULL, DEFAULT true |
| created_at | timestamptz | NOT NULL, DEFAULT NOW() |
| updated_at | timestamptz | NOT NULL, DEFAULT NOW() |

Notes:
- UNIQUE constraint on (account_id, contact_id, chatwoot_conversation_id).
- `last_customer_message_at` is updated every time a new message arrives from the Instagram user. The service checks this before sending replies to determine if the messaging window is still open.
- A conversation becomes `is_active = false` when the Chatwoot conversation is resolved/closed. A new conversation is created if the customer messages again.

## Relationships summary

```
  accounts (1) ──────< (N) contacts
     │                       │
     │                       │
     └──────< (N) conversations >──────┘
                    │
              1 contact has
              N conversations
              (over time)
```

- One account has many contacts (all the people who messaged that IG account)
- One account has many conversations
- One contact has many conversations (over time, as old ones close and new ones open)
- Each conversation belongs to exactly one account AND one contact

## Indexes

```sql
-- Fast webhook routing: find account by IG page ID
CREATE UNIQUE INDEX idx_accounts_ig_page_id ON accounts(ig_page_id);

-- Fast contact lookup on incoming message
CREATE UNIQUE INDEX idx_contacts_account_sender ON contacts(account_id, ig_sender_id);

-- Fast conversation lookup
CREATE INDEX idx_conversations_account_contact ON conversations(account_id, contact_id) WHERE is_active = true;

-- Find active conversations by Chatwoot IDs (for outbound message routing)
CREATE INDEX idx_conversations_chatwoot ON conversations(chatwoot_conversation_id) WHERE is_active = true;

-- Token expiration job
CREATE INDEX idx_accounts_token_expires ON accounts(token_expires_at) WHERE is_active = true AND token_expires_at IS NOT NULL;
```

## Field types reference

| Type | Usage | Notes |
|---|---|---|
| uuid | Primary keys | gen_random_uuid(), avoids sequential ID guessing |
| varchar(64) | Instagram IDs | IG IDs are numeric strings, 64 chars is generous |
| varchar(255) | Names, usernames | Standard max for names |
| varchar(512) | URLs | Base URLs for Chatwoot |
| text | Encrypted tokens, long URLs | No length limit, encrypted values vary in size |
| integer | Chatwoot numeric IDs | Chatwoot uses integer IDs |
| boolean | Flags | is_active |
| timestamptz | All timestamps | Always with timezone, stored as UTC |
