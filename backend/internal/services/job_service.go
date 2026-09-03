package services

import (
	"errors"

	"erro-notebook/backend/internal/models"
	"erro-notebook/backend/internal/repository"
)

var ErrJobNotFound = errors.New("job not found")

type JobService struct {
	jobRepo *repository.JobRepository
}

func NewJobService(jobRepo *repository.JobRepository) *JobService {
	return &JobService{jobRepo: jobRepo}
}

func (s *JobService) GetByJobID(jobID string) (*models.Job, error) {
	job, err := s.jobRepo.GetByJobID(jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, ErrJobNotFound
	}
	return job, nil
}
