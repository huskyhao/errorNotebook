package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"erro-notebook/backend/internal/services"
	"erro-notebook/backend/pkg/response"
	"github.com/gin-gonic/gin"
)

type PracticeHandler struct {
	practiceService *services.PracticeService
}

type gradeSuggestionRequest struct {
	Params map[string]any `json:"params"`
}
type confirmGradeRequest struct {
	ProposalID string   `json:"proposalId" binding:"required"`
	Score      *float64 `json:"score"`
	Feedback   *string  `json:"feedback"`
}

func (h *PracticeHandler) GenerateGradeSuggestion(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid session id")
		return
	}
	orderIndex, err := strconv.Atoi(c.Param("orderIndex"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ORDER_INDEX", "invalid orderIndex")
		return
	}
	var req gradeSuggestionRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
			return
		}
	}
	result, err := h.practiceService.GenerateGradeSuggestion(c.Request.Context(), sessionID, orderIndex, req.Params)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "GRADE_SUGGESTION_FAILED", err.Error())
		return
	}
	response.OK(c, result)
}

func (h *PracticeHandler) ConfirmGradeSuggestion(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid session id")
		return
	}
	orderIndex, err := strconv.Atoi(c.Param("orderIndex"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ORDER_INDEX", "invalid orderIndex")
		return
	}
	var req confirmGradeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.practiceService.ConfirmGradeSuggestion(c.Request.Context(), sessionID, orderIndex, req.ProposalID, req.Score, req.Feedback); err != nil {
		response.Error(c, http.StatusConflict, "GRADE_CONFIRM_FAILED", err.Error())
		return
	}
	response.OK(c, gin.H{"status": "applied", "proposalId": req.ProposalID})
}

func (h *PracticeHandler) GetGradeSuggestion(c *gin.Context) {
	sessionID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid session id")
		return
	}
	orderIndex, err := strconv.Atoi(c.Param("orderIndex"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ORDER_INDEX", "invalid orderIndex")
		return
	}
	item, err := h.practiceService.GetSessionQuestion(sessionID, orderIndex)
	if err != nil || item == nil {
		response.Error(c, http.StatusNotFound, "PRACTICE_QUESTION_NOT_FOUND", "question not found in session")
		return
	}
	proposal, err := h.practiceService.GetGradeSuggestion(c.Request.Context(), c.Param("proposalId"), item.QuestionID)
	if err != nil || proposal == nil {
		response.Error(c, http.StatusNotFound, "PROPOSAL_NOT_FOUND", "grading suggestion not found")
		return
	}
	var content any
	if json.Unmarshal([]byte(proposal.ContentJSON), &content) != nil {
		content = map[string]any{}
	}
	response.OK(c, gin.H{"proposalId": proposal.ProposalID, "status": proposal.Status, "content": content, "expiresAt": proposal.ExpiresAt})
}

func (h *PracticeHandler) GetPracticeRecommendations(c *gin.Context) {
	groups, err := h.practiceService.GetPracticeRecommendations(time.Now())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "RECOMMENDATIONS_FAILED", err.Error())
		return
	}
	response.OK(c, groups)
}

func NewPracticeHandler(practiceService *services.PracticeService) *PracticeHandler {
	return &PracticeHandler{practiceService: practiceService}
}

type createSessionRequest struct {
	Name        string  `json:"name"`
	QuestionIDs []int64 `json:"questionIds" binding:"required"`
}

func (h *PracticeHandler) CreateSession(c *gin.Context) {
	var req createSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	detail, err := h.practiceService.CreateSession(services.CreateSessionInput{
		Name:        req.Name,
		QuestionIDs: req.QuestionIDs,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, "SESSION_CREATE_FAILED", err.Error())
		return
	}

	response.Created(c, detail)
}

func (h *PracticeHandler) ListSessions(c *gin.Context) {
	sessions, err := h.practiceService.ListSessions()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SESSION_LIST_FAILED", err.Error())
		return
	}
	response.OK(c, sessions)
}

func (h *PracticeHandler) GetSession(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid id")
		return
	}

	detail, err := h.practiceService.GetSession(id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SESSION_GET_FAILED", err.Error())
		return
	}
	if detail == nil {
		response.Error(c, http.StatusNotFound, "SESSION_NOT_FOUND", "session not found")
		return
	}

	response.OK(c, detail)
}

type answerQuestionRequest struct {
	OrderIndex *int   `json:"orderIndex" binding:"required"`
	UserAnswer string `json:"userAnswer" binding:"required"`
}

func (h *PracticeHandler) AnswerQuestion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid id")
		return
	}

	var req answerQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.practiceService.AnswerQuestion(id, *req.OrderIndex, req.UserAnswer); err != nil {
		response.Error(c, http.StatusBadRequest, "ANSWER_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"status": "answered"})
}

type skipQuestionRequest struct {
	OrderIndex *int `json:"orderIndex" binding:"required"`
}

func (h *PracticeHandler) SkipQuestion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid id")
		return
	}

	var req skipQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if err := h.practiceService.SkipQuestion(id, *req.OrderIndex); err != nil {
		response.Error(c, http.StatusBadRequest, "SKIP_FAILED", err.Error())
		return
	}

	response.OK(c, gin.H{"status": "skipped"})
}

func (h *PracticeHandler) SubmitSession(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid id")
		return
	}

	result, err := h.practiceService.SubmitSession(id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "SUBMIT_FAILED", err.Error())
		return
	}

	response.OK(c, result)
}

func (h *PracticeHandler) GetResults(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "invalid id")
		return
	}

	result, err := h.practiceService.GetSessionResults(id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "RESULTS_FAILED", err.Error())
		return
	}
	if result == nil {
		response.Error(c, http.StatusNotFound, "SESSION_NOT_FOUND", "session not found")
		return
	}

	response.OK(c, result)
}
