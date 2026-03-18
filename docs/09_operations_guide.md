# Operations Guide — instasae

## About this document

Step-by-step instructions for managing instasae accounts in production.

## Adding a new client account

### Step 1 — Create Chatwoot inbox

1. Log in to Chatwoot (https://sae.imsdigitais.com)
2. Go to Settings > Inboxes > Add Inbox
3. Choose "API" channel
4. Name: "Instagram DM - {client name}"
5. Webhook URL: `https://instasae.imsdigitais.com/webhook/chatwoot`
6. Click "Create API channel"
7. Add agents who will handle this inbox
8. Note down: **Account ID**, **Inbox ID**, **API access token**

### Step 2 — Get Instagram credentials

1. In Meta Developer Dashboard, go to the Instagram product page
2. Click "Add account" under "Generate access tokens"
3. Log in with the client's Instagram professional account
4. Note down: **Access Token** and **Instagram Page ID**

### Step 3 — Create account in instasae

```bash
curl -s -X POST https://instasae.imsdigitais.com/api/accounts \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_ADMIN_API_KEY" \
  -d '{
    "ig_page_id": "INSTAGRAM_PAGE_ID",
    "ig_page_name": "Client Name",
    "ig_access_token": "INSTAGRAM_ACCESS_TOKEN",
    "chatwoot_base_url": "https://sae.imsdigitais.com",
    "chatwoot_account_id": CHATWOOT_ACCOUNT_ID,
    "chatwoot_inbox_id": CHATWOOT_INBOX_ID,
    "chatwoot_api_token": "CHATWOOT_API_TOKEN",
    "webhook_verify_token": "instasaet0k3nv3r1f1c4t1on"
  }'
```

### Step 4 — Subscribe page to webhooks

```bash
curl -X POST "https://graph.instagram.com/v25.0/INSTAGRAM_PAGE_ID/subscribed_apps?subscribed_fields=messages&access_token=INSTAGRAM_ACCESS_TOKEN"
```

Must return `{"success": true}`.

### Step 5 — Test

Send a DM from another Instagram account to the client's account. It should appear in the Chatwoot inbox within seconds.

## Updating an account

### Update any field (tokens, inbox, page)

```bash
curl -s -X PUT https://instasae.imsdigitais.com/api/accounts/ACCOUNT_UUID \
  -H "Content-Type: application/json" \
  -H "X-API-Key: YOUR_ADMIN_API_KEY" \
  -d '{"ig_access_token": "NEW_TOKEN"}'
```

Only include the fields you want to change. Other fields remain unchanged.

### Updatable fields

- `ig_page_id`, `ig_page_name`
- `ig_access_token`
- `chatwoot_base_url`, `chatwoot_account_id`, `chatwoot_inbox_id`, `chatwoot_api_token`
- `webhook_verify_token`
- `token_expires_at`
- `is_active`

## Removing an account (soft delete)

```bash
curl -s -X DELETE https://instasae.imsdigitais.com/api/accounts/ACCOUNT_UUID \
  -H "X-API-Key: YOUR_ADMIN_API_KEY"
```

This sets `is_active=false`. The account stops receiving webhooks but data is preserved.

## Checking account status

```bash
# List all accounts
curl -s https://instasae.imsdigitais.com/api/accounts \
  -H "X-API-Key: YOUR_ADMIN_API_KEY" | jq .

# Health check
curl -s https://instasae.imsdigitais.com/health | jq .
```

## Monitoring

### View logs

```bash
docker service logs -f instasae_instasae          # follow
docker service logs --tail=100 instasae_instasae   # last 100
```

### Common log patterns

| Log message | Meaning |
|---|---|
| `unknown account, skipping` | Webhook for a page not in the database |
| `ignoring echo message` | Normal — account owner's own messages |
| `profile fetch failed` | Instagram API rate limit, contact created with placeholder name |
| `failed to send message to Chatwoot` | Check Chatwoot API token validity |
| `messaging window expired` | Customer hasn't messaged in >7 days (or >24h without HUMAN_AGENT) |
| `media download/upload failed` | B2 or Instagram CDN issue |
| `failed to send attachment to Chatwoot, falling back to URL` | Chatwoot multipart upload failed, URL sent as text |

### Re-subscribe webhooks (if messages stop arriving)

```bash
curl -X POST "https://graph.instagram.com/v25.0/PAGE_ID/subscribed_apps?subscribed_fields=messages&access_token=ACCESS_TOKEN"
```

## Updating the service

```bash
# On dev machine:
cd /mnt/dados/projetos/instasae
docker build -t ghcr.io/italomoia/instasae:latest .
docker push ghcr.io/italomoia/instasae:latest

# On VPS:
docker pull ghcr.io/italomoia/instasae:latest
docker service update --image ghcr.io/italomoia/instasae:latest instasae_instasae --force
```

## Token renewal (every 60 days)

Instagram long-lived tokens expire after 60 days. Before expiry:

1. Generate a new token in Meta Developer Dashboard
2. Update the account:
   ```bash
   curl -s -X PUT https://instasae.imsdigitais.com/api/accounts/ACCOUNT_UUID \
     -H "Content-Type: application/json" \
     -H "X-API-Key: YOUR_ADMIN_API_KEY" \
     -d '{"ig_access_token": "NEW_TOKEN"}'
   ```
3. Re-subscribe the page to webhooks with the new token:
   ```bash
   curl -X POST "https://graph.instagram.com/v25.0/PAGE_ID/subscribed_apps?subscribed_fields=messages&access_token=NEW_TOKEN"
   ```

The token checker job logs warnings when tokens are within 7 days of expiry.

## Troubleshooting

### Messages not arriving in Chatwoot
1. Check health: `curl https://instasae.imsdigitais.com/health`
2. Check logs for errors: `docker service logs --tail=50 instasae_instasae`
3. Verify webhook subscription: re-subscribe the page (see above)
4. Verify account is active: list accounts and check `is_active`

### Replies not reaching Instagram
1. Check logs for `messaging window expired` — customer may need to message first
2. Check logs for `HUMAN_AGENT` errors — tag requires Meta approval
3. Verify Instagram access token hasn't expired (60-day limit)

### Media not showing inline in Chatwoot
1. Check logs for `media download/upload failed` — B2 connectivity issue
2. Check logs for `failed to send attachment to Chatwoot` — falls back to URL text
3. Verify B2 credentials are correct in environment variables
