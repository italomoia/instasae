package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/italomoia/instasae/internal/domain"
	"github.com/italomoia/instasae/internal/model"
)

var _ domain.AccountRepository = (*AccountRepo)(nil)

type AccountRepo struct {
	pool *pgxpool.Pool
	enc  domain.Encryptor
}

func NewAccountRepo(pool *pgxpool.Pool, enc domain.Encryptor) *AccountRepo {
	return &AccountRepo{pool: pool, enc: enc}
}

const accountColumns = `id, ig_page_id, ig_page_name, ig_access_token, chatwoot_base_url,
	chatwoot_account_id, chatwoot_inbox_id, chatwoot_api_token, webhook_verify_token,
	token_expires_at, is_active, created_at, updated_at`

func (r *AccountRepo) scanAccount(row pgx.Row) (*model.Account, error) {
	var a model.Account
	var encToken, encCWToken string

	err := row.Scan(
		&a.ID, &a.IGPageID, &a.IGPageName, &encToken, &a.ChatwootBaseURL,
		&a.ChatwootAccountID, &a.ChatwootInboxID, &encCWToken, &a.WebhookVerifyToken,
		&a.TokenExpiresAt, &a.IsActive, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	plainToken, err := r.enc.Decrypt(encToken)
	if err != nil {
		return nil, fmt.Errorf("decrypting ig_access_token: %w", err)
	}
	a.IGAccessToken = plainToken

	plainCWToken, err := r.enc.Decrypt(encCWToken)
	if err != nil {
		return nil, fmt.Errorf("decrypting chatwoot_api_token: %w", err)
	}
	a.ChatwootAPIToken = plainCWToken

	return &a, nil
}

func (r *AccountRepo) Create(ctx context.Context, params model.CreateAccountParams) (*model.Account, error) {
	encToken, err := r.enc.Encrypt(params.IGAccessToken)
	if err != nil {
		return nil, fmt.Errorf("encrypting ig_access_token: %w", err)
	}

	encCWToken, err := r.enc.Encrypt(params.ChatwootAPIToken)
	if err != nil {
		return nil, fmt.Errorf("encrypting chatwoot_api_token: %w", err)
	}

	row := r.pool.QueryRow(ctx,
		`INSERT INTO accounts (ig_page_id, ig_page_name, ig_access_token, chatwoot_base_url,
			chatwoot_account_id, chatwoot_inbox_id, chatwoot_api_token, webhook_verify_token,
			token_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+accountColumns,
		params.IGPageID, params.IGPageName, encToken, params.ChatwootBaseURL,
		params.ChatwootAccountID, params.ChatwootInboxID, encCWToken, params.WebhookVerifyToken,
		params.TokenExpiresAt,
	)

	return r.scanAccount(row)
}

func (r *AccountRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+accountColumns+` FROM accounts WHERE id = $1`, id)

	acc, err := r.scanAccount(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get account by id: %w", err)
	}
	return acc, nil
}

func (r *AccountRepo) GetByIGPageID(ctx context.Context, igPageID string) (*model.Account, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+accountColumns+` FROM accounts WHERE ig_page_id = $1 AND is_active = true`, igPageID)

	acc, err := r.scanAccount(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get account by ig_page_id: %w", err)
	}
	return acc, nil
}

func (r *AccountRepo) List(ctx context.Context) ([]model.Account, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+accountColumns+` FROM accounts ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []model.Account
	for rows.Next() {
		acc, err := r.scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning account: %w", err)
		}
		accounts = append(accounts, *acc)
	}
	return accounts, rows.Err()
}

func (r *AccountRepo) Update(ctx context.Context, id uuid.UUID, params model.UpdateAccountParams) (*model.Account, error) {
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	addClause := func(col string, val any) {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}

	if params.IGPageID != nil {
		addClause("ig_page_id", *params.IGPageID)
	}
	if params.IGPageName != nil {
		addClause("ig_page_name", *params.IGPageName)
	}
	if params.IGAccessToken != nil {
		enc, err := r.enc.Encrypt(*params.IGAccessToken)
		if err != nil {
			return nil, fmt.Errorf("encrypting ig_access_token: %w", err)
		}
		addClause("ig_access_token", enc)
	}
	if params.ChatwootBaseURL != nil {
		addClause("chatwoot_base_url", *params.ChatwootBaseURL)
	}
	if params.ChatwootAccountID != nil {
		addClause("chatwoot_account_id", *params.ChatwootAccountID)
	}
	if params.ChatwootInboxID != nil {
		addClause("chatwoot_inbox_id", *params.ChatwootInboxID)
	}
	if params.ChatwootAPIToken != nil {
		enc, err := r.enc.Encrypt(*params.ChatwootAPIToken)
		if err != nil {
			return nil, fmt.Errorf("encrypting chatwoot_api_token: %w", err)
		}
		addClause("chatwoot_api_token", enc)
	}
	if params.WebhookVerifyToken != nil {
		addClause("webhook_verify_token", *params.WebhookVerifyToken)
	}
	if params.TokenExpiresAt != nil {
		addClause("token_expires_at", *params.TokenExpiresAt)
	}
	if params.IsActive != nil {
		addClause("is_active", *params.IsActive)
	}

	if len(setClauses) == 0 {
		return r.GetByID(ctx, id)
	}

	addClause("updated_at", time.Now())

	query := fmt.Sprintf(
		`UPDATE accounts SET %s WHERE id = $%d RETURNING %s`,
		strings.Join(setClauses, ", "), argIdx, accountColumns,
	)
	args = append(args, id)

	row := r.pool.QueryRow(ctx, query, args...)
	acc, err := r.scanAccount(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("update account: %w", err)
	}
	return acc, nil
}

func (r *AccountRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE accounts SET is_active = false, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("soft delete account: %w", err)
	}
	return nil
}

func (r *AccountRepo) ListActiveWithExpiringTokens(ctx context.Context, within time.Duration) ([]model.Account, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+accountColumns+` FROM accounts
		WHERE is_active = true
			AND token_expires_at IS NOT NULL
			AND token_expires_at < NOW() + $1::interval
		ORDER BY token_expires_at`, within.String())
	if err != nil {
		return nil, fmt.Errorf("list expiring tokens: %w", err)
	}
	defer rows.Close()

	var accounts []model.Account
	for rows.Next() {
		acc, err := r.scanAccount(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning account: %w", err)
		}
		accounts = append(accounts, *acc)
	}
	return accounts, rows.Err()
}
