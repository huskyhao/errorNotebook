package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"erro-notebook/backend/internal/integrations/ai"
	"erro-notebook/backend/internal/models"
	"erro-notebook/backend/internal/repository"
	"erro-notebook/backend/internal/storage"
)

var ErrQuestionNotFound = errors.New("question not found")
var ErrInvalidCategory = errors.New("category must be an existing top-level subject category")
var ErrAIUnavailable = errors.New("ai service unavailable")
var ErrTaxonomySuggestionUnavailable = errors.New("taxonomy suggestion unavailable or has no applicable candidates")
var ErrProposalNotFound = errors.New("ai proposal not found")
var ErrProposalConflict = errors.New("ai proposal is not applicable")

type QuestionService struct {
	questionRepo  *repository.QuestionRepository
	jobRepo       *repository.JobRepository
	analysisRepo  *repository.AnalysisRepository
	chatRepo      *repository.ChatRepository
	batchRepo     *repository.BatchRepository
	learningRepo  *repository.LearningStateRepository
	categoryRepo  *repository.CategoryRepository
	tagRepo       *repository.TagRepository
	aiClient      *ai.Client
	aiClientAsync *ai.Client
	objectStorage storage.ObjectStorage
	proposalRepo  *repository.AIProposalRepository
}

func (s *QuestionService) listCategories(ctx context.Context) ([]models.Category, error) {
	return s.categoryRepo.List()
}

func (s *QuestionService) listTags(ctx context.Context) ([]models.Tag, error) {
	return s.tagRepo.List()
}

// AuthorizeQuestion is retained as a simple existence check for callers that
// want an early not-found response before operating on a question.
func (s *QuestionService) AuthorizeQuestion(ctx context.Context, id int64) error {
	question, err := s.questionRepo.GetByID(id)
	if err != nil {
		return err
	}
	if question == nil {
		return ErrQuestionNotFound
	}
	return nil
}

func NewQuestionService(
	questionRepo *repository.QuestionRepository,
	jobRepo *repository.JobRepository,
	analysisRepo *repository.AnalysisRepository,
	chatRepo *repository.ChatRepository,
	batchRepo *repository.BatchRepository,
	learningRepo *repository.LearningStateRepository,
	categoryRepo *repository.CategoryRepository,
	tagRepo *repository.TagRepository,
	aiClient *ai.Client,
	aiClientAsync *ai.Client,
	objectStorage storage.ObjectStorage,
	proposalRepos ...*repository.AIProposalRepository,
) *QuestionService {
	var proposalRepo *repository.AIProposalRepository
	if len(proposalRepos) > 0 {
		proposalRepo = proposalRepos[0]
	}
	return &QuestionService{
		questionRepo:  questionRepo,
		jobRepo:       jobRepo,
		analysisRepo:  analysisRepo,
		chatRepo:      chatRepo,
		batchRepo:     batchRepo,
		learningRepo:  learningRepo,
		categoryRepo:  categoryRepo,
		tagRepo:       tagRepo,
		aiClient:      aiClient,
		aiClientAsync: aiClientAsync,
		objectStorage: objectStorage,
		proposalRepo:  proposalRepo,
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
	CategoryIDSet bool
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
	JobID           string `json:"jobId,omitempty"`
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

	batch := &models.BatchImport{TotalFiles: len(input.Files)}
	if err := s.batchRepo.CreateBatch(batch); err != nil {
		return nil, err
	}

	items := make([]models.BatchImportItem, len(input.Files))
	for i, fh := range input.Files {
		question := &models.Question{
			Stem:           "",
			QuestionType:   "subjective",
			OCRStatus:      "queued",
			AnalysisStatus: "queued",
			SourceType:     strings.TrimSpace(input.SourceType),
		}
		if question.SourceType == "" {
			question.SourceType = "image"
		}
		if err := s.questionRepo.Create(question); err != nil {
			return nil, err
		}
		objectKey, err := s.storeUploadedImage(ctx, question.ID, fh)
		if err != nil {
			question.OCRStatus = "failed"
			_ = s.questionRepo.Update(question)
			return nil, fmt.Errorf("store uploaded image: %w", err)
		}
		question.ImagePath = &objectKey
		if err := s.questionRepo.Update(question); err != nil {
			return nil, err
		}
		job := &models.Job{
			JobID: newJobID("ocr"), QuestionID: question.ID, JobType: "ocr",
			Status: "queued", MaxAttempts: 3, ProcessingStage: "queued",
		}
		if err := s.jobRepo.Create(job); err != nil {
			return nil, err
		}
		items[i] = models.BatchImportItem{
			BatchID:         batch.ID,
			QuestionID:      question.ID,
			JobID:           job.JobID,
			ObjectKey:       objectKey,
			FileIndex:       i,
			FileName:        fh.Filename,
			Status:          "queued",
			ProcessingStage: "queued",
		}
	}
	if err := s.batchRepo.CreateItems(items); err != nil {
		return nil, err
	}

	result := &BatchImportResult{
		BatchID: batch.ID,
		Total:   len(input.Files),
	}
	for _, item := range items {
		if item.Status == "pending" {
			item.Status = "queued"
		}
		result.Questions = append(result.Questions, BatchImportItemResult{
			FileIndex:       item.FileIndex,
			FileName:        item.FileName,
			QuestionID:      item.QuestionID,
			JobID:           item.JobID,
			Status:          item.Status,
			ProcessingStage: item.ProcessingStage,
		})
	}
	return result, nil
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
			JobID:           item.JobID,
			Status:          item.Status,
			ProcessingStage: item.ProcessingStage,
			Error:           errMsg,
		})
	}
	return result, nil
}

