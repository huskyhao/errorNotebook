package handlers

import (
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"erro-notebook/backend/internal/models"
	"erro-notebook/backend/internal/repository"
	"erro-notebook/backend/internal/services"
	"erro-notebook/backend/pkg/response"
	"github.com/gin-gonic/gin"
)

type QuestionHandler struct {
	questionService *services.QuestionService
	jobService      *services.JobService
}

// accessMiddleware protects every /questions/:id route, including nested
// chat, analysis, proposal and learning-state endpoints. It deliberately
// returns the same not-found result for another anonymous user's question.
func (h *QuestionHandler) accessMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/api/v1/questions/")
		part := strings.Split(path, "/")[0]
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			if err := h.questionService.AuthorizeQuestion(c.Request.Context(), id); err != nil {
				response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

func NewQuestionHandler(
	questionService *services.QuestionService,
	jobService *services.JobService,
) *QuestionHandler {
	return &QuestionHandler{
		questionService: questionService,
		jobService:      jobService,
	}
}

func (h *QuestionHandler) Import(c *gin.Context) {
	sourceType := c.PostForm("sourceType")
	rawText := c.PostForm("rawText")

	var fileHeader *multipart.FileHeader
	if fh, err := c.FormFile("file"); err == nil {
		fileHeader = fh
	}

	result, err := h.questionService.ImportQuestion(c.Request.Context(), services.ImportQuestionInput{
		SourceType: sourceType,
		RawText:    rawText,
		FileHeader: fileHeader,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, "QUESTION_IMPORT_FAILED", err.Error())
		return
	}

	response.Created(c, result)
}

func (h *QuestionHandler) GetQuestion(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	question, err := h.questionService.GetQuestion(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "QUESTION_FETCH_FAILED", err.Error())
		return
	}

	response.OK(c, question)
}

func (h *QuestionHandler) ListQuestions(c *gin.Context) {
	filters := repository.QuestionListFilters{
		QuestionType:   strings.TrimSpace(c.Query("questionType")),
		OCRStatus:      strings.TrimSpace(c.Query("ocrStatus")),
		AnalysisStatus: strings.TrimSpace(c.Query("analysisStatus")),
		Keyword:        strings.TrimSpace(c.Query("keyword")),
	}
	if categoryID := strings.TrimSpace(c.Query("categoryId")); categoryID != "" {
		id, err := strconv.ParseInt(categoryID, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_CATEGORY_ID", "invalid categoryId")
			return
		}
		if id == 0 {
			filters.Uncategorized = true
		} else {
			filters.CategoryID = &id
		}
	}
	if isFav := strings.TrimSpace(c.Query("isFavorited")); isFav != "" {
		b, err := strconv.ParseBool(isFav)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_FAVORITED", "invalid isFavorited")
			return
		}
		filters.IsFavorited = &b
	}
	if tagIDs := strings.TrimSpace(c.Query("tagIds")); tagIDs != "" {
		parts := strings.Split(tagIDs, ",")
		ids := make([]int64, 0, len(parts))
		for _, p := range parts {
			id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err != nil {
				response.Error(c, http.StatusBadRequest, "INVALID_TAG_IDS", "invalid tagIds")
				return
			}
			ids = append(ids, id)
		}
		filters.TagIDs = ids
	}

	items, err := h.questionService.ListQuestions(c.Request.Context(), filters)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "QUESTION_LIST_FAILED", err.Error())
		return
	}

	response.OK(c, items)
}

type updateQuestionRequest struct {
	Stem          *string                 `json:"stem"`
	QuestionType  *string                 `json:"questionType"`
	CorrectAnswer *string                 `json:"correctAnswer"`
	CategoryID    json.RawMessage         `json:"categoryId"`
	Options       []models.QuestionOption `json:"options"`
}

