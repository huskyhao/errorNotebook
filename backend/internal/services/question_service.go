package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"erro-notebook/backend/internal/integrations/ai"
	"erro-notebook/backend/internal/models"
	"erro-notebook/backend/internal/repository"
)

var ErrQuestionNotFound = errors.New("question not found")

type QuestionService struct {
	questionRepo     *repository.QuestionRepository
	jobRepo          *repository.JobRepository
	analysisRepo     *repository.AnalysisRepository
	chatRepo         *repository.ChatRepository
	batchRepo        *repository.BatchRepository
	learningRepo     *repository.LearningStateRepository
	aiClient         *ai.Client
	aiClientAsync    *ai.Client // long timeout for async analysis goroutines
	maxConcurrentOCR int
}

func NewQuestionService(
	questionRepo *repository.QuestionRepository,
	jobRepo *repository.JobRepository,
	analysisRepo *repository.AnalysisRepository,
	chatRepo *repository.ChatRepository,
	batchRepo *repository.BatchRepository,
	learningRepo *repository.LearningStateRepository,
	aiClient *ai.Client,
	aiClientAsync *ai.Client,
	maxConcurrentOCR int,
) *QuestionService {
	return &QuestionService{
		questionRepo:     questionRepo,
		jobRepo:          jobRepo,
		analysisRepo:     analysisRepo,
		chatRepo:         chatRepo,
		batchRepo:        batchRepo,
		learningRepo:     learningRepo,
		aiClient:         aiClient,
		aiClientAsync:    aiClientAsync,
		maxConcurrentOCR: maxConcurrentOCR,
	}
}

type ImportQuestionInput struct {
	SourceType string
	RawText    string
	FileHeader *multipart.FileHeader
}

type ImportQuestionResult struct {
	JobID      string `json:"jobId"`
	QuestionID int64  `json:"questionId"`
	Status     string `json:"status"`
}

type QuestionDetail struct {
	ID                  int64                   `json:"id"`
	Stem                string                  `json:"stem"`
	QuestionType        string                  `json:"questionType"`
	CorrectAnswer       *string                 `json:"correctAnswer,omitempty"`
	UserAnswer          *string                 `json:"userAnswer,omitempty"`
	OCRStatus           string                  `json:"ocrStatus"`
	AnalysisStatus      string                  `json:"analysisStatus"`
	SourceType          string                  `json:"sourceType"`
	RawOCRText          *string                 `json:"rawOcrText,omitempty"`
	StructureWarnings   []string                `json:"structureWarnings,omitempty"`
	StructureConfidence *float64                `json:"structureConfidence,omitempty"`
	ParseSource         string                  `json:"parseSource"`
	QualityStatus       string                  `json:"qualityStatus"`
	CategoryID          *int64                  `json:"categoryId,omitempty"`
	CategoryName        *string                 `json:"categoryName,omitempty"`
	DiagramDescription  *string                 `json:"diagramDescription,omitempty"`
	HasDiagram          bool                    `json:"hasDiagram"`
	ImagePath           *string                 `json:"imagePath,omitempty"`
	IsFavorited         bool                    `json:"isFavorited"`
	Options             []models.QuestionOption `json:"options"`
	Assets              []models.QuestionAsset  `json:"assets"`
	Tags                []models.Tag            `json:"tags,omitempty"`
	LearningState       *LearningStateDetail    `json:"learningState,omitempty"`
}

type UpdateQuestionInput struct {
	Stem          *string
	QuestionType  *string
	CorrectAnswer *string
	CategoryID    *int64
	Options       []models.QuestionOption
}

type SubmitAnswerInput struct {
	UserAnswer string `json:"userAnswer"`
}