type CreateChatMessageInput struct {
	Message         string                  `json:"message"`
	IdempotencyKey  string                  `json:"idempotencyKey"`
	AttachmentFiles []*multipart.FileHeader `json:"-"`
}

type ChatAttachment struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	FilePath    string `json:"filePath"`
	Size        int64  `json:"size"`
}

const maxChatAttachments = 4
const maxImageBytes = 8 * 1024 * 1024

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
		Stem:           strings.TrimSpace(input.RawText),
		QuestionType:   "subjective",
		OCRStatus:      "queued",
		AnalysisStatus: "queued",
		SourceType:     sourceType,
	}
	if sourceType == "manual" {
		question.OCRStatus = "completed"
	}
	if err := s.questionRepo.Create(question); err != nil {
		return nil, err
	}

	if sourceType == "manual" {
		return &ImportQuestionResult{
			QuestionID: question.ID,
			Status:     "completed",
		}, nil
	}

	objectKey, err := s.storeUploadedImage(ctx, question.ID, input.FileHeader)
	if err != nil {
		return nil, err
	}
	question.ImagePath = &objectKey
	if err := s.questionRepo.Update(question); err != nil {
		return nil, err
	}
	job := &models.Job{
		JobID: newJobID("ocr"), QuestionID: question.ID, JobType: "ocr",
		Status: "queued", MaxAttempts: 3, ProcessingStage: "queued",
	}
	if err := s.jobRepo.Create(job); err != nil {
		return nil, err
	}

	return &ImportQuestionResult{
		JobID:      job.JobID,
		QuestionID: question.ID,
		Status:     "queued",
	}, nil
}

