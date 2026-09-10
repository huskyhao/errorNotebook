package repository

import (
	"errors"
	"fmt"
	"time"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
)

type LearningStateRepository struct {
	db *gorm.DB
}

func NewLearningStateRepository(db *gorm.DB) *LearningStateRepository {
	return &LearningStateRepository{db: db}
}

func (r *LearningStateRepository) GetByQuestionID(questionID int64) (*models.QuestionLearningState, error) {
	var state models.QuestionLearningState
	if err := r.db.Where("question_id = ?", questionID).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get learning state: %w", err)
	}
	return &state, nil
}

func (r *LearningStateRepository) Ensure(questionID int64) (*models.QuestionLearningState, error) {
	return r.EnsureForUser(questionID, 0)
}

func (r *LearningStateRepository) EnsureForUser(questionID, userID int64) (*models.QuestionLearningState, error) {
	var state *models.QuestionLearningState
	var err error
	if userID > 0 {
		state, err = r.GetByQuestionIDForUser(questionID, userID)
	} else {
		state, err = r.GetByQuestionID(questionID)
	}
	if err != nil {
		return nil, err
	}
	if state != nil {
		return state, nil
	}
	state = &models.QuestionLearningState{
		UserID:       userID,
		QuestionID:   questionID,
		MasteryLevel: 0,
		WeaknessTags: "[]",
		ReviewAdvice: "[]",
	}
	if err := r.db.Create(state).Error; err != nil {
		return nil, fmt.Errorf("create learning state: %w", err)
	}
	return state, nil
}

func (r *LearningStateRepository) GetByQuestionIDForUser(questionID, userID int64) (*models.QuestionLearningState, error) {
	var state models.QuestionLearningState
	if err := r.db.Where("question_id = ? AND user_id = ?", questionID, userID).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get learning state by owner: %w", err)
	}
	return &state, nil
}

func (r *LearningStateRepository) Save(state *models.QuestionLearningState) error {
	if err := r.db.Save(state).Error; err != nil {
		return fmt.Errorf("save learning state: %w", err)
	}
	return nil
}

func (r *LearningStateRepository) DeleteByQuestionID(questionID int64) error {
	if err := r.db.Where("question_id = ?", questionID).Delete(&models.QuestionLearningState{}).Error; err != nil {
		return fmt.Errorf("delete learning state: %w", err)
	}
	return nil
}

func (r *LearningStateRepository) DueForReview(now time.Time, limit int) ([]models.QuestionLearningState, error) {
	var states []models.QuestionLearningState
	if err := r.db.
		Preload("Question.Options").
		Where("next_review_at IS NOT NULL AND next_review_at <= ?", now).
		Order("next_review_at asc").
		Limit(limit).
		Find(&states).Error; err != nil {
		return nil, fmt.Errorf("list due learning states: %w", err)
	}
	return states, nil
}

func (r *LearningStateRepository) RecentWrong(limit int) ([]models.QuestionLearningState, error) {
	var states []models.QuestionLearningState
	if err := r.db.
		Preload("Question.Options").
		Where("wrong_count > 0").
		Order("COALESCE(last_practiced_at, updated_at) desc").
		Limit(limit).
		Find(&states).Error; err != nil {
		return nil, fmt.Errorf("list recent wrong states: %w", err)
	}
	return states, nil
}

func (r *LearningStateRepository) WithWeaknessTags(limit int) ([]models.QuestionLearningState, error) {
	var states []models.QuestionLearningState
	if err := r.db.
		Preload("Question.Options").
		Where("weakness_tags IS NOT NULL AND weakness_tags <> '' AND weakness_tags <> '[]'").
		Order("wrong_count desc, updated_at desc").
		Limit(limit).
		Find(&states).Error; err != nil {
		return nil, fmt.Errorf("list weak point states: %w", err)
	}
	return states, nil
}

func (r *LearningStateRepository) NewQuestions(limit int) ([]models.Question, error) {
	var questions []models.Question
	if err := r.db.
		Model(&models.Question{}).
		Preload("Options").
		Joins("LEFT JOIN question_learning_states ON question_learning_states.question_id = questions.id").
		Where("question_learning_states.id IS NULL OR question_learning_states.last_practiced_at IS NULL").
		Order("questions.id desc").
		Limit(limit).
		Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("list new questions: %w", err)
	}
	return questions, nil
}

func (r *LearningStateRepository) MixedQuestions(limit int) ([]models.Question, error) {
	var questions []models.Question
	if err := r.db.
		Preload("Options").
		Order("RAND()").
		Limit(limit).
		Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("list mixed questions: %w", err)
	}
	return questions, nil
}

func (r *LearningStateRepository) DueForReviewForUser(userID int64, now time.Time, limit int) ([]models.QuestionLearningState, error) {
	return r.listStatesForUser(userID, "next_review_at IS NOT NULL AND next_review_at <= ?", []any{now}, "next_review_at asc", limit)
}
func (r *LearningStateRepository) RecentWrongForUser(userID int64, limit int) ([]models.QuestionLearningState, error) {
	return r.listStatesForUser(userID, "wrong_count > 0", nil, "COALESCE(last_practiced_at, updated_at) desc", limit)
}
func (r *LearningStateRepository) WithWeaknessTagsForUser(userID int64, limit int) ([]models.QuestionLearningState, error) {
	return r.listStatesForUser(userID, "weakness_tags IS NOT NULL AND weakness_tags <> '' AND weakness_tags <> '[]'", nil, "wrong_count desc, updated_at desc", limit)
}
func (r *LearningStateRepository) NewQuestionsForUser(userID int64, limit int) ([]models.Question, error) {
	var questions []models.Question
	err := r.db.Model(&models.Question{}).Preload("Options").Where("questions.user_id = ?", userID).
		Joins("LEFT JOIN question_learning_states ON question_learning_states.question_id = questions.id AND question_learning_states.user_id = ?", userID).
		Where("question_learning_states.id IS NULL OR question_learning_states.last_practiced_at IS NULL").Order("questions.id desc").Limit(limit).Find(&questions).Error
	if err != nil {
		return nil, fmt.Errorf("list new questions by owner: %w", err)
	}
	return questions, nil
}
func (r *LearningStateRepository) MixedQuestionsForUser(userID int64, limit int) ([]models.Question, error) {
	var questions []models.Question
	if err := r.db.Where("user_id = ?", userID).Preload("Options").Order("RAND()").Limit(limit).Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("list mixed questions by owner: %w", err)
	}
	return questions, nil
}

func (r *LearningStateRepository) listStatesForUser(userID int64, condition string, args []any, order string, limit int) ([]models.QuestionLearningState, error) {
	var states []models.QuestionLearningState
	query := r.db.Where("question_learning_states.user_id = ?", userID).Preload("Question.Options").Where(condition, args...).Order(order).Limit(limit)
	if err := query.Find(&states).Error; err != nil {
		return nil, fmt.Errorf("list learning states by owner: %w", err)
	}
	return states, nil
}
