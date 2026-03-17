# Business Rules — instasae

## About this document

These rules are enforced on the server side regardless of who or what calls the API. They define what the system MUST do, not how the code is structured.

## Webhook Processing

**BR-WEBHOOK-01 — Immediate 200 response to Meta**
When the Instagram webhook POST arrives, the server MUST respond with HTTP 200 before any processing begins. Processing happens in a background goroutine. Meta retries webhooks that don't receive 200 within 20 seconds.

**BR-WEBHOOK-02 — Signature validation is mandatory**
Every POST to `/webhook/instagram` MUST have its `X-Hub-Signature-256` header validated against the `META_APP_SECRET` using HMAC-SHA256. Invalid signatures are logged and the request is discarded. No exceptions.

**BR-WEBHOOK-03 — Echo messages are always ignored**
If `message.is_echo` is `true`, the message was sent by the account owner (the client) via Instagram app or another tool. instasae MUST NOT forward echo messages to Chatwoot to avoid duplicate messages.

**BR-WEBHOOK-04 — Webhook deduplication via message ID**
Each message's `mid` (message ID) is stored in Redis with SET NX and TTL of 300 seconds. If the SET NX returns false (key already exists), the message is a retry and MUST be ignored.

**BR-WEBHOOK-05 — Unknown accounts are silently skipped**
If the `entry[].id` (recipient Instagram Page ID) does not match any active account in the database, the webhook is logged at WARN level and discarded. No error response to Meta.

**BR-WEBHOOK-06 — Only "instagram" object is processed**
If `payload.object` is not `"instagram"`, the webhook is ignored. This guards against accidentally receiving events from other Meta products if the app is misconfigured.

## Message Routing — Inbound (Instagram → Chatwoot)

**BR-INBOUND-01 — Auto-create contact on first message**
When a message arrives from an `ig_sender_id` that doesn't exist in the contacts table for that account, instasae MUST: (1) Fetch the sender's profile from Instagram API (name, username, profile_pic), (2) Create a contact in Chatwoot via API with the profile info, (3) Store the mapping in the local contacts table.

**BR-INBOUND-02 — Auto-create conversation on first message**
When a message arrives and there's no active conversation for the contact in that account, instasae MUST create a new conversation in Chatwoot via API and store the mapping locally.

**BR-INBOUND-03 — Reuse existing active conversation**
If an active conversation already exists for the contact, new messages are sent to that same Chatwoot conversation. A new conversation is only created when the previous one was marked inactive.

**BR-INBOUND-04 — Update last_customer_message_at on every inbound message**
Every incoming message from a customer updates `conversations.last_customer_message_at`. This timestamp is used to calculate whether the messaging window is still open.

**BR-INBOUND-05 — Profile fetch failure is non-blocking**
If the Instagram profile API call fails (rate limit, permission issue), the contact is still created with `ig_username` and `ig_name` as NULL. The Chatwoot contact gets a placeholder name like "IG User {ig_sender_id}".

## Message Routing — Outbound (Chatwoot → Instagram)

**BR-OUTBOUND-01 — Only outgoing non-private messages are processed**
The Chatwoot callback fires for all message types. instasae MUST only process messages where: `event == "message_created"` AND `message_type == "outgoing"` (or numeric value 1) AND `private == false`. Incoming messages, private notes, and activity messages are all ignored.

**BR-OUTBOUND-02 — Composite messages are split into sequential API calls**
Instagram API does not support sending text and attachment in a single request. If the Chatwoot message contains both text content AND an attachment, instasae MUST send two sequential requests to the Graph API: (1) attachment first, (2) text second. Both must use the same recipient.

**BR-OUTBOUND-03 — HUMAN_AGENT tag on all replies**
All outgoing messages sent via Graph API include the `tag: "HUMAN_AGENT"` parameter. This extends the messaging window from 24 hours to 7 days. The Meta App must have the `human_agent` permission approved.

**BR-OUTBOUND-04 — Messaging window check before sending**
Before sending a message via Graph API, instasae checks `last_customer_message_at`. If the timestamp is older than 7 days, the message is NOT sent and a private note is posted in the Chatwoot conversation: "Message not sent: messaging window expired (last customer message was X days ago)."

**BR-OUTBOUND-05 — Private note on send failure**
If the Graph API returns an error (token expired, rate limit, invalid recipient, window expired), instasae posts a private note in the Chatwoot conversation with the error details. The note format: "⚠️ Message not delivered to Instagram: {error_message} (code: {error_code})".

**BR-OUTBOUND-06 — Partial failure on composite messages**
If a composite message (text + attachment) has the first call succeed and the second fail, instasae logs the partial failure and posts a private note: "⚠️ Attachment sent but text failed: {error_message}".

## Media Handling

**BR-MEDIA-01 — Inbound media is persisted to B2**
When an incoming Instagram message contains an attachment (image, audio, video), instasae MUST: (1) Download the file from the Meta CDN URL, (2) Upload to Backblaze B2 under the `instasae/{account_id}/{date}/` prefix, (3) Use the public B2 URL when creating the message in Chatwoot.

**BR-MEDIA-02 — Media type validation**
Accepted media types: image (png, jpeg, gif, webp), audio (aac, m4a, wav, mp4), video (mp4, ogg, avi, mov, webm). If the content-type is not recognized, the media is logged as unsupported but the text portion (if any) is still forwarded.

**BR-MEDIA-03 — Media size limit**
Maximum media size for download: 25MB (Instagram's own limit). If download exceeds this, skip the media and forward any text content with a note.

**BR-MEDIA-04 — Outbound media URLs are passed through**
When the Chatwoot agent sends an attachment, the URL in the Chatwoot callback is already a permanent URL (Chatwoot/B2 hosted). This URL is sent directly to the Graph API in the `payload.url` field. No re-download or re-upload needed.

## Account Management

**BR-ACCOUNT-01 — Tokens are encrypted at rest**
`ig_access_token` and `chatwoot_api_token` are encrypted using AES-256-GCM before storage and decrypted on read. The encryption key is never stored in the database.

**BR-ACCOUNT-02 — Duplicate ig_page_id is rejected**
Creating an account with an `ig_page_id` that already exists returns HTTP 409 Conflict.

**BR-ACCOUNT-03 — Soft delete preserves data**
DELETE on an account sets `is_active = false`. Contacts and conversations are preserved. A hard delete is not exposed via API.

**BR-ACCOUNT-04 — Token expiration warning**
A background job runs every 6 hours and checks all active accounts. If `token_expires_at` is within 7 days, a WARNING log is emitted with the account ID and page name.

## Chatwoot Integration

**BR-CHATWOOT-01 — Contact creation uses Chatwoot Application API**
Contacts are created via `POST /api/v1/accounts/{id}/contacts` with `inbox_id` parameter. This automatically creates a `contact_inbox` with a `source_id`.

**BR-CHATWOOT-02 — Conversation creation uses source_id**
Conversations are created via `POST /api/v1/accounts/{id}/conversations` with `source_id` from the contact_inbox, `inbox_id`, and `contact_id`.

**BR-CHATWOOT-03 — Messages use Application API**
Messages are sent via `POST /api/v1/accounts/{id}/conversations/{conv_id}/messages` with `message_type: "incoming"` for customer messages.

**BR-CHATWOOT-04 — Callback URL is the instasae webhook**
Each Chatwoot API inbox has its callback URL set to `https://${INSTASAE_DOMAIN}/webhook/chatwoot`. This URL receives all message events from that inbox.
