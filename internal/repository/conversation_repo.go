package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/italomoia/instasae/internal/domain"
	"github.com/italomoia/instasae/internal/model"
)

var _ domain.ConversationRepository = (*ConversationRepo)(nil)

type ConversationRepo struct {
	pool *pgxpool.Pool
}

func NewConversationRepo(pool *pgxpool.Pool) *ConversationRepo {
	return &ConversationRepo{pool: pool}
}

const conversationColumns = `id, account_id, contact_id, chatwoot_conversation_id,
	last_customer_message_at, is_active, created_at, updated_at`

func scanConversation(row pgx.Row) (*model.Conversation, error) {
	var c model.Conversation
	err := row.Scan(
		&c.ID, &c.AccountID, &c.ContactID, &c.ChatwootConversationID,
		&c.LastCustomerMessageAt, &c.IsActive, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ConversationRepo) Create(ctx context.Context, conv *model.Conversation) (*model.Conversation, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO conversations (account_id, contact_id, chatwoot_conversation_id, last_customer_message_at)
		VALUES ($1, $2, $3, $4)
		RETURNING `+conversationColumns,
		conv.AccountID, conv.ContactID, conv.ChatwootConversationID, conv.LastCustomerMessageAt,
	)

	created, err := scanConversation(row)
	if err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return created, nil
}

func (r *ConversationRepo) GetActiveByContact(ctx context.Context, accountID uuid.UUID, contactID uuid.UUID) (*model.Conversation, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+conversationColumns+` FROM conversations
		WHERE account_id = $1 AND contact_id = $2 AND is_active = true`,
		accountID, contactID,
	)

	c, err := scanConversation(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active conversation by contact: %w", err)
	}
	return c, nil
}

func (r *ConversationRepo) GetByChatwootID(ctx context.Context, chatwootConvID int) (*model.Conversation, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+conversationColumns+` FROM conversations
		WHERE chatwoot_conversation_id = $1 AND is_active = true`,
		chatwootConvID,
	)

	c, err := scanConversation(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get conversation by chatwoot id: %w", err)
	}
	return c, nil
}

func (r *ConversationRepo) UpdateLastCustomerMessage(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE conversations SET last_customer_message_at = $2, updated_at = NOW() WHERE id = $1`,
		id, at,
	)
	if err != nil {
		return fmt.Errorf("update last customer message: %w", err)
	}
	return nil
}

func (r *ConversationRepo) Deactivate(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE conversations SET is_active = false, updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("deactivate conversation: %w", err)
	}
	return nil
}
