package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"venturo-skeleton-go/internal/modules/crm/chats/domain"
)

type ChatMessageRepository struct {
	db *pgxpool.Pool
}

func NewChatMessageRepository(db *pgxpool.Pool) *ChatMessageRepository {
	return &ChatMessageRepository{db: db}
}

// FindByChatID retrieves all messages for a specific chat
func (r *ChatMessageRepository) FindByChatID(chatID string) ([]*domain.ChatMessage, error) {
	query := `
		SELECT
			id, chat_id, sender_type, sender_id, sender_name,
			content, message_type, media_url, media_mime_type, media_size_bytes,
			is_read, read_at, is_sent, sent_at, delivery_status, metadata, created_at
		FROM crm.chat_messages
		WHERE chat_id = $1
		ORDER BY sent_at ASC, created_at ASC
	`

	rows, err := r.db.Query(context.Background(), query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*domain.ChatMessage
	for rows.Next() {
		var msg domain.ChatMessage
		err := rows.Scan(
			&msg.ID, &msg.ChatID, &msg.SenderType, &msg.SenderID, &msg.SenderName,
			&msg.Content, &msg.MessageType, &msg.MediaURL, &msg.MediaMimeType, &msg.MediaSizeBytes,
			&msg.IsRead, &msg.ReadAt, &msg.IsSent, &msg.SentAt, &msg.DeliveryStatus, &msg.Metadata, &msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

// Create inserts a new chat message
func (r *ChatMessageRepository) Create(message *domain.ChatMessage) error {
	query := `
		INSERT INTO crm.chat_messages (
			id, chat_id, sender_type, sender_id, sender_name,
			content, message_type, media_url, media_mime_type, media_size_bytes,
			is_read, read_at, is_sent, sent_at, delivery_status, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`

	_, err := r.db.Exec(context.Background(), query,
		message.ID, message.ChatID, message.SenderType, message.SenderID, message.SenderName,
		message.Content, message.MessageType, message.MediaURL, message.MediaMimeType, message.MediaSizeBytes,
		message.IsRead, message.ReadAt, message.IsSent, message.SentAt, message.DeliveryStatus, message.Metadata, message.CreatedAt,
	)

	return err
}

// CountByChatID returns the total count of messages for a chat
func (r *ChatMessageRepository) CountByChatID(chatID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM crm.chat_messages
		WHERE chat_id = $1
	`

	var count int
	err := r.db.QueryRow(context.Background(), query, chatID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Update updates a chat message
func (r *ChatMessageRepository) Update(message *domain.ChatMessage) error {
	query := `
		UPDATE crm.chat_messages
		SET is_read = $1,
		    read_at = $2,
		    delivery_status = $3
		WHERE id = $4
	`

	result, err := r.db.Exec(context.Background(), query,
		message.IsRead, message.ReadAt, message.DeliveryStatus, message.ID,
	)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// FindByWAHAMessageID finds a message by WAHA message ID from metadata
func (r *ChatMessageRepository) FindByWAHAMessageID(wahaMessageID string) (*domain.ChatMessage, error) {
	query := `
		SELECT
			id, chat_id, sender_type, sender_id, sender_name,
			content, message_type, media_url, media_mime_type, media_size_bytes,
			is_read, read_at, is_sent, sent_at, delivery_status, metadata, created_at
		FROM crm.chat_messages
		WHERE metadata->>'waha_message_id' = $1
		LIMIT 1
	`

	var msg domain.ChatMessage
	err := r.db.QueryRow(context.Background(), query, wahaMessageID).Scan(
		&msg.ID, &msg.ChatID, &msg.SenderType, &msg.SenderID, &msg.SenderName,
		&msg.Content, &msg.MessageType, &msg.MediaURL, &msg.MediaMimeType, &msg.MediaSizeBytes,
		&msg.IsRead, &msg.ReadAt, &msg.IsSent, &msg.SentAt, &msg.DeliveryStatus, &msg.Metadata, &msg.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &msg, nil
}