func (s *QuestionService) storeUploadedImage(ctx context.Context, questionID int64, fileHeader *multipart.FileHeader) (string, error) {
	if s.objectStorage == nil {
		return "", fmt.Errorf("object storage is not configured")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("open uploaded image: %w", err)
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext == "" {
		ext = ".bin"
	}
	key := filepath.ToSlash(filepath.Join("questions", fmt.Sprintf("%d", questionID), "original"+ext))
	if err := s.objectStorage.Put(ctx, key, file); err != nil {
		return "", err
	}
	return key, nil
}

func validateImageFile(fileHeader *multipart.FileHeader) error {
	if fileHeader == nil {
		return fmt.Errorf("image file is required")
	}
	contentType := strings.ToLower(strings.TrimSpace(fileHeader.Header.Get("Content-Type")))
	if fileHeader.Size > maxImageBytes {
		return fmt.Errorf("image must not exceed 8 MiB")
	}
	allowed := map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true, "image/bmp": true}
	if allowed[contentType] {
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
	categoryWasChanged := input.CategoryIDSet || input.CategoryID != nil
	if categoryWasChanged {
		if input.CategoryID != nil {
			if s.categoryRepo != nil {
				category, categoryErr := s.categoryRepo.GetByID(*input.CategoryID)
				if categoryErr != nil {
					return nil, categoryErr
				}
				if category == nil || category.ParentID != nil {
					return nil, ErrInvalidCategory
				}
			}
		}
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

	if question.ImagePath != nil && s.objectStorage != nil {
		if removeErr := s.objectStorage.Delete(ctx, *question.ImagePath); removeErr != nil {
			log.Printf("[storage] delete question image failed question=%d: %v", id, removeErr)
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
	if active, err := s.jobRepo.FindActiveByQuestionIDAndType(id, "analyze"); err != nil {
		return nil, err
	} else if active != nil {
		return &AnalyzeQuestionResult{JobID: active.JobID, QuestionID: id, Status: active.Status}, nil
	}

	job := &models.Job{
		JobID: newJobID("analyze"), QuestionID: id, JobType: "analyze",
		Status: "queued", MaxAttempts: 3, ProcessingStage: "queued",
	}
	if err := s.jobRepo.Create(job); err != nil {
		return nil, err
	}

	question.AnalysisStatus = "queued"
	if err := s.questionRepo.Update(question); err != nil {
		return nil, err
	}

	return &AnalyzeQuestionResult{
		JobID:      job.JobID,
		QuestionID: id,
		Status:     "queued",
	}, nil
}

func (s *QuestionService) RetryOCR(ctx context.Context, id int64) (*AnalyzeQuestionResult, error) {
	question, err := s.questionRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}
	job, err := s.jobRepo.GetLatestByQuestionIDAndType(id, "ocr")
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("ocr job not found")
	}
	if job.Status != "failed" {
		return &AnalyzeQuestionResult{JobID: job.JobID, QuestionID: id, Status: job.Status}, nil
	}
	job.Attempts = 0
	if err := s.jobRepo.ResetForRetry(job); err != nil {
		return nil, err
	}
	question.OCRStatus = "queued"
	question.AnalysisStatus = "queued"
	if err := s.questionRepo.Update(question); err != nil {
		return nil, err
	}
	return &AnalyzeQuestionResult{JobID: job.JobID, QuestionID: id, Status: "queued"}, nil
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
	content["taxonomySuggestionMeta"] = map[string]any{
		"source": analysis.Provider, "sourceQuestionFingerprint": analysis.SourceQuestionFingerprint,
		"candidateSnapshot": analysis.TaxonomyCandidateSnapshotJSON, "generatedAt": analysis.GeneratedAt,
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
	state, err := s.learningRepo.Ensure(questionID)
	if err != nil {
		return nil, err
	}

	if s.aiClient == nil {
		return nil, ErrAIUnavailable
	}
	analysis, _ := s.analysisRepo.GetLatestByQuestionID(questionID)
	var analysisPayload *ai.AnalysisPayload
	if analysis != nil {
		var payload ai.AnalysisPayload
		if json.Unmarshal([]byte(analysis.ContentJSON), &payload) == nil {
			analysisPayload = &payload
		}
	}
	categoryCandidates, _ := s.listCategories(ctx)
	tagCandidates, _ := s.listTags(ctx)
	conversation, _ := s.chatRepo.ListByQuestionID(questionID)
	fingerprint := questionContentFingerprint(question)
	resp, err := s.aiClient.AgentAction(ctx, ai.AgentActionRequest{
		TraceID: fmt.Sprintf("trace_agent_%d", time.Now().UnixNano()), QuestionID: questionID,
		Action: "diagnose_mistake", Context: ai.AgentContext{
			Question:        toAIStructuredQuestion(question, parseWarnings(question.StructureWarnings)),
			Warnings:        parseWarnings(question.StructureWarnings),
			ReferenceAnswer: derefString(question.CorrectAnswer), ReferenceAnswerSource: "question.correctAnswer",
			LatestAnswer: derefString(question.UserAnswer), Analysis: analysisPayload,
			Conversation: toAIConversation(conversation), CategoryCandidates: categoryNamesForAI(categoryCandidates),
			TagCandidates: tagNames(tagCandidates), ContentFingerprint: fingerprint, Version: fingerprint,
		}, Params: map[string]any{},
	})
	if err != nil || resp == nil || resp.Status != "completed" {
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrAIUnavailable, err)
		}
		return nil, fmt.Errorf("%w: agent status %s", ErrAIUnavailable, resp.Status)
	}
	var diagnosis ai.DiagnoseMistakeResult
	if err := json.Unmarshal(resp.Result, &diagnosis); err != nil {
		return nil, fmt.Errorf("%w: invalid diagnosis result", ErrAIUnavailable)
	}
	latest, err := s.questionRepo.GetByID(questionID)
	if err != nil || latest == nil || questionContentFingerprint(latest) != fingerprint {
		return nil, fmt.Errorf("%w: question changed while diagnosis was running", ErrAIUnavailable)
	}
	if diagnosis.ReasonType != "证据不足" && strings.TrimSpace(diagnosis.MistakeReason) != "" && len(diagnosis.Evidence) > 0 {
		reason := strings.TrimSpace(diagnosis.MistakeReason)
		state.MistakeReason = &reason
		state.WeaknessTags = marshalStringSlice(cleanStringSlice(diagnosis.WeaknessTags))
		state.ReviewAdvice = marshalStringSlice(cleanStringSlice(diagnosis.ReviewAdvice))
	}
	if err := s.learningRepo.Save(state); err != nil {
		return nil, err
	}
	return toLearningStateDetail(state), nil
}

type ApplyTaxonomyResult struct {
	QuestionID int64   `json:"questionId"`
	CategoryID *int64  `json:"categoryId,omitempty"`
	TagIDs     []int64 `json:"tagIds"`
}

func (s *QuestionService) ApplyTaxonomySuggestion(ctx context.Context, questionID int64) (*QuestionDetail, error) {
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}
	analysis, err := s.analysisRepo.GetLatestByQuestionID(questionID)
	if err != nil || analysis == nil {
		return nil, ErrTaxonomySuggestionUnavailable
	}
	if analysis.SourceQuestionFingerprint != "" && analysis.SourceQuestionFingerprint != questionContentFingerprint(question) {
		return nil, fmt.Errorf("%w: question changed since suggestion was generated", ErrProposalConflict)
	}
	// A manually selected taxonomy (and an already-applied suggestion) is the
	// source of truth. Confirmation is intentionally idempotent and never
	// overwrites an existing choice.
	if question.CategoryID != nil || len(question.Tags) > 0 {
		return s.GetQuestion(ctx, questionID)
	}
	var payload ai.AnalysisPayload
	if err := json.Unmarshal([]byte(analysis.ContentJSON), &payload); err != nil || payload.TaxonomySuggestion == nil {
		return nil, ErrTaxonomySuggestionUnavailable
	}
	suggestion := s.validateTaxonomySuggestion(payload.TaxonomySuggestion)
	if suggestion == nil || suggestion.CategoryID == nil && len(suggestion.TagIDs) == 0 {
		return nil, ErrTaxonomySuggestionUnavailable
	}
	if suggestion.UnresolvedCategory || len(suggestion.UnresolvedTagNames) > 0 {
		return nil, fmt.Errorf("%w: one or more suggested taxonomy candidates no longer exist", ErrProposalConflict)
	}
	if err := s.questionRepo.ApplyTaxonomy(questionID, suggestion.CategoryID, suggestion.TagIDs); err != nil {
		return nil, err
	}
	return s.GetQuestion(ctx, questionID)
}

