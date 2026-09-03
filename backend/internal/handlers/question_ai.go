package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"erro-notebook/backend/internal/integrations/ai"
	"erro-notebook/backend/internal/services"
	"github.com/gin-gonic/gin"
)

type QuestionAIHandler struct {
	service *services.QuestionAIService
}

func NewQuestionAIHandler(service *services.QuestionAIService) *QuestionAIHandler {
	return &QuestionAIHandler{
		service: service,
	}
}

func (h *QuestionAIHandler) ParseImage(c *gin.Context) {
	questionID, err := strconv.ParseInt(c.PostForm("question_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid question_id"})
		return
	}

	sourceType := c.DefaultPostForm("source_type", "image")
	traceID := c.DefaultPostForm("trace_id", strconv.FormatInt(time.Now().UnixNano(), 10))

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "open uploaded file failed"})
		return
	}
	defer src.Close()

	tempFile, err := os.CreateTemp("", "erro-ai-*"+filepath.Ext(fileHeader.Filename))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create temp file failed"})
		return
	}
	tempPath := tempFile.Name()
	if _, err := io.Copy(tempFile, src); err != nil {
		tempFile.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save uploaded file failed"})
		return
	}
	if err := tempFile.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save uploaded file failed"})
		return
	}
	defer os.Remove(tempPath)

	result, err := h.service.ParseQuestionImage(c.Request.Context(), questionID, traceID, sourceType, tempPath)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

type analyzeQuestionHTTPPayload struct {
	QuestionID int64                 `json:"questionId" binding:"required"`
	TraceID    string                `json:"traceId" binding:"required"`
	Question   ai.StructuredQuestion `json:"question" binding:"required"`
	UserAnswer string                `json:"userAnswer"`
	Context    map[string]any        `json:"context"`
}

func (h *QuestionAIHandler) AnalyzeQuestion(c *gin.Context) {
	var payload analyzeQuestionHTTPPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.service.AnalyzeQuestion(c.Request.Context(), ai.AnalyzeQuestionRequest{
		QuestionID: payload.QuestionID,
		TraceID:    payload.TraceID,
		Question:   payload.Question,
		UserAnswer: payload.UserAnswer,
		Context:    payload.Context,
	}, "")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
