package repository

import (
	"errors"
	"fmt"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
)

type ChatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) Create(message *models.ChatMessage) error {
	if err := r.db.Create(message).Error; err != nil {
		return fmt.Errorf("create chat message: %w", err)
	}
	return nil
}

func (r *ChatRepository) ListByQuestionID(questionID int64) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	if err := r.db.Where("question_id = ?", questionID).Order("id asc").Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	return messages, nil
}

func (r *ChatRepository) ListByQuestionIDForUser(questionID, userID int64) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	if err := r.db.Where("question_id = ? AND user_id = ?", questionID, userID).Order("id asc").Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list chat messages by owner: %w", err)
	}
	return messages, nil
}

func (r *ChatRepository) GetByIdempotencyKey(questionID int64, key string) (*models.ChatMessage, error) {
	var message models.ChatMessage
	if err := r.db.Where("question_id = ? AND idempotency_key = ?", questionID, key).First(&message).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get chat message by idempotency key: %w", err)
	}
	return &message, nil
}