func (s *QuestionService) RunAgentAction(ctx context.Context, questionID int64, action string, params map[string]any) (*ai.AgentActionResponse, error) {
	if action != "diagnose_mistake" && action != "explain_alternative" && action != "hint" && action != "suggest_taxonomy" && action != "generate_similar_question" && action != "grade_subjective_answer" {
		return nil, fmt.Errorf("unsupported agent action: %s", action)
	}
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}
	if s.aiClient == nil {
		return nil, ErrAIUnavailable
	}
	analysis, _ := s.analysisRepo.GetLatestByQuestionID(questionID)
	var analysisPayload *ai.AnalysisPayload
	if analysis != nil {
		var payload ai.AnalysisPayload
		if json.Unmarshal([]byte(analysis.ContentJSON), &payload) == nil {
			analysisPayload = &payload
		}
	}
	categories, _ := s.listCategories(ctx)
	tags, _ := s.listTags(ctx)
	messages, _ := s.chatRepo.ListByQuestionID(questionID)
	fingerprint := questionContentFingerprint(question)
	resp, err := s.aiClient.AgentAction(ctx, ai.AgentActionRequest{
		TraceID: fmt.Sprintf("trace_agent_%d", time.Now().UnixNano()), QuestionID: questionID, Action: action,
		Context: ai.AgentContext{
			Question: toAIStructuredQuestion(question, parseWarnings(question.StructureWarnings)),
			Warnings: parseWarnings(question.StructureWarnings), ReferenceAnswer: derefString(question.CorrectAnswer),
			ReferenceAnswerSource: "question.correctAnswer", LatestAnswer: derefString(question.UserAnswer),
			Analysis: analysisPayload, Conversation: toAIConversation(messages), CategoryCandidates: categoryNamesForAI(categories),
			TagCandidates: tagNames(tags), ContentFingerprint: fingerprint, Version: fingerprint,
		}, Params: params,
	})
	if err != nil {
		return nil, err
	}
	if s.proposalRepo != nil && (action == "generate_similar_question" || action == "grade_subjective_answer") && resp != nil && len(resp.Result) > 0 && (resp.Status == "completed" || resp.Status == "needs_review") {
		var envelope struct {
			ProposalID string `json:"proposalId"`
		}
		_ = json.Unmarshal(resp.Result, &envelope)
		proposalID := envelope.ProposalID
		if proposalID == "" {
			proposalID = fmt.Sprintf("proposal_%d_%d", questionID, time.Now().UnixNano())
		}
		key := fmt.Sprintf("%d:%s:%s", questionID, action, fingerprint)
		proposal, saveErr := s.proposalRepo.CreateOrGet(&models.AIProposal{ProposalID: proposalID, QuestionID: questionID, Action: action, Status: "pending", ContentJSON: string(resp.Result), SourceFingerprint: fingerprint, IdempotencyKey: key, ExpiresAt: time.Now().Add(24 * time.Hour)})
		if saveErr != nil {
			return nil, saveErr
		}
		if proposal != nil {
			proposalID = proposal.ProposalID
		}
		var resultMap map[string]any
		if json.Unmarshal(resp.Result, &resultMap) == nil {
			resultMap["proposalId"] = proposalID
			resp.Result, _ = json.Marshal(resultMap)
		}
	}
	return resp, nil
}

