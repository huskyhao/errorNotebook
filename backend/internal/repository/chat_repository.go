package repository

import (
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
