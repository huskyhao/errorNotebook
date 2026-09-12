package repository

import (
	"errors"
	"fmt"
	"time"

	"erro-notebook/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *JobRepository) GetLatestByQuestionIDAndType(questionID int64, jobType string) (*models.Job, error) {
	var job models.Job
	if err := r.db.Where("question_id = ? AND job_type = ?", questionID, jobType).
		Order("id desc").First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest job: %w", err)
	}
	return &job, nil
}

func (r *JobRepository) FindActiveByQuestionIDAndType(questionID int64, jobType string) (*models.Job, error) {
	var job models.Job
	if err := r.db.Where("question_id = ? AND job_type = ? AND status IN ?", questionID, jobType, []string{"queued", "pending", "processing"}).
		Order("id desc").First(&job).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active job: %w", err)
	}
	return &job, nil
}

// ClaimNext atomically claims one pending job, or one processing job whose
// worker lease expired. The row lock prevents multiple backend workers from
// executing the same task concurrently.
func (r *JobRepository) ClaimNext(now time.Time, staleAfter time.Duration) (*models.Job, error) {
	var job models.Job
	staleAt := now.Add(-staleAfter)
	err := r.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where(
			"(status IN ? AND (next_run_at IS NULL OR next_run_at <= ?)) OR (status = ? AND locked_at <= ?)",
			[]string{"queued", "pending"}, now, "processing", staleAt,
		).Order("created_at asc").Limit(1).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		if err := query.Find(&job).Error; err != nil {
			return fmt.Errorf("find claimable job: %w", err)
		}
		if job.ID == 0 {
			return nil
		}

		job.Status = "processing"
		job.Attempts++
		job.LockedAt = &now
		job.StartedAt = &now
		job.FinishedAt = nil
		job.ErrorCode = nil
		job.ErrorMessage = nil
		if job.MaxAttempts <= 0 {
			job.MaxAttempts = 3
		}
		if err := tx.Save(&job).Error; err != nil {
			return fmt.Errorf("claim job: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if job.ID == 0 {
		return nil, nil
	}
	return &job, nil
}

func (r *JobRepository) ResetForRetry(job *models.Job) error {
	now := time.Now()
	job.Status = "queued"
	job.NextRunAt = &now
	job.LockedAt = nil
	job.FinishedAt = nil
	job.ErrorCode = nil
	job.ErrorMessage = nil
	job.ProcessingStage = "queued"
	return r.Update(job)
}
