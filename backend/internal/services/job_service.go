package services

import (
	"context"
	"errors"

	"erro-notebook/backend/internal/models"
	"erro-notebook/backend/internal/repository"
)

var ErrJobNotFound = errors.New("job not found")
var ErrJobNotRetryable = errors.New("job is not retryable")

type JobService struct {
	jobRepo *repository.JobRepository
}

func NewJobService(jobRepo *repository.JobRepository) *JobService {
	return &JobService{jobRepo: jobRepo}
}

func (s *JobService) GetByJobID(ctx context.Context, jobID string) (*models.Job, error) {
	job, err := s.jobRepo.GetByJobID(jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, ErrJobNotFound
	}
	if job.Status == "pending" {
		job.Status = "queued"
	}
	return job, nil
}

func (s *JobService) RetryJob(ctx context.Context, jobID string) (*models.Job, error) {
	job, err := s.GetByJobID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job.Status != "failed" {
		return nil, ErrJobNotRetryable
	}
	job.Attempts = 0
	if err := s.jobRepo.ResetForRetry(job); err != nil {
		return nil, err
	}
	return job, nil
}