func (h *QuestionHandler) UpdateQuestion(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var req updateQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	var categoryID *int64
	categoryIDSet := req.CategoryID != nil
	if categoryIDSet && string(req.CategoryID) != "null" {
		var value int64
		if err := json.Unmarshal(req.CategoryID, &value); err != nil || value <= 0 {
			response.Error(c, http.StatusBadRequest, "INVALID_CATEGORY_ID", "categoryId must be a positive id or null")
			return
		}
		categoryID = &value
	}

	question, err := h.questionService.UpdateQuestion(c.Request.Context(), id, services.UpdateQuestionInput{
		Stem:          req.Stem,
		QuestionType:  req.QuestionType,
		CorrectAnswer: req.CorrectAnswer,
		CategoryID:    categoryID,
		CategoryIDSet: categoryIDSet,
		Options:       req.Options,
	})
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		if errors.Is(err, services.ErrInvalidCategory) {
			response.Error(c, http.StatusBadRequest, "INVALID_CATEGORY_ID", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "QUESTION_UPDATE_FAILED", err.Error())
		return
	}

	response.OK(c, question)
}

type submitAnswerRequest struct {
	UserAnswer string `json:"userAnswer" binding:"required"`
}

func (h *QuestionHandler) SubmitAnswer(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var req submitAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	question, err := h.questionService.SubmitAnswer(c.Request.Context(), id, services.SubmitAnswerInput{
		UserAnswer: req.UserAnswer,
	})
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "QUESTION_ANSWER_FAILED", err.Error())
		return
	}

	response.OK(c, question)
}

func (h *QuestionHandler) AnalyzeQuestion(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	result, err := h.questionService.AnalyzeQuestion(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		response.Error(c, http.StatusBadGateway, "QUESTION_ANALYZE_FAILED", err.Error())
		return
	}

	response.Created(c, result)
}

func (h *QuestionHandler) RetryOCR(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	result, err := h.questionService.RetryOCR(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		response.Error(c, http.StatusBadRequest, "OCR_RETRY_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

func (h *QuestionHandler) GetAnalysis(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	analysis, err := h.questionService.GetLatestAnalysis(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "QUESTION_ANALYSIS_FETCH_FAILED", err.Error())
		return
	}
	if analysis == nil {
		response.Error(c, http.StatusNotFound, "ANALYSIS_NOT_FOUND", "analysis not found")
		return
	}

	response.OK(c, analysis)
}

func (h *QuestionHandler) GetLearningState(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	state, err := h.questionService.GetLearningState(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "LEARNING_STATE_FETCH_FAILED", err.Error())
		return
	}
	response.OK(c, state)
}

type updateLearningStateRequest struct {
	MasteryLevel  *int     `json:"masteryLevel"`
	MistakeReason *string  `json:"mistakeReason"`
	WeaknessTags  []string `json:"weaknessTags"`
	ReviewAdvice  []string `json:"reviewAdvice"`
}

func (h *QuestionHandler) UpdateLearningState(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req updateLearningStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	state, err := h.questionService.UpdateLearningState(c.Request.Context(), id, services.UpdateLearningStateInput{
		MasteryLevel:  req.MasteryLevel,
		MistakeReason: req.MistakeReason,
		WeaknessTags:  req.WeaknessTags,
		ReviewAdvice:  req.ReviewAdvice,
	})
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "LEARNING_STATE_UPDATE_FAILED", err.Error())
		return
	}
	response.OK(c, state)
}

func (h *QuestionHandler) GenerateLearningState(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	state, err := h.questionService.GenerateLearningState(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "LEARNING_STATE_GENERATE_FAILED", err.Error())
		return
	}
	response.OK(c, state)
}

