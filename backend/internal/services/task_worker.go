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

	"erro-notebook/backend/internal/auth"
	"erro-notebook/backend/internal/integrations/ai"
	"erro-notebook/backend/internal/models"
)

// StartTaskWorkers starts database-backed workers. Jobs remain queued in the
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
	question, err := s.questionRepo.GetByIDForUser(job.QuestionID, job.UserID)
	if err != nil || question == nil {
		return fmt.Errorf("load question: %w", err)
	}
	if err := s.runOCRObject(ctx, question, job); err != nil {
		return err
	}
	if ocrTaskStatus(question) == "needs_review" && strings.TrimSpace(question.Stem) == "" {
		question.OCRStatus = "needs_review"
		_ = s.questionRepo.Update(question)
		job.ProcessingStage = "needs_review"
		_ = s.jobRepo.Update(markJobNeedsReview(job))
		s.updateBatchItem(job.JobID, "needs_review", "needs_review", "")
		return nil
	}
	// A usable stem may continue into AI even when OCR confidence or options
	// need review. The warning remains on Question for the UI to display.
	job.ProcessingStage = "completed"
	_ = s.jobRepo.Update(markJobCompleted(job))
	s.updateBatchItem(job.JobID, "processing", "analysis_queued", "")
	if _, err := s.AnalyzeQuestion(auth.WithUserID(ctx, job.UserID), question.ID); err != nil {
		return fmt.Errorf("enqueue analysis: %w", err)
	}
	return nil
}