func (s *QuestionService) GetAIProposal(ctx context.Context, proposalID string) (*models.AIProposal, error) {
	if s.proposalRepo == nil {
		return nil, ErrProposalNotFound
	}
	return s.proposalRepo.Get(proposalID)
}

func (s *QuestionService) ConfirmSimilarQuestion(ctx context.Context, questionID int64, proposalID string) (*QuestionDetail, error) {
	if s.proposalRepo == nil {
		return nil, ErrProposalNotFound
	}
	proposal, err := s.proposalRepo.Get(proposalID)
	if err != nil {
		return nil, err
	}
	if proposal == nil || proposal.QuestionID != questionID || proposal.Action != "generate_similar_question" {
		return nil, ErrProposalConflict
	}
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}
	fingerprint := questionContentFingerprint(question)
	createdID, err := s.proposalRepo.ConfirmSimilar(proposalID, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProposalConflict, err)
	}
	created, err := s.questionRepo.GetByID(createdID)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, ErrProposalConflict
	}
	return s.GetQuestion(ctx, createdID)
}

func (s *QuestionService) RejectAIProposal(ctx context.Context, proposalID string, questionIDs ...int64) error {
	if s.proposalRepo == nil {
		return ErrProposalNotFound
	}
	if len(questionIDs) > 0 {
		proposal, err := s.proposalRepo.Get(proposalID)
		if err != nil {
			return err
		}
		if proposal == nil || proposal.QuestionID != questionIDs[0] {
			return ErrProposalConflict
		}
	}
	return s.proposalRepo.Reject(proposalID)
}

