package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"erro-notebook/backend/internal/integrations/ai"
	"erro-notebook/backend/internal/models"
)

// StartTaskWorkers starts database-backed workers. Jobs remain pending in the
// database when the process exits, and expired processing leases are reclaimed
// on the next startup.
func (s *QuestionService) StartTaskWorkers(ctx context.Context, count int, pollInterval, staleAfter time.Duration) {
	if count < 1 {
		count = 1
	}
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	if staleAfter <= 0 {
		staleAfter = 10 * time.Minute
	}
	for workerID := 1; workerID <= count; workerID++ {
		go s.taskWorkerLoop(ctx, workerID, pollInterval, staleAfter)
	}
}

func (s *QuestionService) taskWorkerLoop(ctx context.Context, workerID int, pollInterval, staleAfter time.Duration) {
	log.Printf("[task-worker] started worker=%d", workerID)
	for {
		if ctx.Err() != nil {
			return
		}
		job, err := s.jobRepo.ClaimNext(time.Now(), staleAfter)
		if err != nil {
			log.Printf("[task-worker] claim failed worker=%d: %v", workerID, err)
			sleepContext(ctx, pollInterval)
			continue
		}
		if job == nil {
			sleepContext(ctx, pollInterval)
			continue
		}
		s.updateBatchItem(job.JobID, "processing", job.ProcessingStage, "")
		log.Printf("[task-worker] claimed worker=%d job=%s type=%s attempt=%d", workerID, job.JobID, job.JobType, job.Attempts)
		s.processClaimedJob(ctx, job)
	}
}

func (s *QuestionService) processClaimedJob(parent context.Context, job *models.Job) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()

	var err error
	switch job.JobType {
	case "ocr":
		err = s.processOCRJob(ctx, job)
	case "analyze":
		err = s.processAnalysisJob(ctx, job)
	default:
		err = fmt.Errorf("unsupported job type: %s", job.JobType)
	}
	if err == nil {
		return
	}
	s.failOrRetryJob(job, err)
}

func (s *QuestionService) processOCRJob(ctx context.Context, job *models.Job) error {
	question, err := s.questionRepo.GetByID(job.QuestionID)
	if err != nil || question == nil {
		return fmt.Errorf("load question: %w", err)
	}
	if err := s.runOCRObject(ctx, question, job); err != nil {
		return err
	}
	if ocrTaskStatus(question) == "needs_review" {
		job.ProcessingStage = "needs_review"
		_ = s.jobRepo.Update(markJobNeedsReview(job))
		s.updateBatchItem(job.JobID, "needs_review", "needs_review", "")
		return nil
	}
	job.ProcessingStage = "completed"
	_ = s.jobRepo.Update(markJobCompleted(job))
	s.updateBatchItem(job.JobID, "completed", "completed", "")
	if _, err := s.AnalyzeQuestion(ctx, question.ID); err != nil {
		return fmt.Errorf("enqueue analysis: %w", err)
	}
	return nil
}

func (s *QuestionService) processAnalysisJob(ctx context.Context, job *models.Job) error {
	question, err := s.questionRepo.GetByID(job.QuestionID)
	if err != nil || question == nil {
		return fmt.Errorf("load question: %w", err)
	}
	if existing, err := s.analysisRepo.GetByJobID(job.JobID); err != nil {
		return err
	} else if existing != nil {
		question.AnalysisStatus = "completed"
		_ = s.questionRepo.Update(question)
		_ = s.jobRepo.Update(markJobCompleted(job))
		return nil
	}
	question.AnalysisStatus = "processing"
	if err := s.questionRepo.Update(question); err != nil {
		return fmt.Errorf("mark analysis processing: %w", err)
	}
	job.ProcessingStage = "analysis"
	warnings := parseWarnings(question.StructureWarnings)
	req := aiAnalyzeRequest(question, warnings)
	imagePath, cleanup, err := s.materializeQuestionImage(ctx, question)
	if err != nil {
		return err
	}
	defer cleanup()
	resp, err := s.aiClientAsync.AnalyzeQuestion(ctx, req, imagePath)
	if err != nil {
		return fmt.Errorf("call ai service: %w", err)
	}
	resp.Analysis.TaxonomySuggestion = s.validateTaxonomySuggestion(resp.Analysis.TaxonomySuggestion)
	contentJSON, err := json.Marshal(resp.Analysis)
	if err != nil {
		return fmt.Errorf("marshal analysis: %w", err)
	}
	answer := resp.Analysis.Answer
	jobID := job.JobID
	if err := s.analysisRepo.Create(&models.Analysis{
		QuestionID: question.ID, JobID: &jobID, Provider: "ai-service", Answer: &answer, ContentJSON: string(contentJSON),
	}); err != nil {
		return fmt.Errorf("save analysis: %w", err)
	}
	question.AnalysisStatus = "completed"
	if answer != "" {
		question.CorrectAnswer = &answer
	}
	if err := s.questionRepo.Update(question); err != nil {
		return fmt.Errorf("save question analysis status: %w", err)
	}
	job.ProcessingStage = "completed"
	_ = s.jobRepo.Update(markJobCompleted(job))
	return nil
}

