package repository

import (
	"errors"
	"fmt"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
)

type AnalysisRepository struct {
	db *gorm.DB
}

func NewAnalysisRepository(db *gorm.DB) *AnalysisRepository {
	return &AnalysisRepository{db: db}
}

func (r *AnalysisRepository) Create(analysis *models.Analysis) error {
	if err := r.db.Create(analysis).Error; err != nil {
		return fmt.Errorf("create analysis: %w", err)
	}
	return nil
}

func (r *AnalysisRepository) GetLatestByQuestionID(questionID int64) (*models.Analysis, error) {
	var analysis models.Analysis
	if err := r.db.Where("question_id = ?", questionID).Order("id desc").First(&analysis).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest analysis: %w", err)
	}
	return &analysis, nil
}

func (r *AnalysisRepository) GetByJobID(jobID string) (*models.Analysis, error) {
	var analysis models.Analysis
	if err := r.db.Where("job_id = ?", jobID).First(&analysis).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get analysis by job: %w", err)
	}
	return &analysis, nil
}