func (s *QuestionService) CreateChatMessage(ctx context.Context, questionID int64, input CreateChatMessageInput) ([]models.ChatMessage, error) {
	question, err := s.questionRepo.GetByID(questionID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrQuestionNotFound
	}
	var existing *models.ChatMessage
	if strings.TrimSpace(input.IdempotencyKey) != "" && s.chatRepo != nil {
		existing, err = s.chatRepo.GetByIdempotencyKey(questionID, strings.TrimSpace(input.IdempotencyKey))
		if err != nil {
			return nil, err
		}
		if existing != nil {
			messages, listErr := s.chatRepo.ListByQuestionID(questionID)
			if listErr != nil {
				return nil, listErr
			}
			for _, message := range messages {
				if message.ID > existing.ID && message.Role == "assistant" {
					return messages, nil
				}
			}
		}
	}

	attachments, err := s.saveChatAttachments(ctx, questionID, input.AttachmentFiles)
	if err != nil {
		return nil, err
	}
	if existing != nil && len(attachments) == 0 && existing.AttachmentJSON != nil {
		_ = json.Unmarshal([]byte(*existing.AttachmentJSON), &attachments)
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

	userMessage := existing
	if userMessage == nil {
		userMessage = &models.ChatMessage{QuestionID: questionID, Role: "user", Message: messageText, AttachmentJSON: attachmentJSON}
		if strings.TrimSpace(input.IdempotencyKey) != "" {
			key := strings.TrimSpace(input.IdempotencyKey)
			userMessage.IdempotencyKey = &key
		}
		if err := s.chatRepo.Create(userMessage); err != nil {
			return nil, err
		}
	} else {
		messageText = userMessage.Message
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

	if s.aiClient == nil {
		return nil, ErrAIUnavailable
	}
	var resp *ai.ChatResponse
	if len(attachments) > 0 {
		images := make([]ai.ChatImage, 0, len(attachments))
		for _, attachment := range attachments {
			reader, openErr := s.openQuestionImage(ctx, attachment.FilePath)
			if openErr != nil {
				return nil, fmt.Errorf("%w: attachment object unavailable", ErrAIUnavailable)
			}
			data, readErr := io.ReadAll(io.LimitReader(reader, maxImageBytes+1))
			reader.Close()
			if readErr != nil || len(data) == 0 || len(data) > maxImageBytes {
				return nil, fmt.Errorf("%w: invalid attachment bytes", ErrAIUnavailable)
			}
			images = append(images, ai.ChatImage{FileName: attachment.FileName, ContentType: attachment.ContentType, Bytes: data})
		}
		resp, err = s.aiClient.ChatWithImages(ctx, aiReq, images)
	} else {
		resp, err = s.aiClient.Chat(ctx, aiReq)
	}
	var replyText string
	if err != nil {
		log.Printf("[chat] ai-service FAILED question=%d: %v", questionID, err)
		return nil, fmt.Errorf("%w: %v", ErrAIUnavailable, err)
	}
	replyText = resp.Reply

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

func (s *QuestionService) saveChatAttachments(ctx context.Context, questionID int64, files []*multipart.FileHeader) ([]ChatAttachment, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if len(files) > maxChatAttachments {
		return nil, fmt.Errorf("at most %d chat images are allowed", maxChatAttachments)
	}
	if s.objectStorage == nil {
		return nil, fmt.Errorf("object storage is not configured")
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
		data, contentType, readErr := readValidatedImage(src, fh.Header.Get("Content-Type"))
		src.Close()
		if readErr != nil {
			return nil, readErr
		}

		filename := fmt.Sprintf("%d_%d%s", time.Now().UnixNano(), idx, strings.ToLower(filepath.Ext(fh.Filename)))
		objectKey := filepath.ToSlash(filepath.Join("questions", fmt.Sprintf("%d", questionID), "chat", filename))
		if err := s.objectStorage.Put(ctx, objectKey, bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("save chat attachment: %w", err)
		}

		attachments = append(attachments, ChatAttachment{
			FileName:    fh.Filename,
			ContentType: contentType,
			FilePath:    objectKey,
			Size:        fh.Size,
		})
	}
	return attachments, nil
}

func readValidatedImage(reader io.Reader, declared string) ([]byte, string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxImageBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read image: %w", err)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("image is empty")
	}
	if len(data) > maxImageBytes {
		return nil, "", fmt.Errorf("image exceeds 8 MiB")
	}
	detected := http.DetectContentType(data)
	allowed := map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true, "image/bmp": true}
	if !allowed[detected] {
		return nil, "", fmt.Errorf("image bytes are not a supported image")
	}
	declared = strings.ToLower(strings.TrimSpace(declared))
	if declared != "" && declared != "application/octet-stream" && declared != detected {
		return nil, "", fmt.Errorf("declared MIME does not match image bytes")
	}
	return data, detected, nil
}

func buildChatPromptWithAttachments(message string, attachments []ChatAttachment) string {
	if len(attachments) == 0 {
		return message
	}
	lines := []string{message, "", "用户本次追问附加了图片；图片内容将作为实际视觉输入，以下仅为不可信文件元数据："}
	for _, attachment := range attachments {
		lines = append(lines, fmt.Sprintf("- %s (%s, %d bytes)", attachment.FileName, attachment.ContentType, attachment.Size))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (s *QuestionService) GetChatMessages(ctx context.Context, questionID int64) ([]models.ChatMessage, error) {
	return s.chatRepo.ListByQuestionID(questionID)
}

func (s *QuestionService) runOCRObject(ctx context.Context, question *models.Question, job *models.Job) error {
	if s.objectStorage == nil || question.ImagePath == nil || strings.TrimSpace(*question.ImagePath) == "" {
		return fmt.Errorf("source image is unavailable")
	}
	reader, err := s.openQuestionImage(ctx, *question.ImagePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	tempFile, err := os.CreateTemp("", "erro-question-*")
	if err != nil {
		return fmt.Errorf("create ai input: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if _, err := io.Copy(tempFile, reader); err != nil {
		tempFile.Close()
		return fmt.Errorf("materialize source image: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close ai input: %w", err)
	}

	question.OCRStatus = "processing"
	job.ProcessingStage = "ocr"
	if err := s.questionRepo.Update(question); err != nil {
		return err
	}
	traceID := fmt.Sprintf("trace_ocr_%d", time.Now().UnixNano())
	resp, err := s.aiClient.ParseQuestionImage(ctx, question.ID, traceID, question.SourceType, tempPath)
	if err != nil {
		return fmt.Errorf("parse question image by ai service: %w", err)
	}
	if strings.TrimSpace(resp.RawText) == "" && strings.TrimSpace(resp.StructuredQuestion.Stem) == "" {
		return fmt.Errorf("ocr returned an empty result")
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
	if warningsJSON, marshalErr := json.Marshal(warnings); marshalErr == nil {
		value := string(warningsJSON)
		question.StructureWarnings = &value
	}
	question.StructureConfidence = resp.StructuredQuestion.Metadata.OCRConfidence
	question.ParseSource = "rules"
	if resp.StructuredQuestion.Metadata.ExtractionMethod != nil && strings.TrimSpace(*resp.StructuredQuestion.Metadata.ExtractionMethod) != "" {
		question.ParseSource = strings.TrimSpace(*resp.StructuredQuestion.Metadata.ExtractionMethod)
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
	if err := s.questionRepo.Update(question); err != nil {
		return err
	}
	if err := s.questionRepo.ReplaceOptions(question.ID, toQuestionOptions(question.ID, resp.StructuredQuestion.Options)); err != nil {
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
	if warnings == nil {
		warnings = make([]string, 0)
	}
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

func toAIConversation(messages []models.ChatMessage) []ai.ChatMessageItem {
	result := make([]ai.ChatMessageItem, 0, 10)
	for _, message := range messages {
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}
		result = append(result, ai.ChatMessageItem{Role: message.Role, Content: truncateRunes(message.Message, 1000)})
	}
	if len(result) > 10 {
		result = result[len(result)-10:]
	}
	return result
}

func categoryNames(categories []models.Category) []string {
	result := make([]string, 0, len(categories))
	for _, category := range categories {
		if category.ParentID == nil {
			result = append(result, category.Name)
		}
	}
	return result
}

// categoryNamesForAI keeps automatic classification at the broad-subject level.
func categoryNamesForAI(categories []models.Category) []string {
	result := make([]string, 0, len(categories))
	seen := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		name := strings.TrimSpace(category.Name)
		if category.ParentID != nil || name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, name)
	}
	return result
}

func tagNames(tags []models.Tag) []string {
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		result = append(result, tag.Name)
	}
	return result
}

func questionContentFingerprint(question *models.Question) string {
	payload := struct {
		Stem          string
		QuestionType  string
		CorrectAnswer string
		UserAnswer    string
		Options       []models.QuestionOption
	}{question.Stem, question.QuestionType, derefString(question.CorrectAnswer), derefString(question.UserAnswer), question.Options}
	data, _ := json.Marshal(payload)
	digest := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", digest[:])
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

// validateTaxonomySuggestion resolves AI-proposed names against the business
// taxonomy without changing the question. Unmatched names remain visible so
// the UI can explain which parts were not applicable.
func (s *QuestionService) validateTaxonomySuggestion(input *ai.TaxonomySuggestion) *ai.TaxonomySuggestion {
	if input == nil {
		return nil
	}
	suggestion := *input
	suggestion.CategoryName = strings.TrimSpace(suggestion.CategoryName)
	suggestion.TagNames = cleanStringSlice(suggestion.TagNames)
	if len(suggestion.TagNames) > 3 {
		suggestion.TagNames = suggestion.TagNames[:3]
	}
	suggestion.CategoryID = nil
	suggestion.TagIDs = nil
	suggestion.UnresolvedTagNames = nil
	suggestion.UnresolvedCategory = false
	if suggestion.Confidence != nil {
		confidence := *suggestion.Confidence
		if confidence < 0 {
			confidence = 0
		}
		if confidence > 1 {
			confidence = 1
		}
		suggestion.Confidence = &confidence
	}

	if s.categoryRepo != nil && suggestion.CategoryName != "" {
		categories, err := s.categoryRepo.List()
		if err == nil {
			for _, category := range categories {
				if category.ParentID == nil && strings.EqualFold(category.Name, suggestion.CategoryName) {
					categoryID := category.ID
					suggestion.CategoryID = &categoryID
					break
				}
			}
		}
		if suggestion.CategoryID == nil {
			suggestion.UnresolvedCategory = true
		}
	}

	if s.tagRepo != nil && len(suggestion.TagNames) > 0 {
		tags, err := s.tagRepo.List()
		if err == nil {
			byName := make(map[string]int64, len(tags))
			for _, tag := range tags {
				byName[strings.ToLower(strings.TrimSpace(tag.Name))] = tag.ID
			}
			for _, name := range suggestion.TagNames {
				if tagID, ok := byName[strings.ToLower(name)]; ok {
					suggestion.TagIDs = append(suggestion.TagIDs, tagID)
				} else {
					suggestion.UnresolvedTagNames = append(suggestion.UnresolvedTagNames, name)
				}
			}
		}
	}

	return &suggestion
}

// ensureSuggestedTags materializes new AI knowledge-point labels for this instance.
func (s *QuestionService) ensureSuggestedTags(suggestion *ai.TaxonomySuggestion) {
	if suggestion == nil || suggestion.CategoryID == nil || s.tagRepo == nil {
		return
	}
	created := 0
	for _, rawName := range cleanStringSlice(suggestion.UnresolvedTagNames) {
		if created >= 3 {
			break
		}
		name := strings.TrimSpace(rawName)
		runeCount := len([]rune(name))
		if runeCount < 2 || runeCount > 32 || strings.ContainsAny(name, "\r\n\t") {
			continue
		}
		if _, err := s.tagRepo.FindOrCreate(name); err != nil {
			log.Printf("[taxonomy] create suggested tag name=%q: %v", name, err)
			continue
		}
		created++
	}
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
	job.LockedAt = nil
	job.NextRunAt = nil
	job.ErrorCode = nil
	job.ErrorMessage = nil
	return job
}

func markJobNeedsReview(job *models.Job) *models.Job {
	now := time.Now()
	job.Status = "needs_review"
	job.FinishedAt = &now
	job.LockedAt = nil
	job.NextRunAt = nil
	job.ErrorCode = nil
	job.ErrorMessage = nil
	return job
}

func markJobFailed(job *models.Job, code string, message string) *models.Job {
	now := time.Now()
	job.Status = "failed"
	job.FinishedAt = &now
	job.LockedAt = nil
	job.NextRunAt = nil
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
