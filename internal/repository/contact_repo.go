package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/italomoia/instasae/internal/domain"
	"github.com/italomoia/instasae/internal/model"
)

var _ domain.ContactRepository = (*ContactRepo)(nil)

type ContactRepo struct {
	pool *pgxpool.Pool
}

func NewContactRepo(pool *pgxpool.Pool) *ContactRepo {
	return &ContactRepo{pool: pool}
}

const contactColumns = `id, account_id, ig_sender_id, chatwoot_contact_id, chatwoot_contact_source_id,
	ig_username, ig_name, ig_profile_pic, created_at, updated_at`

func scanContact(row pgx.Row) (*model.Contact, error) {
	var c model.Contact
	err := row.Scan(
		&c.ID, &c.AccountID, &c.IGSenderID, &c.ChatwootContactID, &c.ChatwootContactSourceID,
		&c.IGUsername, &c.IGName, &c.IGProfilePic, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ContactRepo) Create(ctx context.Context, contact *model.Contact) (*model.Contact, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO contacts (account_id, ig_sender_id, chatwoot_contact_id, chatwoot_contact_source_id,
			ig_username, ig_name, ig_profile_pic)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+contactColumns,
		contact.AccountID, contact.IGSenderID, contact.ChatwootContactID, contact.ChatwootContactSourceID,
		contact.IGUsername, contact.IGName, contact.IGProfilePic,
	)

	created, err := scanContact(row)
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}
	return created, nil
}

func (r *ContactRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Contact, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+contactColumns+` FROM contacts WHERE id = $1`,
		id,
	)

	c, err := scanContact(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get contact by id: %w", err)
	}
	return c, nil
}

func (r *ContactRepo) GetByAccountAndSender(ctx context.Context, accountID uuid.UUID, igSenderID string) (*model.Contact, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+contactColumns+` FROM contacts WHERE account_id = $1 AND ig_sender_id = $2`,
		accountID, igSenderID,
	)

	c, err := scanContact(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get contact by account and sender: %w", err)
	}
	return c, nil
}