func aiAnalyzeRequest(question *models.Question, warnings []string) ai.AnalyzeQuestionRequest {
	return ai.AnalyzeQuestionRequest{
		QuestionID: question.ID,
		TraceID:    fmt.Sprintf("trace_analyze_%d", time.Now().UnixNano()),
		Question:   toAIStructuredQuestion(question, warnings),
		UserAnswer: derefString(question.UserAnswer),
		Context: map[string]any{
			"sourceType": question.SourceType, "structureWarnings": warnings,
			"structureConfidence": question.StructureConfidence, "parseSource": question.ParseSource,
			"structureMayBePartial": hasStructureWarning(warnings, "options_incomplete") || hasMissingOptionWarning(warnings),
		},
	}
}

func (s *QuestionService) materializeQuestionImage(ctx context.Context, question *models.Question) (string, func(), error) {
	if !question.HasDiagram || question.ImagePath == nil || s.objectStorage == nil {
		return "", func() {}, nil
	}
	reader, err := s.openQuestionImage(ctx, *question.ImagePath)
	if err != nil {
		return "", func() {}, fmt.Errorf("open question image: %w", err)
	}
	temp, err := os.CreateTemp("", "erro-analysis-*")
	if err != nil {
		reader.Close()
		return "", func() {}, fmt.Errorf("create analysis image: %w", err)
	}
	path := temp.Name()
	cleanup := func() { temp.Close(); reader.Close(); os.Remove(path) }
	if _, err := io.Copy(temp, reader); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("materialize question image: %w", err)
	}
	if err := temp.Close(); err != nil {
		reader.Close()
		os.Remove(path)
		return "", func() {}, fmt.Errorf("close analysis image: %w", err)
	}
	return path, cleanup, nil
}

func (s *QuestionService) openQuestionImage(ctx context.Context, key string) (io.ReadCloser, error) {
	if s.objectStorage != nil {
		if reader, err := s.objectStorage.Open(ctx, key); err == nil {
			return reader, nil
		}
	}
	// Compatibility for questions imported before object storage was added.
	// ImagePath is server-managed data, never a client-supplied path.
	if strings.TrimSpace(key) != "" {
		return os.Open(key)
	}
	return nil, fmt.Errorf("image object is unavailable")
}

func (s *QuestionService) failOrRetryJob(job *models.Job, err error) {
	message := strings.TrimSpace(err.Error())
	code := "TASK_FAILED"
	if job.JobType == "ocr" {
		code = "OCR_FAILED"
	} else if job.JobType == "analyze" {
		code = "ANALYZE_FAILED"
	}
	if job.Attempts < maxJobAttempts(job) {
		next := time.Now().Add(retryDelay(job.Attempts))
		job.Status = "pending"
		job.NextRunAt = &next
		job.LockedAt = nil
		job.ProcessingStage = "retry_wait"
		job.ErrorCode = &code
		job.ErrorMessage = &message
		_ = s.jobRepo.Update(job)
		s.updateBatchItem(job.JobID, "pending", "retry_wait", message)
		log.Printf("[task-worker] retry scheduled job=%s attempt=%d next=%s error=%s", job.JobID, job.Attempts, next.Format(time.RFC3339), message)
		return
	}
	markJobFailed(job, code, message)
	job.LockedAt = nil
	_ = s.jobRepo.Update(job)
	if job.JobType == "ocr" {
		s.markQuestionOCRFailed(job.QuestionID)
	} else if job.JobType == "analyze" {
		s.failQuestion(job.QuestionID)
	}
	s.updateBatchItem(job.JobID, "failed", "failed", message)
	log.Printf("[task-worker] terminal failure job=%s error=%s", job.JobID, message)
}

func (s *QuestionService) updateBatchItem(jobID, status, stage, message string) {
	if s.batchRepo == nil {
		return
	}
	item, err := s.batchRepo.GetItemByJobID(jobID)
	if err != nil || item == nil {
		return
	}
	item.Status = status
	item.ProcessingStage = stage
	if strings.TrimSpace(message) == "" {
		item.ErrorMsg = nil
	} else {
		value := strings.TrimSpace(message)
		item.ErrorMsg = &value
	}
	if err := s.batchRepo.UpdateItem(item); err != nil {
		log.Printf("[task-worker] update batch item failed job=%s: %v", jobID, err)
	}
}

func maxJobAttempts(job *models.Job) int {
	if job.MaxAttempts <= 0 {
		return 3
	}
	return job.MaxAttempts
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Duration(1<<minInt(attempt, 5)) * time.Second
	if delay > time.Minute {
		return time.Minute
	}
	return delay
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sleepContext(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
