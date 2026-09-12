package repository

import (
	"fmt"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
)

type PracticeSessionRepository struct {
	db *gorm.DB
}

func NewPracticeSessionRepository(db *gorm.DB) *PracticeSessionRepository {
	return &PracticeSessionRepository{db: db}
}

func (r *PracticeSessionRepository) Create(session *models.PracticeSession) error {
	if err := r.db.Create(session).Error; err != nil {
		return fmt.Errorf("create practice session: %w", err)
	}
	return nil
}

func (r *PracticeSessionRepository) CreateQuestions(items []models.PracticeSessionQuestion) error {
	if len(items) == 0 {
		return nil
	}
	if err := r.db.Create(&items).Error; err != nil {
		return fmt.Errorf("create practice session questions: %w", err)
	}
	return nil
}

func (r *PracticeSessionRepository) GetByID(id int64) (*models.PracticeSession, error) {
	var session models.PracticeSession
	if err := r.db.
		Preload("Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order("order_index asc")
		}).
		Preload("Questions.Question.Options").
		First(&session, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get practice session: %w", err)
	}
	return &session, nil
}

func (r *PracticeSessionRepository) List() ([]models.PracticeSession, error) {
	var sessions []models.PracticeSession
	if err := r.db.
		Order("created_at desc").
		Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("list practice sessions: %w", err)
	}
	return sessions, nil
}

func (r *PracticeSessionRepository) Update(session *models.PracticeSession) error {
	if err := r.db.Save(session).Error; err != nil {
		return fmt.Errorf("update practice session: %w", err)
	}
	return nil
}

func (r *PracticeSessionRepository) UpdateQuestion(item *models.PracticeSessionQuestion) error {
	if err := r.db.Save(item).Error; err != nil {
		return fmt.Errorf("update practice session question: %w", err)
	}
	return nil
}

func (r *PracticeSessionRepository) FindQuestion(sessionID int64, orderIndex int) (*models.PracticeSessionQuestion, error) {
	var item models.PracticeSessionQuestion
	if err := r.db.
		Where("session_id = ? AND order_index = ?", sessionID, orderIndex).
		First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("find practice session question: %w", err)
	}
	return &item, nil
}

func (r *PracticeSessionRepository) ListQuestionsBySession(sessionID int64) ([]models.PracticeSessionQuestion, error) {
	var items []models.PracticeSessionQuestion
	if err := r.db.
		Where("session_id = ?", sessionID).
		Order("order_index asc").
		Preload("Question.Options").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list practice session questions: %w", err)
	}
	return items, nil
}

func (r *PracticeSessionRepository) GetQuestionByID(questionID int64) (*models.Question, error) {
	var question models.Question
	if err := r.db.
		Preload("Options").
		First(&question, questionID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get question: %w", err)
	}
	return &question, nil
}
