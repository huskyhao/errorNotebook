package repository

import (
	"errors"
	"fmt"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
)

type JobRepository struct {
	db *gorm.DB
}

func NewJobRepository(db *gorm.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) Create(job *models.Job) error {
	if err := r.db.Create(job).Error; err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

func (r *JobRepository) Update(job *models.Job) error {
	if err := r.db.Save(job).Error; err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	return nil
}

func (r *JobRepository) GetByJobID(jobID string) (*models.Job, error) {
	var job models.Job
	if err := r.db.Where("job_id = ?", jobID).First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get job by job_id: %w", err)
	}
	return &job, nil
}