func (s *QuestionService) processAnalysisJob(ctx context.Context, job *models.Job) error {
	question, err := s.questionRepo.GetByIDForUser(job.QuestionID, job.UserID)
	if err != nil || question == nil {
		return fmt.Errorf("load question: %w", err)
	}
	if existing, err := s.analysisRepo.GetByJobID(job.JobID); err != nil {
		return err
	} else if existing != nil {
		question.AnalysisStatus = "completed"
		_ = s.questionRepo.Update(question)
		_ = s.jobRepo.Update(markJobCompleted(job))
		s.updateBatchItemByQuestion(job.QuestionID, job.UserID, "completed", "completed", "")
		return nil
	}
	question.AnalysisStatus = "processing"
	if err := s.questionRepo.Update(question); err != nil {
		return fmt.Errorf("mark analysis processing: %w", err)
	}
	job.ProcessingStage = "analysis"
	warnings := parseWarnings(question.StructureWarnings)
	var categories []models.Category
	var tags []models.Tag
	if s.categoryRepo != nil {
		categories, _ = s.categoryRepo.ListForUser(job.UserID)
	}
	if s.tagRepo != nil {
		tags, _ = s.tagRepo.ListForUser(job.UserID)
	}
	categoryCandidates := categoryNamesForAI(categories, job.UserID)
	req := aiAnalyzeRequest(question, warnings, categoryCandidates, tagNames(tags))
	imagePath, cleanup, err := s.materializeQuestionImage(ctx, question)
	if err != nil {
		return err
	}
	defer cleanup()
	resp, err := s.aiClientAsync.AnalyzeQuestion(ctx, req, imagePath)
	if err != nil {
		return fmt.Errorf("call ai service: %w", err)
	}
	suggestion := s.validateTaxonomySuggestion(resp.Analysis.TaxonomySuggestion, job.UserID)
	if suggestion != nil {
		s.ensurePrivateSuggestedTags(suggestion, job.UserID)
		suggestion = s.validateTaxonomySuggestion(suggestion, job.UserID)
	}
	resp.Analysis.TaxonomySuggestion = suggestion
	contentJSON, err := json.Marshal(resp.Analysis)
	if err != nil {
		return fmt.Errorf("marshal analysis: %w", err)
	}
	answer := resp.Analysis.Answer
	jobID := job.JobID
	snapshot, _ := json.Marshal(map[string]any{"categoryCandidates": categoryCandidates, "tagCandidates": tagNames(tags)})
	if err := s.analysisRepo.Create(&models.Analysis{
		UserID: job.UserID, QuestionID: question.ID, JobID: &jobID, Provider: "ai-service", Answer: &answer, ContentJSON: string(contentJSON),
		SourceQuestionFingerprint: questionContentFingerprint(question), TaxonomyCandidateSnapshotJSON: string(snapshot), GeneratedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("save analysis: %w", err)
	}
	question.AnalysisStatus = resp.Status
	if question.AnalysisStatus == "" {
		question.AnalysisStatus = "completed"
	}
	if answer != "" {
		question.CorrectAnswer = &answer
	}
	if err := s.questionRepo.Update(question); err != nil {
		return fmt.Errorf("save question analysis status: %w", err)
	}
	// AI taxonomy is the default when the question has no manual taxonomy yet.
	// The user can still adjust it later from the detail panel; a manual choice
	// is never overwritten by a later analysis.
	if suggestion := resp.Analysis.TaxonomySuggestion; suggestion != nil &&
		(question.CategoryID == nil && len(question.Tags) == 0) &&
		(suggestion.CategoryID != nil || len(suggestion.TagIDs) > 0) {
		if err := s.questionRepo.ApplyTaxonomy(question.ID, suggestion.CategoryID, suggestion.TagIDs, job.UserID); err != nil {
			log.Printf("[task-worker] auto-apply taxonomy failed question=%d: %v", question.ID, err)
		}
	}
	if resp.Status == "needs_review" {
		job.ProcessingStage = "needs_review"
		_ = s.jobRepo.Update(markJobNeedsReview(job))
	} else {
		job.ProcessingStage = "completed"
		_ = s.jobRepo.Update(markJobCompleted(job))
	}
	if resp.Status == "needs_review" {
		s.updateBatchItem(job.JobID, "needs_review", "needs_review", "")
		s.updateBatchItemByQuestion(job.QuestionID, job.UserID, "needs_review", "needs_review", "")
	} else {
		s.updateBatchItem(job.JobID, "completed", "completed", "")
		s.updateBatchItemByQuestion(job.QuestionID, job.UserID, "completed", "completed", "")
	}
	return nil
}

func aiAnalyzeRequest(question *models.Question, warnings []string, categoryCandidates []string, tagCandidates []string) ai.AnalyzeQuestionRequest {
	return ai.AnalyzeQuestionRequest{
		QuestionID: question.ID,
		TraceID:    fmt.Sprintf("trace_analyze_%d", time.Now().UnixNano()),
		Question:   toAIStructuredQuestion(question, warnings),
		UserAnswer: derefString(question.UserAnswer),
		Context: map[string]any{
			"sourceType": question.SourceType, "structureWarnings": warnings,
			"structureConfidence": question.StructureConfidence, "parseSource": question.ParseSource,
			"structureMayBePartial": hasStructureWarning(warnings, "options_incomplete") || hasMissingOptionWarning(warnings),
			"categoryCandidates":    categoryCandidates, "tagCandidates": tagCandidates,
		},
	}
}

func (s *QuestionService) materializeQuestionImage(ctx context.Context, question *models.Question) (string, func(), error) {
	// An imported image is useful to the vision model even when OCR classified
	// it as a text-only question. HasDiagram describes the parsed structure; it
	// must not decide whether the original evidence is sent to AI.
	if question == nil || question.ImagePath == nil {
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
		job.Status = "queued"
		job.NextRunAt = &next
		job.LockedAt = nil
		job.ProcessingStage = "retry_wait"
		job.ErrorCode = &code
		job.ErrorMessage = &message
		_ = s.jobRepo.Update(job)
		s.updateBatchItem(job.JobID, "queued", "retry_wait", message)
		log.Printf("[task-worker] retry scheduled job=%s attempt=%d next=%s error=%s", job.JobID, job.Attempts, next.Format(time.RFC3339), message)
		return
	}
	markJobFailed(job, code, message)
	job.LockedAt = nil
	_ = s.jobRepo.Update(job)
	if job.JobType == "ocr" {
		s.markQuestionOCRFailedForUser(job.QuestionID, job.UserID)
	} else if job.JobType == "analyze" {
		s.failQuestionForUser(job.QuestionID, job.UserID)
	}
	s.updateBatchItem(job.JobID, "failed", "failed", message)
	s.updateBatchItemByQuestion(job.QuestionID, job.UserID, "failed", "failed", message)
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

func (s *QuestionService) updateBatchItemByQuestion(questionID, userID int64, status, stage, message string) {
	if s.batchRepo == nil {
		return
	}
	item, err := s.batchRepo.GetItemByQuestionID(questionID, userID)
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
		log.Printf("[task-worker] update batch item by question failed question=%d: %v", questionID, err)
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
