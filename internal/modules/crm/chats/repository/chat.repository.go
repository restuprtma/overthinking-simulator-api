package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"
	"lakukan-be/internal/modules/crm/chats/domain"
)

type ChatRepository struct {
	db *pgxpool.Pool
}

func NewChatRepository(db *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{db: db}
}

// FindByID retrieves a chat by ID and company ID
func (r *ChatRepository) FindByID(id, companyID string) (*domain.Chat, error) {
	query := `
		SELECT
			c.id, c.company_id, c.lead_id, c.customer_name, c.phone, c.email,
			c.assigned_to_company_user_id, c.platform, c.status, c.category,
			c.last_message_at, c.last_message_preview, c.last_message_sender,
			c.unread_count, c.message_count, c.ai_insights,
			c.avg_response_time_minutes, c.first_response_time_minutes, c.sentiment_score,
			c.tags, c.created_at, c.created_by, c.updated_at, c.updated_by, c.deleted_at, c.deleted_by,
			l.name as lead_name,
			u.id as assigned_to_user_id, u.full_name as assigned_to_user_name, u.email as assigned_to_user_email
		FROM crm.chats c
		LEFT JOIN crm.leads l ON c.lead_id = l.id
		LEFT JOIN core.company_users cu ON c.assigned_to_company_user_id = cu.id
		LEFT JOIN core.users u ON cu.user_id = u.id
		WHERE c.id = $1 AND c.company_id = $2 AND c.deleted_at IS NULL
	`

	var chat domain.Chat
	var tags pq.StringArray
	err := r.db.QueryRow(context.Background(), query, id, companyID).Scan(
		&chat.ID, &chat.CompanyID, &chat.LeadID, &chat.CustomerName, &chat.Phone, &chat.Email,
		&chat.AssignedToCompanyUserID, &chat.Platform, &chat.Status, &chat.Category,
		&chat.LastMessageAt, &chat.LastMessagePreview, &chat.LastMessageSender,
		&chat.UnreadCount, &chat.MessageCount, &chat.AIInsights,
		&chat.AvgResponseTimeMinutes, &chat.FirstResponseTimeMinutes, &chat.SentimentScore,
		&tags, &chat.CreatedAt, &chat.CreatedBy, &chat.UpdatedAt, &chat.UpdatedBy, &chat.DeletedAt, &chat.DeletedBy,
		&chat.LeadName,
		&chat.AssignedToUserID, &chat.AssignedToUserName, &chat.AssignedToUserEmail,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	chat.Tags = tags

	return &chat, nil
}

// FindByPhoneAndPlatform retrieves a chat by phone, platform, and company ID
func (r *ChatRepository) FindByPhoneAndPlatform(phone, platform, companyID string) (*domain.Chat, error) {
	query := `
		SELECT
			c.id, c.company_id, c.lead_id, c.customer_name, c.phone, c.email,
			c.assigned_to_company_user_id, c.platform, c.status, c.category,
			c.last_message_at, c.last_message_preview, c.last_message_sender,
			c.unread_count, c.message_count, c.ai_insights,
			c.avg_response_time_minutes, c.first_response_time_minutes, c.sentiment_score,
			c.tags, c.created_at, c.created_by, c.updated_at, c.updated_by, c.deleted_at, c.deleted_by,
			l.name as lead_name,
			u.id as assigned_to_user_id, u.full_name as assigned_to_user_name, u.email as assigned_to_user_email
		FROM crm.chats c
		LEFT JOIN crm.leads l ON c.lead_id = l.id
		LEFT JOIN core.company_users cu ON c.assigned_to_company_user_id = cu.id
		LEFT JOIN core.users u ON cu.user_id = u.id
		WHERE c.phone = $1 AND c.platform = $2 AND c.company_id = $3 AND c.deleted_at IS NULL
	`

	var chat domain.Chat
	var tags pq.StringArray
	err := r.db.QueryRow(context.Background(), query, phone, platform, companyID).Scan(
		&chat.ID, &chat.CompanyID, &chat.LeadID, &chat.CustomerName, &chat.Phone, &chat.Email,
		&chat.AssignedToCompanyUserID, &chat.Platform, &chat.Status, &chat.Category,
		&chat.LastMessageAt, &chat.LastMessagePreview, &chat.LastMessageSender,
		&chat.UnreadCount, &chat.MessageCount, &chat.AIInsights,
		&chat.AvgResponseTimeMinutes, &chat.FirstResponseTimeMinutes, &chat.SentimentScore,
		&tags, &chat.CreatedAt, &chat.CreatedBy, &chat.UpdatedAt, &chat.UpdatedBy, &chat.DeletedAt, &chat.DeletedBy,
		&chat.LeadName,
		&chat.AssignedToUserID, &chat.AssignedToUserName, &chat.AssignedToUserEmail,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	chat.Tags = tags

	return &chat, nil
}

// FindAll retrieves all chats with pagination and filters
func (r *ChatRepository) FindAll(companyID string, limit, offset int, search string, platform, status, category, assignedToCompanyUserID *string) ([]*domain.Chat, error) {
	query := `
		SELECT
			c.id, c.company_id, c.lead_id, c.customer_name, c.phone, c.email,
			c.assigned_to_company_user_id, c.platform, c.status, c.category,
			c.last_message_at, c.last_message_preview, c.last_message_sender,
			c.unread_count, c.message_count,
			c.avg_response_time_minutes, c.first_response_time_minutes, c.sentiment_score,
			c.tags, c.created_at, c.updated_at,
			l.name as lead_name,
			u.id as assigned_to_user_id, u.full_name as assigned_to_user_name, u.email as assigned_to_user_email
		FROM crm.chats c
		LEFT JOIN crm.leads l ON c.lead_id = l.id
		LEFT JOIN core.company_users cu ON c.assigned_to_company_user_id = cu.id
		LEFT JOIN core.users u ON cu.user_id = u.id
		WHERE c.company_id = $1 AND c.deleted_at IS NULL
	`

	args := []interface{}{companyID}
	argPos := 2

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (c.customer_name ILIKE $%d OR c.phone ILIKE $%d OR c.email ILIKE $%d)`, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add platform filter
	if platform != nil && *platform != "" {
		query += fmt.Sprintf(` AND c.platform = $%d`, argPos)
		args = append(args, *platform)
		argPos++
	}

	// Add status filter
	if status != nil && *status != "" {
		query += fmt.Sprintf(` AND c.status = $%d`, argPos)
		args = append(args, *status)
		argPos++
	}

	// Add category filter
	if category != nil && *category != "" {
		query += fmt.Sprintf(` AND c.category = $%d`, argPos)
		args = append(args, *category)
		argPos++
	}

	// Add assigned_to_company_user_id filter
	if assignedToCompanyUserID != nil && *assignedToCompanyUserID != "" {
		query += fmt.Sprintf(` AND c.assigned_to_company_user_id = $%d`, argPos)
		args = append(args, *assignedToCompanyUserID)
		argPos++
	}

	query += ` ORDER BY c.last_message_at DESC NULLS LAST, c.created_at DESC`
	query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argPos, argPos+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []*domain.Chat
	for rows.Next() {
		var chat domain.Chat
		var tags pq.StringArray
		err := rows.Scan(
			&chat.ID, &chat.CompanyID, &chat.LeadID, &chat.CustomerName, &chat.Phone, &chat.Email,
			&chat.AssignedToCompanyUserID, &chat.Platform, &chat.Status, &chat.Category,
			&chat.LastMessageAt, &chat.LastMessagePreview, &chat.LastMessageSender,
			&chat.UnreadCount, &chat.MessageCount,
			&chat.AvgResponseTimeMinutes, &chat.FirstResponseTimeMinutes, &chat.SentimentScore,
			&tags, &chat.CreatedAt, &chat.UpdatedAt,
			&chat.LeadName,
			&chat.AssignedToUserID, &chat.AssignedToUserName, &chat.AssignedToUserEmail,
		)
		if err != nil {
			return nil, err
		}
		chat.Tags = tags
		chats = append(chats, &chat)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return chats, nil
}

// Count returns the total count of chats with filters
func (r *ChatRepository) Count(companyID, search string, platform, status, category, assignedToCompanyUserID *string) (int, error) {
	query := `
		SELECT COUNT(DISTINCT c.id)
		FROM crm.chats c
		WHERE c.company_id = $1 AND c.deleted_at IS NULL
	`

	args := []interface{}{companyID}
	argPos := 2

	// Add search filter
	if search != "" {
		query += fmt.Sprintf(` AND (c.customer_name ILIKE $%d OR c.phone ILIKE $%d OR c.email ILIKE $%d)`, argPos, argPos, argPos)
		args = append(args, "%"+search+"%")
		argPos++
	}

	// Add platform filter
	if platform != nil && *platform != "" {
		query += fmt.Sprintf(` AND c.platform = $%d`, argPos)
		args = append(args, *platform)
		argPos++
	}

	// Add status filter
	if status != nil && *status != "" {
		query += fmt.Sprintf(` AND c.status = $%d`, argPos)
		args = append(args, *status)
		argPos++
	}

	// Add category filter
	if category != nil && *category != "" {
		query += fmt.Sprintf(` AND c.category = $%d`, argPos)
		args = append(args, *category)
		argPos++
	}

	// Add assigned_to_company_user_id filter
	if assignedToCompanyUserID != nil && *assignedToCompanyUserID != "" {
		query += fmt.Sprintf(` AND c.assigned_to_company_user_id = $%d`, argPos)
		args = append(args, *assignedToCompanyUserID)
		argPos++
	}

	var count int
	err := r.db.QueryRow(context.Background(), query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Create inserts a new chat
func (r *ChatRepository) Create(chat *domain.Chat) error {
	query := `
		INSERT INTO crm.chats (
			id, company_id, lead_id, customer_name, phone, email,
			assigned_to_company_user_id, platform, status, category,
			unread_count, message_count, tags,
			created_at, created_by, updated_at, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`

	_, err := r.db.Exec(context.Background(), query,
		chat.ID, chat.CompanyID, chat.LeadID, chat.CustomerName, chat.Phone, chat.Email,
		chat.AssignedToCompanyUserID, chat.Platform, chat.Status, chat.Category,
		chat.UnreadCount, chat.MessageCount, pq.Array(chat.Tags),
		chat.CreatedAt, chat.CreatedBy, chat.UpdatedAt, chat.UpdatedBy,
	)

	return err
}

// Update modifies an existing chat
func (r *ChatRepository) Update(chat *domain.Chat) error {
	query := `
		UPDATE crm.chats
		SET customer_name = $1, email = $2, assigned_to_company_user_id = $3,
		    status = $4, category = $5,
		    last_message_at = $6, last_message_preview = $7, last_message_sender = $8,
		    unread_count = $9, message_count = $10,
		    avg_response_time_minutes = $11, first_response_time_minutes = $12,
		    sentiment_score = $13, tags = $14, updated_at = $15, updated_by = $16
		WHERE id = $17 AND company_id = $18 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(context.Background(), query,
		chat.CustomerName, chat.Email, chat.AssignedToCompanyUserID,
		chat.Status, chat.Category,
		chat.LastMessageAt, chat.LastMessagePreview, chat.LastMessageSender,
		chat.UnreadCount, chat.MessageCount,
		chat.AvgResponseTimeMinutes, chat.FirstResponseTimeMinutes,
		chat.SentimentScore, pq.Array(chat.Tags), chat.UpdatedAt, chat.UpdatedBy,
		chat.ID, chat.CompanyID,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