func (h *QuestionHandler) ApplyTaxonomySuggestion(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	question, err := h.questionService.ApplyTaxonomySuggestion(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		if errors.Is(err, services.ErrTaxonomySuggestionUnavailable) {
			response.Error(c, http.StatusConflict, "TAXONOMY_SUGGESTION_UNAVAILABLE", err.Error())
			return
		}
		if errors.Is(err, services.ErrProposalConflict) {
			response.Error(c, http.StatusConflict, "TAXONOMY_SUGGESTION_EXPIRED", err.Error())
			return
		}
		response.Error(c, http.StatusBadRequest, "TAXONOMY_SUGGESTION_APPLY_FAILED", err.Error())
		return
	}
	response.OK(c, question)
}

type runAgentActionRequest struct {
	Action string         `json:"action" binding:"required"`
	Params map[string]any `json:"params"`
}

func (h *QuestionHandler) RunAgentAction(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var req runAgentActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	result, err := h.questionService.RunAgentAction(c.Request.Context(), id, req.Action, req.Params)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		if errors.Is(err, services.ErrAIUnavailable) {
			response.Error(c, http.StatusBadGateway, "AI_AGENT_UNAVAILABLE", err.Error())
			return
		}
		response.Error(c, http.StatusBadRequest, "AGENT_ACTION_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

func (h *QuestionHandler) GetAIProposal(c *gin.Context) {
	questionID, ok := parseIDParam(c)
	if !ok {
		return
	}
	proposal, err := h.questionService.GetAIProposal(c.Request.Context(), c.Param("proposalId"))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "PROPOSAL_FETCH_FAILED", err.Error())
		return
	}
	if proposal == nil {
		response.Error(c, http.StatusNotFound, "PROPOSAL_NOT_FOUND", "proposal not found")
		return
	}
	if proposal.QuestionID != questionID {
		response.Error(c, http.StatusNotFound, "PROPOSAL_NOT_FOUND", "proposal not found")
		return
	}
	var content any
	if json.Unmarshal([]byte(proposal.ContentJSON), &content) != nil {
		content = map[string]any{}
	}
	response.OK(c, gin.H{"proposalId": proposal.ProposalID, "questionId": proposal.QuestionID, "action": proposal.Action, "status": proposal.Status, "content": content, "sourceFingerprint": proposal.SourceFingerprint, "expiresAt": proposal.ExpiresAt, "createdQuestionId": proposal.CreatedQuestionID})
}

func (h *QuestionHandler) ConfirmSimilarProposal(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	result, err := h.questionService.ConfirmSimilarQuestion(c.Request.Context(), id, c.Param("proposalId"))
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", err.Error())
			return
		}
		response.Error(c, http.StatusConflict, "PROPOSAL_NOT_APPLICABLE", err.Error())
		return
	}
	response.OK(c, result)
}

func (h *QuestionHandler) RejectAIProposal(c *gin.Context) {
	questionID, ok := parseIDParam(c)
	if !ok {
		return
	}
	if err := h.questionService.RejectAIProposal(c.Request.Context(), c.Param("proposalId"), questionID); err != nil {
		if errors.Is(err, services.ErrProposalConflict) {
			response.Error(c, http.StatusNotFound, "PROPOSAL_NOT_FOUND", "proposal not found")
			return
		}
		response.Error(c, http.StatusConflict, "PROPOSAL_REJECT_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"status": "rejected"})
}

type createChatMessageRequest struct {
	Message        string `json:"message" binding:"required"`
	IdempotencyKey string `json:"idempotencyKey"`
}

func (h *QuestionHandler) CreateChatMessage(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var req createChatMessageRequest
	var files []*multipart.FileHeader
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		req.Message = c.PostForm("message")
		req.IdempotencyKey = c.PostForm("idempotencyKey")
		form, err := c.MultipartForm()
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_MULTIPART", err.Error())
			return
		}
		files = form.File["attachments"]
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
	}

	messages, err := h.questionService.CreateChatMessage(c.Request.Context(), id, services.CreateChatMessageInput{
		Message:         req.Message,
		IdempotencyKey:  req.IdempotencyKey,
		AttachmentFiles: files,
	})
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		if errors.Is(err, services.ErrAIUnavailable) {
			response.Error(c, http.StatusBadGateway, "AI_CHAT_UNAVAILABLE", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "QUESTION_CHAT_FAILED", err.Error())
		return
	}

	response.Created(c, messages)
}