type LearningStateDetail struct {
	ID              int64      `json:"id"`
	QuestionID      int64      `json:"questionId"`
	MasteryLevel    int        `json:"masteryLevel"`
	WrongCount      int        `json:"wrongCount"`
	CorrectStreak   int        `json:"correctStreak"`
	LastPracticedAt *time.Time `json:"lastPracticedAt,omitempty"`
	NextReviewAt    *time.Time `json:"nextReviewAt,omitempty"`
	MistakeReason   string     `json:"mistakeReason"`
	WeaknessTags    []string   `json:"weaknessTags"`
	ReviewAdvice    []string   `json:"reviewAdvice"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type UpdateLearningStateInput struct {
	MasteryLevel  *int
	MistakeReason *string
	WeaknessTags  []string
	ReviewAdvice  []string
}

type AnalyzeQuestionResult struct {
	JobID      string `json:"jobId"`
	QuestionID int64  `json:"questionId"`
	Status     string `json:"status"`
}

type AnalysisDetail struct {
	ID         int64          `json:"id"`
	QuestionID int64          `json:"questionId"`
	Provider   string         `json:"provider"`
	Answer     *string        `json:"answer,omitempty"`
	Content    map[string]any `json:"content"`
	CreatedAt  time.Time      `json:"createdAt"`
}

type BatchImportInput struct {
	SourceType string
	Files      []*multipart.FileHeader
}

type BatchImportItemResult struct {
	FileIndex       int    `json:"fileIndex"`
	FileName        string `json:"fileName"`
	QuestionID      int64  `json:"questionId,omitempty"`
	Status          string `json:"status"`
	ProcessingStage string `json:"processingStage,omitempty"`
	Error           string `json:"error,omitempty"`
}

type BatchImportResult struct {
	BatchID   int64                   `json:"batchId"`
	Total     int                     `json:"total"`
	Questions []BatchImportItemResult `json:"questions"`
}

func (s *QuestionService) BatchImportQuestions(ctx context.Context, input BatchImportInput) (*BatchImportResult, error) {
	if sourceType := strings.TrimSpace(input.SourceType); sourceType != "" && sourceType != "image" {
		return nil, fmt.Errorf("batch import only supports image files")
	}
	for _, fileHeader := range input.Files {
		if err := validateImageFile(fileHeader); err != nil {
			return nil, err
		}
	}

	batch := &models.BatchImport{
		UserID:     1,
		TotalFiles: len(input.Files),
	}
	if err := s.batchRepo.CreateBatch(batch); err != nil {
		return nil, err
	}

	items := make([]models.BatchImportItem, len(input.Files))
	for i, fh := range input.Files {
		question := &models.Question{
			UserID:         1,
			Stem:           "",
			QuestionType:   "subjective",
			OCRStatus:      "uploaded",
			AnalysisStatus: "pending",
			SourceType:     strings.TrimSpace(input.SourceType),
		}
		if question.SourceType == "" {
			question.SourceType = "image"
		}
		if err := s.questionRepo.Create(question); err != nil {
			return nil, err
		}
		items[i] = models.BatchImportItem{
			BatchID:         batch.ID,
			QuestionID:      question.ID,
			FileIndex:       i,
			FileName:        fh.Filename,
			Status:          "pending",
			ProcessingStage: "pending",
		}
	}
	if err := s.batchRepo.CreateItems(items); err != nil {
		return nil, err
	}

	// Launch OCR workers without blocking the request on the concurrency gate.
	limit := s.maxConcurrentOCR
	if limit < 1 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	for i := range items {
		go func(idx int, item models.BatchImportItem, fh *multipart.FileHeader) {
			sem <- struct{}{}
			defer func() { <-sem }()
			s.processBatchItem(item, fh)
		}(i, items[i], input.Files[i])
	}

	result := &BatchImportResult{
		BatchID: batch.ID,
		Total:   len(input.Files),
	}
	for _, item := range items {
		result.Questions = append(result.Questions, BatchImportItemResult{
			FileIndex:       item.FileIndex,
			FileName:        item.FileName,
			QuestionID:      item.QuestionID,
			Status:          item.Status,
			ProcessingStage: item.ProcessingStage,
		})
	}
	return result, nil
}

func (s *QuestionService) processBatchItem(item models.BatchImportItem, fh *multipart.FileHeader) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[batch] panic batch=%d item=%d question=%d file=%s stage=%s: %v", item.BatchID, item.FileIndex, item.QuestionID, item.FileName, item.ProcessingStage, r)
			s.failBatchItem(&item, "panic", fmt.Sprintf("batch item panic at %s: %v", item.ProcessingStage, r))
		}
	}()

	item.Status = "processing"
	item.ProcessingStage = "loading_question"
	_ = s.batchRepo.UpdateItem(&item)

	question, err := s.questionRepo.GetByID(item.QuestionID)
	if err != nil || question == nil {
		s.failBatchItem(&item, "loading_question", "question not found")
		return
	}

	item.ProcessingStage = "creating_ocr_job"
	_ = s.batchRepo.UpdateItem(&item)
	job := &models.Job{
		JobID:      newJobID("ocr"),
		QuestionID: question.ID,
		JobType:    "ocr",
		Status:     "pending",
	}
	if err := s.jobRepo.Create(job); err != nil {
		s.failBatchItem(&item, "creating_ocr_job", fmt.Sprintf("create job: %v", err))
		return
	}

	item.ProcessingStage = "ocr_structuring"
	_ = s.batchRepo.UpdateItem(&item)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := s.runOCR(ctx, question, job, fh); err != nil {
		s.markQuestionOCRFailed(question.ID)
		s.failBatchItem(&item, "ocr_structuring", err.Error())
		return
	}

	item.Status = ocrTaskStatus(question)
	item.ProcessingStage = item.Status
	item.ErrorMsg = nil
	_ = s.batchRepo.UpdateItem(&item)

	// Auto-trigger analysis
	if _, err := s.AnalyzeQuestion(context.Background(), question.ID); err != nil {
		log.Printf("[batch] analyze trigger failed question=%d: %v", question.ID, err)
	}
}

func (s *QuestionService) failBatchItem(item *models.BatchImportItem, stage string, message string) {
	item.Status = "failed"
	item.ProcessingStage = stage
	errMsg := strings.TrimSpace(message)
	if errMsg == "" {
		errMsg = "batch item failed"
	}
	item.ErrorMsg = &errMsg
	_ = s.batchRepo.UpdateItem(item)
}

func (s *QuestionService) markQuestionOCRFailed(questionID int64) {
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil || question == nil {
		return
	}
	question.OCRStatus = "failed"
	_ = s.questionRepo.Update(question)
}

func (s *QuestionService) GetBatchImport(ctx context.Context, batchID int64) (*BatchImportResult, error) {
	batch, err := s.batchRepo.GetBatchByID(batchID)
	if err != nil {
		return nil, err
	}
	if batch == nil {
		return nil, ErrQuestionNotFound
	}

	items, err := s.batchRepo.GetItemsByBatchID(batchID)
	if err != nil {
		return nil, err
	}

	result := &BatchImportResult{
		BatchID: batch.ID,
		Total:   batch.TotalFiles,
	}
	for _, item := range items {
		errMsg := ""
		if item.ErrorMsg != nil {
			errMsg = *item.ErrorMsg
		}
		result.Questions = append(result.Questions, BatchImportItemResult{
			FileIndex:       item.FileIndex,
			FileName:        item.FileName,
			QuestionID:      item.QuestionID,
			Status:          item.Status,
			ProcessingStage: item.ProcessingStage,
			Error:           errMsg,
		})
	}
	return result, nil
}

type CreateChatMessageInput struct {
	Message         string                  `json:"message"`
	AttachmentFiles []*multipart.FileHeader `json:"-"`
}

type ChatAttachment struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	FilePath    string `json:"filePath"`
	Size        int64  `json:"size"`
}

func (s *QuestionService) ImportQuestion(ctx context.Context, input ImportQuestionInput) (*ImportQuestionResult, error) {
	sourceType := strings.TrimSpace(input.SourceType)
	if sourceType == "" {
		sourceType = "image"
	}
	if sourceType != "manual" {
		if sourceType != "image" {
			return nil, fmt.Errorf("unsupported source type: %s", sourceType)
		}
		if err := validateImageFile(input.FileHeader); err != nil {
			return nil, err
		}
	}

	question := &models.Question{
		UserID:         1,
		Stem:           strings.TrimSpace(input.RawText),
		QuestionType:   "subjective",
		OCRStatus:      "uploaded",
		AnalysisStatus: "pending",
		SourceType:     sourceType,
	}
	if sourceType == "manual" {
		question.OCRStatus = "completed"
	}
	if err := s.questionRepo.Create(question); err != nil {
		return nil, err
	}

	job := &models.Job{
		JobID:      newJobID("ocr"),
		QuestionID: question.ID,
		JobType:    "ocr",
		Status:     "pending",
	}
	if err := s.jobRepo.Create(job); err != nil {
		return nil, err
	}

	if sourceType == "manual" {
		if err := s.jobRepo.Update(markJobCompleted(job)); err != nil {
			return nil, err
		}
		return &ImportQuestionResult{
			JobID:      job.JobID,
			QuestionID: question.ID,
			Status:     "completed",
		}, nil
	}

	if err := s.runOCR(ctx, question, job, input.FileHeader); err != nil {
		return nil, err
	}

	// Auto-trigger analysis after successful OCR (runs async in background).
	if _, err := s.AnalyzeQuestion(ctx, question.ID); err != nil {
		// Pre-flight check failed (question not found, DB error); import still succeeds.
	}

	return &ImportQuestionResult{
		JobID:      job.JobID,
		QuestionID: question.ID,
		Status:     ocrTaskStatus(question),
	}, nil
}

func validateImageFile(fileHeader *multipart.FileHeader) error {
	if fileHeader == nil {
		return fmt.Errorf("image file is required")
	}
	contentType := strings.ToLower(strings.TrimSpace(fileHeader.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "image/") {
		return nil
	}
	switch strings.ToLower(filepath.Ext(fileHeader.Filename)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return nil
	default:
		return fmt.Errorf("only image files are supported")
	}
}

func (s *QuestionService) GetQuestion(ctx context.Context, id int64) (*QuestionDetail, error) {
	question, err := s.questionRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}

	detail := toQuestionDetail(question)
	s.attachLearningState(detail)
	return detail, nil
}

func (s *QuestionService) ListQuestions(ctx context.Context, filters repository.QuestionListFilters) ([]QuestionDetail, error) {
	questions, err := s.questionRepo.List(filters)
	if err != nil {
		return nil, err
	}

	result := make([]QuestionDetail, 0, len(questions))
	for i := range questions {
		detail := toQuestionDetail(&questions[i])
		s.attachLearningState(detail)
		result = append(result, *detail)
	}
	return result, nil
}

func (s *QuestionService) UpdateQuestion(ctx context.Context, id int64, input UpdateQuestionInput) (*QuestionDetail, error) {
	question, err := s.questionRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}

	if input.Stem != nil {
		question.Stem = strings.TrimSpace(*input.Stem)
	}
	if input.QuestionType != nil {
		questionType := normalizeQuestionType(*input.QuestionType)
		if !models.IsSupportedQuestionType(questionType) {
			return nil, fmt.Errorf("unsupported question type: %s", *input.QuestionType)
		}
		question.QuestionType = questionType
	}
	if input.CorrectAnswer != nil {
		value := strings.TrimSpace(*input.CorrectAnswer)
		question.CorrectAnswer = &value
	}
	if input.CategoryID != nil {
		question.CategoryID = input.CategoryID
	}
	if err := s.questionRepo.Update(question); err != nil {
		return nil, err
	}

	if input.Options != nil {
		if err := s.questionRepo.ReplaceOptions(id, normalizeOptions(id, input.Options)); err != nil {
			return nil, err
		}
	}
	if input.Stem != nil || input.QuestionType != nil || input.CorrectAnswer != nil || input.Options != nil {
		optionsForQuality := question.Options
		if input.Options != nil {
			optionsForQuality = input.Options
		}
		question.ParseSource = "manual_corrected"
		if hasObviousStructureIssue(question.QuestionType, optionsForQuality) {
			warningsJSON, _ := json.Marshal([]string{"options_incomplete"})
			value := string(warningsJSON)
			question.StructureWarnings = &value
		} else {
			warningsJSON, _ := json.Marshal([]string{})
			value := string(warningsJSON)
			question.StructureWarnings = &value
		}
		if err := s.questionRepo.Update(question); err != nil {
			return nil, err
		}
	}

	return s.GetQuestion(ctx, id)
}

func (s *QuestionService) DeleteQuestion(ctx context.Context, id int64) error {
	question, err := s.questionRepo.GetByID(id)
	if err != nil {
		return err
	}
	if question == nil {
		return ErrQuestionNotFound
	}

	if question.ImagePath != nil {
		uploadsDir := filepath.Dir(*question.ImagePath)
		if removeErr := os.RemoveAll(uploadsDir); removeErr != nil {
			// Log but don't fail — cleanup is best-effort.
		}
	}

	return s.questionRepo.Delete(id)
}

func (s *QuestionService) ToggleFavorite(ctx context.Context, id int64, isFavorited bool) (*QuestionDetail, error) {
	question, err := s.questionRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}
	question.IsFavorited = isFavorited
	if err := s.questionRepo.Update(question); err != nil {
		return nil, err
	}
	return s.GetQuestion(ctx, id)
}

func (s *QuestionService) SetQuestionTags(ctx context.Context, questionID int64, tagIDs []int64) (*QuestionDetail, error) {
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}
	if err := s.questionRepo.SetTags(questionID, tagIDs); err != nil {
		return nil, err
	}
	return s.GetQuestion(ctx, questionID)
}

func (s *QuestionService) SubmitAnswer(ctx context.Context, id int64, input SubmitAnswerInput) (*QuestionDetail, error) {
	question, err := s.questionRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}

	answer := strings.TrimSpace(input.UserAnswer)
	question.UserAnswer = &answer
	if err := s.questionRepo.Update(question); err != nil {
		return nil, err
	}

	return s.GetQuestion(ctx, id)
}

func (s *QuestionService) AnalyzeQuestion(ctx context.Context, id int64) (*AnalyzeQuestionResult, error) {
	question, err := s.questionRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}

	job := &models.Job{
		JobID:      newJobID("analyze"),
		QuestionID: id,
		JobType:    "analyze",
		Status:     "pending",
	}
	if err := s.jobRepo.Create(job); err != nil {
		return nil, err
	}

	question.AnalysisStatus = "processing"
	if err := s.questionRepo.Update(question); err != nil {
		return nil, err
	}

	traceID := fmt.Sprintf("trace_analyze_%d", time.Now().UnixNano())
	warnings := parseWarnings(question.StructureWarnings)
	req := ai.AnalyzeQuestionRequest{
		QuestionID: question.ID,
		TraceID:    traceID,
		Question:   toAIStructuredQuestion(question, warnings),
		UserAnswer: derefString(question.UserAnswer),
		Context: map[string]any{
			"sourceType":            question.SourceType,
			"structureWarnings":     warnings,
			"structureConfidence":   question.StructureConfidence,
			"parseSource":           question.ParseSource,
			"structureMayBePartial": hasStructureWarning(warnings, "options_incomplete") || hasMissingOptionWarning(warnings),
		},
	}

	imagePath := ""
	if question.HasDiagram && question.ImagePath != nil {
		imagePath = *question.ImagePath
	}

	log.Printf("[analyze-async] launching goroutine question=%d job=%s", id, job.JobID)
	go s.runAnalyzeAsync(job, req, imagePath)

	return &AnalyzeQuestionResult{
		JobID:      job.JobID,
		QuestionID: id,
		Status:     "processing",
	}, nil
}

func (s *QuestionService) runAnalyzeAsync(job *models.Job, req ai.AnalyzeQuestionRequest, imagePath string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[analyze-async] PANIC question=%d job=%s: %v", req.QuestionID, job.JobID, r)
			s.failQuestion(req.QuestionID)
			markJobFailed(job, "ANALYZE_PANIC", fmt.Sprintf("%v", r))
			_ = s.jobRepo.Update(job)
		}
	}()

	log.Printf("[analyze-async] started question=%d job=%s", req.QuestionID, job.JobID)
	bgCtx := context.Background()
	now := time.Now()
	job.Status = "processing"
	job.StartedAt = &now
	_ = s.jobRepo.Update(job)

	log.Printf("[analyze-async] calling ai-service question=%d", req.QuestionID)
	resp, err := s.aiClientAsync.AnalyzeQuestion(bgCtx, req, imagePath)
	if err != nil {
		log.Printf("[analyze-async] ai-service FAILED question=%d: %v", req.QuestionID, err)
		s.failQuestion(req.QuestionID)
		markJobFailed(job, "ANALYZE_FAILED", err.Error())
		_ = s.jobRepo.Update(job)
		return
	}
	log.Printf("[analyze-async] ai-service OK question=%d answer=%s", req.QuestionID, resp.Analysis.Answer)

	contentJSON, err := json.Marshal(resp.Analysis)
	if err != nil {
		log.Printf("[analyze-async] marshal FAILED question=%d: %v", req.QuestionID, err)
		s.failQuestion(req.QuestionID)
		markJobFailed(job, "ANALYZE_MARSHAL_FAILED", err.Error())
		_ = s.jobRepo.Update(job)
		return
	}

	answer := resp.Analysis.Answer
	analysis := &models.Analysis{
		QuestionID:  req.QuestionID,
		Provider:    "ai-service",
		Answer:      &answer,
		ContentJSON: string(contentJSON),
	}
	if err := s.analysisRepo.Create(analysis); err != nil {
		log.Printf("[analyze-async] db-create FAILED question=%d: %v", req.QuestionID, err)
		s.failQuestion(req.QuestionID)
		markJobFailed(job, "ANALYZE_DB_FAILED", err.Error())
		_ = s.jobRepo.Update(job)
		return
	}

	question, err := s.questionRepo.GetByID(req.QuestionID)
	if err != nil || question == nil {
		log.Printf("[analyze-async] question-gone question=%d", req.QuestionID)
		markJobFailed(job, "ANALYZE_QUESTION_GONE", "question not found after analysis")
		_ = s.jobRepo.Update(job)
		return
	}

	question.AnalysisStatus = "completed"
	if resp.Analysis.Answer != "" {
		question.CorrectAnswer = &resp.Analysis.Answer
	}
	_ = s.questionRepo.Update(question)
	_ = s.jobRepo.Update(markJobCompleted(job))
	log.Printf("[analyze-async] completed question=%d", req.QuestionID)
}

func (s *QuestionService) failQuestion(questionID int64) {
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil || question == nil {
		return
	}
	question.AnalysisStatus = "failed"
	_ = s.questionRepo.Update(question)
}

func (s *QuestionService) GetLatestAnalysis(ctx context.Context, questionID int64) (*AnalysisDetail, error) {
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}

	analysis, err := s.analysisRepo.GetLatestByQuestionID(questionID)
	if err != nil {
		return nil, err
	}
	if analysis == nil {
		return nil, nil
	}

	content := map[string]any{}
	if err := json.Unmarshal([]byte(analysis.ContentJSON), &content); err != nil {
		return nil, fmt.Errorf("unmarshal analysis content: %w", err)
	}

	return &AnalysisDetail{
		ID:         analysis.ID,
		QuestionID: analysis.QuestionID,
		Provider:   analysis.Provider,
		Answer:     analysis.Answer,
		Content:    content,
		CreatedAt:  analysis.CreatedAt,
	}, nil
}

func (s *QuestionService) GetLearningState(ctx context.Context, questionID int64) (*LearningStateDetail, error) {
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}
	if s.learningRepo == nil {
		return nil, fmt.Errorf("learning state repository unavailable")
	}
	state, err := s.learningRepo.Ensure(questionID)
	if err != nil {
		return nil, err
	}
	return toLearningStateDetail(state), nil
}

func (s *QuestionService) UpdateLearningState(ctx context.Context, questionID int64, input UpdateLearningStateInput) (*LearningStateDetail, error) {
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}
	state, err := s.learningRepo.Ensure(questionID)
	if err != nil {
		return nil, err
	}
	if input.MasteryLevel != nil {
		state.MasteryLevel = clampInt(*input.MasteryLevel, 0, 5)
	}
	if input.MistakeReason != nil {
		value := strings.TrimSpace(*input.MistakeReason)
		state.MistakeReason = trimOptionalString(&value)
	}
	if input.WeaknessTags != nil {
		state.WeaknessTags = marshalStringSlice(cleanStringSlice(input.WeaknessTags))
	}
	if input.ReviewAdvice != nil {
		state.ReviewAdvice = marshalStringSlice(cleanStringSlice(input.ReviewAdvice))
	}
	if err := s.learningRepo.Save(state); err != nil {
		return nil, err
	}
	return toLearningStateDetail(state), nil
}

func (s *QuestionService) GenerateLearningState(ctx context.Context, questionID int64) (*LearningStateDetail, error) {
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}
	analysis, _ := s.analysisRepo.GetLatestByQuestionID(questionID)
	state, err := s.learningRepo.Ensure(questionID)
	if err != nil {
		return nil, err
	}

	reason := buildMistakeReason(question, analysis)
	state.MistakeReason = &reason
	tags := inferWeaknessTags(question, analysis)
	state.WeaknessTags = marshalStringSlice(tags)
	state.ReviewAdvice = marshalStringSlice(buildReviewAdvice(question, tags))
	if err := s.learningRepo.Save(state); err != nil {
		return nil, err
	}
	return toLearningStateDetail(state), nil
}

func (s *QuestionService) CreateChatMessage(ctx context.Context, questionID int64, input CreateChatMessageInput) ([]models.ChatMessage, error) {
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}

	attachments, err := s.saveChatAttachments(questionID, input.AttachmentFiles)
	if err != nil {
		return nil, err
	}
	var attachmentJSON *string
	if len(attachments) > 0 {
		payload, err := json.Marshal(attachments)
		if err != nil {
			return nil, fmt.Errorf("marshal chat attachments: %w", err)
		}
		value := string(payload)
		attachmentJSON = &value
	}
	messageText := strings.TrimSpace(input.Message)
	if messageText == "" && len(attachments) > 0 {
		messageText = "请结合我补充的图片继续讲解。"
	}

	userMessage := &models.ChatMessage{
		QuestionID:     questionID,
		Role:           "user",
		Message:        messageText,
		AttachmentJSON: attachmentJSON,
	}
	if err := s.chatRepo.Create(userMessage); err != nil {
		return nil, err
	}

	// Build AI chat request context
	analysis, _ := s.analysisRepo.GetLatestByQuestionID(questionID)
	var analysisPayload *ai.AnalysisPayload
	if analysis != nil {
		var ap ai.AnalysisPayload
		if err := json.Unmarshal([]byte(analysis.ContentJSON), &ap); err == nil {
			analysisPayload = &ap
		}
	}

	allHistory, _ := s.chatRepo.ListByQuestionID(questionID)
	history := buildChatHistory(allHistory, userMessage)

	traceID := fmt.Sprintf("trace_chat_%d", time.Now().UnixNano())
	warnings := parseWarnings(question.StructureWarnings)
	aiReq := ai.ChatRequest{
		QuestionID: questionID,
		TraceID:    traceID,
		Question:   toAIStructuredQuestion(question, warnings),
		UserAnswer: derefString(question.UserAnswer),
		Analysis:   analysisPayload,
		History:    history,
		Message:    buildChatPromptWithAttachments(messageText, attachments),
	}

	resp, err := s.aiClient.Chat(ctx, aiReq)
	var replyText string
	if err != nil {
		log.Printf("[chat] ai-service FAILED question=%d: %v", questionID, err)
		replyText = buildFallbackReply(question, input.Message)
	} else {
		replyText = resp.Reply
	}

	assistantMessage := &models.ChatMessage{
		QuestionID: questionID,
		Role:       "assistant",
		Message:    replyText,
	}
	if err := s.chatRepo.Create(assistantMessage); err != nil {
		return nil, err
	}

	return s.chatRepo.ListByQuestionID(questionID)
}

func (s *QuestionService) saveChatAttachments(questionID int64, files []*multipart.FileHeader) ([]ChatAttachment, error) {
	if len(files) == 0 {
		return nil, nil
	}
	dir := filepath.Join("backend", "uploads", fmt.Sprintf("%d", questionID), "chat")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create chat attachment directory: %w", err)
	}

	attachments := make([]ChatAttachment, 0, len(files))
	for idx, fh := range files {
		if err := validateImageFile(fh); err != nil {
			return nil, err
		}
		src, err := fh.Open()
		if err != nil {
			return nil, fmt.Errorf("open chat attachment: %w", err)
		}
		defer src.Close()

		filename := fmt.Sprintf("%d_%d%s", time.Now().UnixNano(), idx, strings.ToLower(filepath.Ext(fh.Filename)))
		targetPath := filepath.Join(dir, filename)
		dst, err := os.Create(targetPath)
		if err != nil {
			return nil, fmt.Errorf("create chat attachment: %w", err)
		}
		if _, err := dst.ReadFrom(src); err != nil {
			dst.Close()
			return nil, fmt.Errorf("save chat attachment: %w", err)
		}
		if err := dst.Close(); err != nil {
			return nil, fmt.Errorf("close chat attachment: %w", err)
		}

		attachments = append(attachments, ChatAttachment{
			FileName:    fh.Filename,
			ContentType: fh.Header.Get("Content-Type"),
			FilePath:    targetPath,
			Size:        fh.Size,
		})
	}
	return attachments, nil
}

func buildChatPromptWithAttachments(message string, attachments []ChatAttachment) string {
	if len(attachments) == 0 {
		return message
	}
	lines := []string{message, "", "用户本次追问附加了图片，当前后端已保存附件，后续可交给多模态 agent 处理。附件列表："}
	for _, attachment := range attachments {
		lines = append(lines, fmt.Sprintf("- %s (%s)", attachment.FileName, attachment.FilePath))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (s *QuestionService) GetChatMessages(ctx context.Context, questionID int64) ([]models.ChatMessage, error) {
	return s.chatRepo.ListByQuestionID(questionID)
}

func (s *QuestionService) runOCR(ctx context.Context, question *models.Question, job *models.Job, fileHeader *multipart.FileHeader) error {
	now := time.Now()
	job.Status = "processing"
	job.StartedAt = &now
	if err := s.jobRepo.Update(job); err != nil {
		return err
	}

	question.OCRStatus = "processing"
	if err := s.questionRepo.Update(question); err != nil {
		return err
	}

	file, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("open uploaded file: %w", err)
	}
	defer file.Close()

	tempFile, err := os.CreateTemp("", "erro-question-*"+filepath.Ext(fileHeader.Filename))
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.ReadFrom(file); err != nil {
		tempFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	traceID := fmt.Sprintf("trace_ocr_%d", time.Now().UnixNano())
	resp, err := s.aiClient.ParseQuestionImage(ctx, question.ID, traceID, question.SourceType, tempPath)
	if err != nil {
		question.OCRStatus = "failed"
		_ = s.questionRepo.Update(question)
		markJobFailed(job, "OCR_FAILED", err.Error())
		_ = s.jobRepo.Update(job)
		return fmt.Errorf("parse question image by ai service: %w", err)
	}

	question.Stem = strings.TrimSpace(resp.StructuredQuestion.Stem)
	question.QuestionType = normalizeQuestionType(resp.StructuredQuestion.QuestionType)
	if !models.IsSupportedQuestionType(question.QuestionType) {
		question.QuestionType = models.QuestionTypeSubjective
	}
	question.RawOCRText = &resp.RawText
	warnings := resp.StructuredQuestion.Warnings
	if len(warnings) == 0 {
		warnings = resp.Warnings
	}
	if warningsJSON, err := json.Marshal(warnings); err == nil {
		value := string(warningsJSON)
		question.StructureWarnings = &value
	}
	question.StructureConfidence = resp.StructuredQuestion.Metadata.OCRConfidence
	if resp.StructuredQuestion.Metadata.ExtractionMethod != nil && strings.TrimSpace(*resp.StructuredQuestion.Metadata.ExtractionMethod) != "" {
		question.ParseSource = strings.TrimSpace(*resp.StructuredQuestion.Metadata.ExtractionMethod)
	} else {
		question.ParseSource = "rules"
	}
	question.OCRStatus = "completed"
	question.HasDiagram = resp.StructuredQuestion.HasDiagram
	if resp.StructuredQuestion.SuggestedAnswer != "" {
		answer := strings.TrimSpace(resp.StructuredQuestion.SuggestedAnswer)
		question.CorrectAnswer = &answer
	}
	if resp.StructuredQuestion.DiagramDescription != "" {
		desc := strings.TrimSpace(resp.StructuredQuestion.DiagramDescription)
		question.DiagramDescription = &desc
	}

	uploadsDir := filepath.Join("backend", "uploads", fmt.Sprintf("%d", question.ID))
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		return fmt.Errorf("create uploads directory: %w", err)
	}
	ext := filepath.Ext(fileHeader.Filename)
	persistPath := filepath.Join(uploadsDir, "original"+ext)
	src, err := os.Open(tempPath)
	if err != nil {
		return fmt.Errorf("open temp file for persist: %w", err)
	}
	defer src.Close()
	dst, err := os.Create(persistPath)
	if err != nil {
		return fmt.Errorf("create persist file: %w", err)
	}
	defer dst.Close()
	if _, err := dst.ReadFrom(src); err != nil {
		return fmt.Errorf("persist image file: %w", err)
	}
	question.ImagePath = &persistPath

	if err := s.questionRepo.Update(question); err != nil {
		return err
	}

	if err := s.questionRepo.ReplaceOptions(question.ID, toQuestionOptions(question.ID, resp.StructuredQuestion.Options)); err != nil {
		return err
	}

	var finalJob *models.Job
	if ocrTaskStatus(question) == "needs_review" {
		finalJob = markJobNeedsReview(job)
	} else {
		finalJob = markJobCompleted(job)
	}
	if err := s.jobRepo.Update(finalJob); err != nil {
		return err
	}

	return nil
}

func toQuestionDetail(question *models.Question) *QuestionDetail {
	warnings := parseWarnings(question.StructureWarnings)
	return &QuestionDetail{
		ID:                  question.ID,
		Stem:                question.Stem,
		QuestionType:        question.QuestionType,
		CorrectAnswer:       question.CorrectAnswer,
		UserAnswer:          question.UserAnswer,
		OCRStatus:           question.OCRStatus,
		AnalysisStatus:      question.AnalysisStatus,
		SourceType:          question.SourceType,
		RawOCRText:          question.RawOCRText,
		StructureWarnings:   warnings,
		StructureConfidence: question.StructureConfidence,
		ParseSource:         question.ParseSource,
		QualityStatus:       qualityStatus(warnings, question.StructureConfidence, question.ParseSource),
		CategoryID:          question.CategoryID,
		CategoryName:        categoryName(question.Category),
		DiagramDescription:  question.DiagramDescription,
		HasDiagram:          question.HasDiagram,
		ImagePath:           question.ImagePath,
		IsFavorited:         question.IsFavorited,
		Options:             question.Options,
		Assets:              question.Assets,
		Tags:                question.Tags,
	}
}

func (s *QuestionService) attachLearningState(detail *QuestionDetail) {
	if s.learningRepo == nil || detail == nil {
		return
	}
	state, err := s.learningRepo.GetByQuestionID(detail.ID)
	if err != nil || state == nil {
		return
	}
	detail.LearningState = toLearningStateDetail(state)
}

func toQuestionOptions(questionID int64, items []ai.OptionItem) []models.QuestionOption {
	options := make([]models.QuestionOption, 0, len(items))
	for idx, item := range items {
		options = append(options, models.QuestionOption{
			QuestionID: questionID,
			OptionKey:  strings.TrimSpace(item.Key),
			Content:    strings.TrimSpace(item.Content),
			SortOrder:  idx + 1,
		})
	}
	return options
}

func normalizeOptions(questionID int64, options []models.QuestionOption) []models.QuestionOption {
	result := make([]models.QuestionOption, 0, len(options))
	for idx, item := range options {
		result = append(result, models.QuestionOption{
			QuestionID: questionID,
			OptionKey:  strings.TrimSpace(item.OptionKey),
			Content:    strings.TrimSpace(item.Content),
			SortOrder:  idx + 1,
		})
	}
	return result
}

func toAIOptions(options []models.QuestionOption) []ai.OptionItem {
	result := make([]ai.OptionItem, 0, len(options))
	for _, option := range options {
		result = append(result, ai.OptionItem{
			Key:     option.OptionKey,
			Content: option.Content,
		})
	}
	return result
}

func toAIStructuredQuestion(question *models.Question, warnings []string) ai.StructuredQuestion {
	parseSource := strings.TrimSpace(question.ParseSource)
	var extractionMethod *string
	if parseSource != "" {
		extractionMethod = &parseSource
	}
	return ai.StructuredQuestion{
		Stem:               question.Stem,
		QuestionType:       question.QuestionType,
		Options:            toAIOptions(question.Options),
		Assets:             toAIAssets(question.Assets),
		SuggestedAnswer:    derefString(question.CorrectAnswer),
		RawText:            derefString(question.RawOCRText),
		HasDiagram:         question.HasDiagram,
		DiagramDescription: derefString(question.DiagramDescription),
		Warnings:           warnings,
		Metadata: ai.QuestionMetadata{
			SourceType:       question.SourceType,
			HasDiagram:       question.HasDiagram,
			OCRConfidence:    question.StructureConfidence,
			ImportMode:       "single_question",
			ExtractionMethod: extractionMethod,
		},
	}
}

func toAIAssets(assets []models.QuestionAsset) []any {
	result := make([]any, 0, len(assets))
	for _, asset := range assets {
		result = append(result, map[string]any{
			"assetType": asset.AssetType,
			"fileUrl":   asset.FileURL,
		})
	}
	return result
}

func buildChatHistory(allMessages []models.ChatMessage, excludeMsg *models.ChatMessage) []ai.ChatMessageItem {
	maxHistory := 10
	result := make([]ai.ChatMessageItem, 0, maxHistory)
	for _, msg := range allMessages {
		if excludeMsg != nil && msg.ID == excludeMsg.ID {
			continue
		}
		content := truncateRunes(msg.Message, 1000)
		result = append(result, ai.ChatMessageItem{Role: msg.Role, Content: content})
	}
	if len(result) > maxHistory {
		result = result[len(result)-maxHistory:]
	}
	return result
}

func buildFallbackReply(question *models.Question, message string) string {
	stem := truncateRunes(question.Stem, 80)
	return fmt.Sprintf("已收到你的追问：%s。当前 AI 服务暂时不可用，请稍后重试。题目《%s》的相关解析可在详情面板中查看。", strings.TrimSpace(message), stem)
}

// truncateRunes safely truncates a string to max runes without splitting multi-byte characters.
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func parseWarnings(value *string) []string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	var warnings []string
	if err := json.Unmarshal([]byte(*value), &warnings); err != nil {
		return nil
	}
	return warnings
}

func hasStructureWarning(warnings []string, target string) bool {
	for _, warning := range warnings {
		if warning == target {
			return true
		}
	}
	return false
}

func hasMissingOptionWarning(warnings []string) bool {
	for _, warning := range warnings {
		if strings.HasPrefix(warning, "missing_options_") {
			return true
		}
	}
	return false
}

func qualityStatus(warnings []string, confidence *float64, parseSource string) string {
	if strings.TrimSpace(parseSource) == "manual_corrected" && !hasStructureWarning(warnings, "options_incomplete") && !hasMissingOptionWarning(warnings) {
		return "ok"
	}
	if confidence != nil && *confidence < 0.72 {
		return "needs_review"
	}
	for _, warning := range warnings {
		if warning == "options_incomplete" ||
			strings.HasPrefix(warning, "missing_options_") ||
			warning == "llm_refine_failed" ||
			warning == "llm_refine_unavailable" ||
			warning == "ocr_low_confidence" {
			return "needs_review"
		}
	}
	return "ok"
}

func ocrTaskStatus(question *models.Question) string {
	if question == nil {
		return "failed"
	}
	if qualityStatus(parseWarnings(question.StructureWarnings), question.StructureConfidence, question.ParseSource) == "needs_review" {
		return "needs_review"
	}
	if strings.TrimSpace(question.OCRStatus) == "" {
		return "pending"
	}
	return question.OCRStatus
}

func hasObviousStructureIssue(questionType string, options []models.QuestionOption) bool {
	questionType = normalizeQuestionType(questionType)
	if !models.RequiresOptions(questionType) {
		return false
	}
	return len(options) == 0
}

func toLearningStateDetail(state *models.QuestionLearningState) *LearningStateDetail {
	if state == nil {
		return nil
	}
	return &LearningStateDetail{
		ID:              state.ID,
		QuestionID:      state.QuestionID,
		MasteryLevel:    state.MasteryLevel,
		WrongCount:      state.WrongCount,
		CorrectStreak:   state.CorrectStreak,
		LastPracticedAt: state.LastPracticedAt,
		NextReviewAt:    state.NextReviewAt,
		MistakeReason:   derefString(state.MistakeReason),
		WeaknessTags:    unmarshalStringSlice(state.WeaknessTags),
		ReviewAdvice:    unmarshalStringSlice(state.ReviewAdvice),
		CreatedAt:       state.CreatedAt,
		UpdatedAt:       state.UpdatedAt,
	}
}

func marshalStringSlice(values []string) string {
	payload, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(payload)
}

func unmarshalStringSlice(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(value), &items); err != nil {
		return []string{}
	}
	return items
}

func cleanStringSlice(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func buildMistakeReason(question *models.Question, analysis *models.Analysis) string {
	if question.UserAnswer != nil && question.CorrectAnswer != nil && strings.TrimSpace(*question.UserAnswer) != "" {
		return fmt.Sprintf("本题曾作答为「%s」，标准答案为「%s」，建议重点复盘答案差异和题干条件。", strings.TrimSpace(*question.UserAnswer), strings.TrimSpace(*question.CorrectAnswer))
	}
	if analysis != nil {
		var content ai.AnalysisPayload
		if err := json.Unmarshal([]byte(analysis.ContentJSON), &content); err == nil && len(content.Pitfalls) > 0 {
			return "主要易错点：" + strings.Join(content.Pitfalls, "；")
		}
	}
	return "建议结合题干、标准答案和解析步骤，记录本题出错的具体环节。"
}

func inferWeaknessTags(question *models.Question, analysis *models.Analysis) []string {
	tags := []string{questionTypeLabelForLearning(question.QuestionType)}
	if analysis != nil {
		var content ai.AnalysisPayload
		if err := json.Unmarshal([]byte(analysis.ContentJSON), &content); err == nil {
			tags = append(tags, content.KnowledgePoints...)
			if len(content.Pitfalls) > 0 {
				tags = append(tags, "易错点")
			}
		}
	}
	return cleanStringSlice(tags)
}

func buildReviewAdvice(question *models.Question, tags []string) []string {
	advice := []string{"先复述题干条件，再独立重做一次。"}
	if len(tags) > 0 {
		advice = append(advice, "围绕「"+tags[0]+"」补一题同类练习。")
	}
	if models.IsObjectiveQuestionType(normalizeQuestionType(question.QuestionType)) {
		advice = append(advice, "核对每个选项的排除理由，避免只记答案。")
	} else {
		advice = append(advice, "保留关键步骤，主观题后续需要人工或 AI 辅助复核。")
	}
	return advice
}

func questionTypeLabelForLearning(questionType string) string {
	switch normalizeQuestionType(questionType) {
	case models.QuestionTypeSingleChoice:
		return "单选题"
	case models.QuestionTypeMultipleChoice:
		return "多选题"
	case models.QuestionTypeTrueFalse:
		return "判断题"
	case models.QuestionTypeFillBlank:
		return "填空题"
	case models.QuestionTypeShortAnswer:
		return "简答题"
	case models.QuestionTypeEssay:
		return "解答题"
	case models.QuestionTypeCalculation:
		return "计算题"
	default:
		return "主观题"
	}
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeQuestionType(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "", "unknown":
		return models.QuestionTypeSubjective
	case "choice", "single", "radio", "单选", "单选题":
		return models.QuestionTypeSingleChoice
	case "multi_choice", "multiple", "multi", "checkbox", "多选", "多选题":
		return models.QuestionTypeMultipleChoice
	case "judge", "judgement", "judgment", "boolean", "判断", "判断题":
		return models.QuestionTypeTrueFalse
	case "blank", "fill", "fill_in_blank", "填空", "填空题":
		return models.QuestionTypeFillBlank
	case "short", "short_answer", "简答", "简答题":
		return models.QuestionTypeShortAnswer
	case "作文", "论述", "论述题":
		return models.QuestionTypeEssay
	case "compute", "math_calculation", "计算", "计算题":
		return models.QuestionTypeCalculation
	default:
		return normalized
	}
}

func newJobID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func markJobCompleted(job *models.Job) *models.Job {
	now := time.Now()
	job.Status = "completed"
	job.FinishedAt = &now
	job.ErrorCode = nil
	job.ErrorMessage = nil
	return job
}

func markJobNeedsReview(job *models.Job) *models.Job {
	now := time.Now()
	job.Status = "needs_review"
	job.FinishedAt = &now
	job.ErrorCode = nil
	job.ErrorMessage = nil
	return job
}

func markJobFailed(job *models.Job, code string, message string) *models.Job {
	now := time.Now()
	job.Status = "failed"
	job.FinishedAt = &now
	job.ErrorCode = &code
	job.ErrorMessage = &message
	return job
}

func categoryName(category *models.Category) *string {
	if category == nil {
		return nil
	}
	return &category.Name
}