func (h *QuestionHandler) GetChatMessages(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	messages, err := h.questionService.GetChatMessages(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "CHAT_MESSAGES_FAILED", err.Error())
		return
	}

	response.OK(c, messages)
}

func (h *QuestionHandler) DeleteQuestion(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	if err := h.questionService.DeleteQuestion(c.Request.Context(), id); err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "QUESTION_DELETE_FAILED", err.Error())
		return
	}

	response.NoContent(c)
}

type toggleFavoriteRequest struct {
	IsFavorited bool `json:"isFavorited"`
}

func (h *QuestionHandler) ToggleFavorite(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var req toggleFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	question, err := h.questionService.ToggleFavorite(c.Request.Context(), id, req.IsFavorited)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "QUESTION_FAVORITE_FAILED", err.Error())
		return
	}

	response.OK(c, question)
}

type setQuestionTagsRequest struct {
	TagIDs []int64 `json:"tagIds"`
}

func (h *QuestionHandler) SetQuestionTags(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	var req setQuestionTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	question, err := h.questionService.SetQuestionTags(c.Request.Context(), id, req.TagIDs)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
			return
		}
		if errors.Is(err, repository.ErrTagNotFound) {
			response.Error(c, http.StatusBadRequest, "INVALID_TAG_IDS", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "QUESTION_TAG_FAILED", err.Error())
		return
	}

	response.OK(c, question)
}

func (h *QuestionHandler) GetJob(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("jobId"))
	if jobID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_JOB_ID", "jobId is required")
		return
	}

	job, err := h.jobService.GetByJobID(c.Request.Context(), jobID)
	if err != nil {
		if errors.Is(err, services.ErrJobNotFound) {
			response.Error(c, http.StatusNotFound, "JOB_NOT_FOUND", "job not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "JOB_FETCH_FAILED", err.Error())
		return
	}

	response.OK(c, job)
}

func (h *QuestionHandler) RetryJob(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("jobId"))
	if jobID == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_JOB_ID", "jobId is required")
		return
	}
	job, err := h.jobService.RetryJob(c.Request.Context(), jobID)
	if err != nil {
		if errors.Is(err, services.ErrJobNotFound) {
			response.Error(c, http.StatusNotFound, "JOB_NOT_FOUND", "job not found")
			return
		}
		if errors.Is(err, services.ErrJobNotRetryable) {
			response.Error(c, http.StatusConflict, "JOB_NOT_RETRYABLE", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "JOB_RETRY_FAILED", err.Error())
		return
	}
	response.OK(c, job)
}

func (h *QuestionHandler) BatchImport(c *gin.Context) {
	sourceType := c.DefaultPostForm("sourceType", "image")
	form, err := c.MultipartForm()
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_MULTIPART", err.Error())
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		response.Error(c, http.StatusBadRequest, "NO_FILES", "no files provided")
		return
	}

	result, err := h.questionService.BatchImportQuestions(c.Request.Context(), services.BatchImportInput{
		SourceType: sourceType,
		Files:      files,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BATCH_IMPORT_FAILED", err.Error())
		return
	}

	response.Created(c, result)
}

func (h *QuestionHandler) GetBatchImport(c *gin.Context) {
	id, ok := parseIDParam(c)
	if !ok {
		return
	}

	result, err := h.questionService.GetBatchImport(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, services.ErrQuestionNotFound) {
			response.Error(c, http.StatusNotFound, "BATCH_NOT_FOUND", "batch not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "BATCH_FETCH_FAILED", err.Error())
		return
	}

	response.OK(c, result)
}

func parseIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid id")
		return 0, false
	}
	return id, true
}
